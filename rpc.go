package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	Version          = "2.0"            // JSON RPC Version
	MaxBytesRead     = 1 << 20          // 1mb
	ExecutionTimeout = 15 * time.Second // execution timout
)

var (
	defaultReq  = Request[any]{JsonRpc: Version}
	DefaultOpts = Opts{
		MaxBytesRead:     MaxBytesRead,
		ExecutionTimeout: ExecutionTimeout,
	}
)

// NewServer creates a new JSON RPC Server that can handle requests.
func NewServer(opts Opts) *Service {
	return NewService(opts)
}

// NewDefaultServer creates a new JSON RPC Server using the default options.
func NewDefaultServer() *Service {
	return NewService(DefaultOpts)
}

type (
	Opts struct {
		// MaxBytesRead is the maximum bytes a request object can contain
		MaxBytesRead int64
		// ExecutionTimeout is the maximum time a method should execute for. If the
		// execution exceeds the timeout, and ExectutionTimeout Error is returned for
		// that request
		ExecutionTimeout time.Duration
	}
	Service struct {
		methodMap        map[string]func(context.Context, *RequestParams) (any, error)
		executionTimeout time.Duration
		maxBytesRead     int64
	}
	RequestFunc[R any] = func(context.Context, *RequestParams) (R, error)
	ServiceRegistrar   interface {
		Registry() *ServiceRegistry
	}
	Request[P any] struct {
		JsonRpc string `json:"jsonrpc"` // must always be 2.0
		Id      any    `json:"id"`      // should be a string, number or null.
		Method  string `json:"method"`  // the method being called
		Params  P      `json:"params"`  // the params for the method being called
	}
	Response[R any] struct {
		JsonRpc string `json:"jsonrpc,omitempty"` // must always be 2.0
		Id      any    `json:"id"`                // the id passed in the request object
		Result  R      `json:"result,omitempty"`  // required when the request is successful
		Error   *Error `json:"error,omitempty"`   // required when the request is a failure
	}
	RequestParams struct {
		Payload []byte
	}
	methodResp struct {
		err  error
		resp any
	}
)

// Bind unmarshals the request payload into the value pointed to by v.
func (p *RequestParams) Bind(v any) error {
	return json.Unmarshal(p.Payload, v)
}

// ServiceRegistry holds the name and method handlers for a service.
type ServiceRegistry struct {
	name    string
	methods map[string]func(context.Context, *RequestParams) (any, error)
}

// NewRegistry creates a ServiceRegistry with the given service name.
func NewRegistry(name string) *ServiceRegistry {
	return &ServiceRegistry{
		name:    name,
		methods: map[string]func(context.Context, *RequestParams) (any, error){},
	}
}

// Handle registers a typed handler function under the given method name.
// The coercion from RequestFunc[R] to func(...)(any,error) happens here,
// inside the library, so callers never need an explicit wrapper.
func Handle[R any](r *ServiceRegistry, name string, fn RequestFunc[R]) *ServiceRegistry {
	r.methods[name] = func(ctx context.Context, p *RequestParams) (any, error) {
		return fn(ctx, p)
	}
	return r
}

func errorResponse(req *Request[any], err error) Response[any] {
	res := Response[any]{
		JsonRpc: req.JsonRpc,
		Id:      req.Id,
		Error:   &Error{},
	}
	var e Error
	if errors.As(err, &e) {
		res.Error.Code = e.Code
		res.Error.Message = e.Message
		if res.Error.Code == InvalidRpcVersion.Code {
			res.JsonRpc = ""
		}
		return res
	}
	res.Error.Code = InternalError.Code
	res.Error.Message = err.Error()
	return res
}

func successResponse(req Request[any], body any) Response[any] {
	return Response[any]{
		JsonRpc: req.JsonRpc,
		Id:      req.Id,
		Result:  body,
	}
}

func writeResponse(w http.ResponseWriter, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(response)
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if r.Body == nil {
		writeResponse(w, errorResponse(&defaultReq, RequestBodyIsEmpty))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBytesRead)
	buf := bytes.NewBuffer([]byte{})
	_, err := io.Copy(buf, r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeResponse(w, errorResponse(&defaultReq, RequestBodyTooLargeError))
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var reqPayload any
	err = json.NewDecoder(buf).Decode(&reqPayload)
	if err != nil {
		writeResponse(w, errorResponse(&defaultReq, ParseError))
		return
	}

	switch reqPayload.(type) {
	case []any:
		payloads := reqPayload.([]any)
		resp := make([]Response[any], len(payloads))
		wg := sync.WaitGroup{}
		wg.Add(len(payloads))
		for i, payload := range payloads {
			go func(index int, p any) {
				defer wg.Done()
				m, ok := p.(map[string]any)
				if !ok {
					resp[index] = errorResponse(&defaultReq, InvalidRequest)
					return
				}
				resp[index] = s.handle(r.Context(), parseRequest(m))
			}(i, payload)
		}
		wg.Wait()
		writeResponse(w, resp)
	case map[string]any:
		payload := reqPayload.(map[string]any)
		writeResponse(w, s.handle(r.Context(), parseRequest(payload)))
	default:
		writeResponse(w, errorResponse(&defaultReq, InvalidRequest))
	}
}
func parseRequest(payload map[string]any) Request[any] {
	req := defaultReq
	version, ok := payload["jsonrpc"]
	if ok {
		if v, ok := version.(string); ok {
			req.JsonRpc = v
		} else {
			req.JsonRpc = ""
		}
	}
	req.Id = payload["id"]
	if m, ok := payload["method"].(string); ok {
		req.Method = m
	}
	req.Params = payload["params"]
	return req
}

func (s *Service) handle(ctx context.Context, req Request[any]) Response[any] {
	if req.JsonRpc != Version {
		return errorResponse(&req, InvalidRpcVersion)
	}
	if strings.TrimSpace(req.Method) == "" {
		return errorResponse(&req, InvalidMethodParam)
	}
	res, err := s.handleMethod(ctx, req)
	if err != nil {
		return errorResponse(&req, err)
	}
	return successResponse(req, res)
}

func (s *Service) handleMethod(ctx context.Context, req Request[any]) (any, error) {
	fn, ok := s.methodMap[req.Method]
	if !ok {
		return nil, MethodNotFound
	}
	payload, err := json.Marshal(req.Params)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", InvalidRequest, err.Error())
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan methodResp, 1)
	go func() {
		params := &RequestParams{Payload: payload}
		res := methodResp{}
		res.resp, res.err = fn(ctx, params)
		result <- res
	}()
	delay := time.NewTimer(s.executionTimeout)
	select {
	case <-delay.C:
		return nil, ExecutionTimeoutError
	case r := <-result:
		if !delay.Stop() {
			<-delay.C
		}
		return r.resp, r.err
	}
}

func (s *Service) Register(services ...ServiceRegistrar) {
	for _, srv := range services {
		reg := srv.Registry()
		nameFmt := "%s"
		if reg.name != "" {
			nameFmt = "%s.%s"
		}
		for methodName, fn := range reg.methods {
			s.methodMap[fmt.Sprintf(nameFmt, reg.name, methodName)] = fn
		}
	}
}

func NewService(opts Opts) *Service {
	return &Service{
		methodMap:        map[string]func(context.Context, *RequestParams) (any, error){},
		executionTimeout: opts.ExecutionTimeout,
		maxBytesRead:     opts.MaxBytesRead,
	}
}
