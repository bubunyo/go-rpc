package rpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bubunyo/go-rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	EchoService struct{}
	EchoRequest struct {
		Echo string `json:"echo"`
	}
	EchoResponse struct {
		Echo string `json:"echo"`
	}
	TestService struct {
		ProcessFn func(_ context.Context, req *rpc.RequestParams) (any, error)
	}
)

func (s EchoService) Register() (string, rpc.RequestMap) {
	return "EchoService", map[string]rpc.RequestFunc{
		"Ping": s.Ping,
	}
}

func (ts TestService) Register() (string, rpc.RequestMap) {
	return "TestService", map[string]rpc.RequestFunc{
		"Exec": ts.Exec,
	}
}

func (s TestService) MethodName() string {
	return "TestService.Exec"
}

func (s TestService) Exec(ctx context.Context, req *rpc.RequestParams) (any, error) {
	return s.ProcessFn(ctx, req)
}

func NewEchoService() rpc.ServiceRegistrar {
	return EchoService{}
}

func (s EchoService) Ping(_ context.Context, req *rpc.RequestParams) (any, error) {
	var er EchoRequest
	_ = json.Unmarshal(req.Payload, &er)
	return EchoResponse{
		Echo: "echo " + er.Echo,
	}, nil
}

func requestObj(t *testing.T, method string, params any) *http.Request {
	reqObj := map[string]any{
		"jsonrpc": rpc.Version,
		"method":  method,
		"params":  params,
		"id":      method,
	}
	payload, err := json.Marshal(reqObj)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(payload))
	require.NoError(t, err)
	return req
}

func successResponse(t *testing.T, resp *http.Response) any {
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var r map[string]any
	err := json.NewDecoder(resp.Body).Decode(&r)
	require.NoError(t, err)
	assert.Equal(t, "2.0", r["jsonrpc"])
	assert.Contains(t, r, "id")
	assert.NotContains(t, r, "error", "Response contains error", r["error"])
	return r["result"]
}

func errorResponse(t *testing.T, resp *http.Response) (int, string) {
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var r map[string]any
	err := json.NewDecoder(resp.Body).Decode(&r)
	require.NoError(t, err)
	assert.Contains(t, r, "id")
	assert.NotContains(t, r, "result", "Response contains result", r["result"])
	e := r["error"].(map[string]any)
	assert.NotEqual(t, 0, e["code"])
	if int(e["code"].(float64)) != rpc.InvalidRpcVersion.Code {
		assert.Equal(t, "2.0", r["jsonrpc"])
	}
	assert.NotEmpty(t, e["message"])
	return int(e["code"].(float64)), e["message"].(string)
}

func TestRpcServerResponses(t *testing.T) {
	server := rpc.NewDefaultServer()
	server.AddService(NewEchoService())
	req := requestObj(t, "EchoService.Ping", map[string]any{
		"echo": "ping",
	})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	result := successResponse(t, rec.Result()).(map[string]any)
	assert.Equal(t, "echo ping", result["echo"])
}

func TestRpcServer_ErrorResponses(t *testing.T) {
	server := rpc.NewDefaultServer()
	server.AddService(NewEchoService())
	req := requestObj(t, "EchoService.NonMethod", map[string]any{
		"echo": "ping",
	})
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	code, msg := errorResponse(t, rec.Result())
	assert.Equal(t, -32601, code)
	assert.Equal(t, "The method does not exist / is not available", msg)
}

func TestRpcServer_InvalidJsonRpcVersion(t *testing.T) {
	server := rpc.NewDefaultServer()
	server.AddService(NewEchoService())
	reqObj := map[string]any{
		"jsonrpc": "1.0",
		"method":  "EchoService.Ping",
		"params": map[string]any{
			"echo": "ping",
		},
		"id": "test",
	}
	payload, err := json.Marshal(reqObj)
	require.NoError(t, err)
	req, _ := http.NewRequest(http.MethodPost, "", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)
	code, msg := errorResponse(t, rec.Result())
	assert.Equal(t, -32004, code)
	assert.Equal(t, "Invalid RPC Version", msg)
}

func TestRpcServer_EmptyMethodName(t *testing.T) {
	server := rpc.NewDefaultServer()
	server.AddService(NewEchoService())
	cases := []string{" ", "", "\n\n", "\t\n"}
	for _, m := range cases {
		t.Run("case "+m, func(t *testing.T) {
			req := requestObj(t, m, map[string]any{
				"echo": "ping",
			})
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			code, _ := errorResponse(t, rec.Result())
			assert.Equal(t, -32602, code)
		})
	}
}

func TestRpcServer_ValidRequestParams(t *testing.T) {
	server := rpc.NewDefaultServer()
	ts := &TestService{}
	ts.ProcessFn = func(_ context.Context, req *rpc.RequestParams) (any, error) {
		return "ok", nil
	}
	server.AddService(ts)
	cases := []struct {
		name  string
		param any
	}{
		{"zero_param", map[string]any{}},
		{"nil_param", nil},
		{"string_param", "test"},
		{"string_param", 95},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := requestObj(t, ts.MethodName(), nil)
			server.ServeHTTP(rec, req)
			resp := successResponse(t, rec.Result())
			assert.Equal(t, "ok", resp)
		})
	}
}

func TestRpcServer_ExecutionTimeout(t *testing.T) {
	opts := rpc.Opts{
		ExecutionTimeout: time.Second,
		MaxBytesRead:     rpc.MaxBytesRead,
	}
	server := rpc.NewServer(opts)
	ts := &TestService{}
	ts.ProcessFn = func(_ context.Context, req *rpc.RequestParams) (any, error) {
		time.Sleep(opts.ExecutionTimeout + (2 * time.Second))
		return "ok", nil
	}
	server.AddService(ts)
	rec := httptest.NewRecorder()
	req := requestObj(t, ts.MethodName(), nil)
	server.ServeHTTP(rec, req)
	code, msg := errorResponse(t, rec.Result())
	assert.Equal(t, rpc.ExecutionTimeoutError.Code, code)
	assert.Equal(t, rpc.ExecutionTimeoutError.Message, msg)
}

func TestRpcServer_ExecuteMultipleRequests(t *testing.T) {
	opts := rpc.Opts{
		ExecutionTimeout: time.Second,
		MaxBytesRead:     rpc.MaxBytesRead,
	}
	server := rpc.NewServer(opts)
	ts := &TestService{}
	ts.ProcessFn = func(_ context.Context, req *rpc.RequestParams) (any, error) {
		var s string
		_ = json.Unmarshal(req.Payload, &s)
		switch s {
		case "wait":
			time.Sleep(opts.ExecutionTimeout + (2 * time.Second))
			return "ok - " + s, nil
		case "error":
			return nil, errors.New("static error")
		default:
			return "ok - " + s, nil
		}
	}
	server.AddService(ts)
	rec := httptest.NewRecorder()

	reqObj := []map[string]any{
		{
			"jsonrpc": rpc.Version,
			"method":  ts.MethodName(),
			"params":  "random",
			"id":      "test-1",
		},
		{
			"jsonrpc": rpc.Version,
			"method":  ts.MethodName(),
			"params":  "error",
			"id":      "test-2",
		},
		{
			"jsonrpc": rpc.Version,
			"method":  ts.MethodName(),
			"params":  "wait",
			"id":      "test-3",
		},
		{
			"jsonrpc": "1.0",
			"method":  ts.MethodName(),
			"params":  "bad-version",
			"id":      "test-4",
		},
	}
	payload, err := json.Marshal(reqObj)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(payload))
	require.NoError(t, err)

	server.ServeHTTP(rec, req)

	var res []map[string]any
	err = json.NewDecoder(rec.Result().Body).Decode(&res)
	require.NoError(t, err)
	for _, e := range res {
		switch e["id"] {
		case "test-1":
			assert.Equal(t, "ok - random", e["result"])
		case "test-2":
			er := e["error"].(map[string]any)
			assert.Equal(t, float64(-32602), er["code"])
			assert.Equal(t, "static error", er["message"])
		case "test-3":
			er := e["error"].(map[string]any)
			assert.Equal(t, float64(-32001), er["code"])
			assert.Equal(t, "Execution Timeout", er["message"])
		case "test-4":
			er := e["error"].(map[string]any)
			assert.Equal(t, float64(-32004), er["code"])
			assert.Equal(t, "Invalid RPC Version", er["message"])
		}
	}
}
