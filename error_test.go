package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	e := Error{
		Code:    500,
		Message: "Standard Error",
	}
	assert.Equal(t, "rpc error [500] Standard Error", e.Error())
}

// TestNewError verifies that NewError populates Code and Message correctly.
func TestNewError(t *testing.T) {
	e := NewError(-32700, "Parse error")
	assert.Equal(t, -32700, e.Code)
	assert.Equal(t, "Parse error", e.Message)
}

// TestNewError_Equality verifies that two calls with identical arguments
// produce equal Error values.
func TestNewError_Equality(t *testing.T) {
	a := NewError(-32600, "Invalid Request")
	b := NewError(-32600, "Invalid Request")
	assert.Equal(t, a, b)
}

// TestNewError_ZeroValues verifies that NewError with zero values is valid.
func TestNewError_ZeroValues(t *testing.T) {
	e := NewError(0, "")
	assert.Equal(t, 0, e.Code)
	assert.Equal(t, "", e.Message)
	assert.Equal(t, "rpc error [0] ", e.Error())
}

// TestError_ImplementsErrorInterface verifies that Error satisfies the error
// interface — a compile-time guarantee via assignment.
func TestError_ImplementsErrorInterface(t *testing.T) {
	var _ error = Error{}
	var _ error = NewError(-1, "test")
}

// TestErrorResponse_KnownError verifies that errorResponse correctly populates
// the Code and Message from an Error value.
func TestErrorResponse_KnownError(t *testing.T) {
	req := defaultReq
	e := NewError(-32601, "Method not found")
	resp := errorResponse(&req, e)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Equal(t, "Method not found", resp.Error.Message)
	assert.Equal(t, Version, resp.JsonRpc)
}

// TestErrorResponse_UnknownError verifies that errorResponse falls back to
// InternalError.Code for a plain error value.
func TestErrorResponse_UnknownError(t *testing.T) {
	req := defaultReq
	resp := errorResponse(&req, plainErr("boom"))
	assert.Equal(t, InternalError.Code, resp.Error.Code)
	assert.Equal(t, "boom", resp.Error.Message)
}

// TestErrorResponse_InvalidRpcVersion verifies current behaviour where the
// "jsonrpc" field is blanked when InvalidRpcVersion is the error. This is a
// spec violation — the field should always be "2.0". Update when fixed.
func TestErrorResponse_InvalidRpcVersion_BlanksJsonRpc(t *testing.T) {
	req := defaultReq
	resp := errorResponse(&req, InvalidRpcVersion)
	assert.Equal(t, "", resp.JsonRpc,
		"current (buggy) behaviour: jsonrpc is blanked for InvalidRpcVersion")
}

// TestErrorResponse_PreservesRequestID verifies that the response inherits
// the Id from the request.
func TestErrorResponse_PreservesRequestID(t *testing.T) {
	req := Request{JsonRpc: Version, Id: "req-42"}
	resp := errorResponse(&req, MethodNotFound)
	assert.Equal(t, "req-42", resp.Id)
}

// plainErr is a minimal error implementation used in white-box tests.
type plainErr string

func (p plainErr) Error() string { return string(p) }
