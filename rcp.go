package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
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
	defaultReq  = Request{JsonRpc: Version}
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
		methodMeta       map[string]MethodMeta
		executionTimeout time.Duration
		maxBytesRead     int64
	}
	RequestFunc      = func(context.Context, *RequestParams) (any, error)
	RequestMap       = map[string]RequestFunc
	ServiceRegistrar interface {
		Register() (string, RequestMap)
	}
	// MethodMeta holds AI-readable metadata describing a registered RPC method.
	// It is returned by the built-in rpc.discover method.
	MethodMeta struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Params      any    `json:"params,omitempty"`
		Result      any    `json:"result,omitempty"`
	}
	// ServiceDescriptor is an optional interface that services can implement to
	// provide machine-readable descriptions of their methods for AI agent discovery.
	// Describe returns a map of unqualified method names to their MethodMeta.
	ServiceDescriptor interface {
		Describe() map[string]MethodMeta
	}
	Request struct {
		JsonRpc string `json:"jsonrpc"` // must always be 2.0
		Id      any    `json:"id"`      // should be a string, number or null.
		Method  string `json:"method"`  // the method being called
		Params  any    `json:"params"`  // the params for the method being called
	}
	Response struct {
		JsonRpc string `json:"jsonrpc,omitempty"` // must always be 2.0
		Id      any    `json:"id"`                // the id passed in the request object
		Result  any    `json:"result,omitempty"`  // required when the request is successful
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

func errorResponse(req *Request, err error) Response {
	res := Response{
		JsonRpc: req.JsonRpc,
		Id:      req.Id,
		Error:   &Error{},
	}
	switch err.(type) {
	case Error:
		e := err.(Error)
		res.Error.Code = e.Code
		res.Error.Message = e.Message
		if res.Error.Code == InvalidRpcVersion.Code {
			res.JsonRpc = ""
		}
		return res
	default:
		res.Error.Code = InternalError.Code
		res.Error.Message = err.Error()
		return res
	}
}

func successResponse(req Request, body any) Response {
	return Response{
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
		resp := make([]Response, len(payloads))
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
func parseRequest(payload map[string]any) Request {
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

func (s *Service) handle(ctx context.Context, req Request) Response {
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

func (s *Service) handleMethod(ctx context.Context, req Request) (any, error) {
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

func (s *Service) AddService(services ...ServiceRegistrar) {
	for _, srv := range services {
		name, requestMap := srv.Register()
		nameFmt := "%s"
		if name != "" {
			nameFmt = "%s.%s"
		}
		var descriptions map[string]MethodMeta
		if descriptor, ok := srv.(ServiceDescriptor); ok {
			descriptions = descriptor.Describe()
		}
		for methodName, fn := range requestMap {
			fullName := fmt.Sprintf(nameFmt, name, methodName)
			s.methodMap[fullName] = fn
			meta := MethodMeta{Name: fullName}
			if d, ok := descriptions[methodName]; ok {
				meta.Description = d.Description
				meta.Params = d.Params
				meta.Result = d.Result
			}
			s.methodMeta[fullName] = meta
		}
	}
}

func NewService(opts Opts) *Service {
	svc := &Service{
		methodMap:        map[string]func(context.Context, *RequestParams) (any, error){},
		methodMeta:       map[string]MethodMeta{},
		executionTimeout: opts.ExecutionTimeout,
		maxBytesRead:     opts.MaxBytesRead,
	}
	svc.methodMap["rpc.discover"] = svc.discover
	return svc
}

// discover implements the built-in rpc.discover method. It returns a sorted
// list of all registered methods with their AI-readable metadata, enabling AI
// agents to enumerate and understand the available RPC API.
func (s *Service) discover(_ context.Context, _ *RequestParams) (any, error) {
	methods := make([]MethodMeta, 0, len(s.methodMap)-1)
	for name := range s.methodMap {
		if name == "rpc.discover" {
			continue
		}
		meta, ok := s.methodMeta[name]
		if !ok {
			meta = MethodMeta{Name: name}
		}
		methods = append(methods, meta)
	}
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name < methods[j].Name
	})
	return methods, nil
}
