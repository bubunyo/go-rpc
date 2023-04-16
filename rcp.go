package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const Version = "2.0"

var (
	defaultReq = Request{JsonRpc: Version}
)

// NewServer creates a new JSON RPC Server that can handle requests.
func NewServer() *Service {
	return NewService("")
}

const (
	MaxBytesRead = 1 << 20 // 1mb
)

type (
	Service struct {
		name      string
		methodMap map[string]func(context.Context, *RequestParams) (any, error)

		// ExecutionTimeout is the maximum time a method should execute for. If the
		// execution exceeds the timeout, and ExectutionTimeout Error is returned for
		// that request
		ExecutionTimeout time.Duration

		MaxBytesRead int64
	}
	Request struct {
		// must always be 2.0
		JsonRpc string `json:"jsonrpc"`
		// should be a string, number or null, if it is not included then the request is a notification.
		Id     any    `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params"`
	}
	Response struct {
		// must always be 2.0
		JsonRpc string `json:"jsonrpc,omitempty"`
		Id      any    `json:"id"`
		// required when the request is successful
		Result any `json:"result,omitempty"`
		// required when the request is a failure
		Error *Error `json:"error,omitempty"`
	}
	RequestParams struct {
		Payload []byte
	}
	methodResp struct {
		err  error
		resp any
	}
)

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
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
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
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxBytesRead)
	buf := bytes.NewBuffer([]byte{})
	n, err := io.Copy(buf, r.Body)
	if err != nil {
		if n > MaxBytesRead {
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
				resp[index] = s.handle(parseRequest(p.(map[string]any)))
			}(i, payload)
		}
		wg.Wait()
		writeResponse(w, resp)
	case map[string]any:
		payload := reqPayload.(map[string]any)
		writeResponse(w, s.handle(parseRequest(payload)))
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
	req.Method = payload["method"].(string)
	req.Params = payload["params"]
	return req
}

func (s *Service) handle(req Request) Response {
	if req.JsonRpc != Version {
		return errorResponse(&req, InvalidRpcVersion)
	}
	if strings.TrimSpace(req.Method) == "" {
		return errorResponse(&req, InvalidMethodParam)
	}
	res, err := s.handleMethod(req)
	if err != nil {
		return errorResponse(&req, err)
	}
	return successResponse(req, res)
}

func (s *Service) handleMethod(req Request) (any, error) {
	fn, ok := s.methodMap[req.Method]
	if !ok {
		return nil, MethodNotFound
	}
	payload, err := json.Marshal(req.Params)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", InvalidRequest, err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan methodResp)
	go func() {
		params := &RequestParams{Payload: payload}
		res := methodResp{}
		res.resp, res.err = fn(ctx, params)
		result <- res
	}()
	delay := time.NewTimer(s.ExecutionTimeout)
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

func (s Service) AddService(services ...*Service) {
	prefix := ""
	if s.name != "" {
		prefix = s.name + "."
	}
	for _, srv := range services {
		for methodName, fn := range srv.methodMap {
			s.methodMap[prefix+srv.name+"."+methodName] = fn
		}
	}
}

func NewService(name string) *Service {
	return &Service{
		name:             name,
		methodMap:        map[string]func(context.Context, *RequestParams) (any, error){},
		ExecutionTimeout: 15 * time.Second,
		MaxBytesRead:     MaxBytesRead,
	}
}

func (s *Service) RegisterMethod(methodName string, fn func(context.Context, *RequestParams) (any, error)) {
	s.methodMap[methodName] = fn
}