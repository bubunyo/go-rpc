package rpc

import "fmt"

var (
	ParseError         = NewError(-32700, "Invalid JSON was received by the server")
	InvalidRequest     = NewError(-32600, "The JSON sent is not a valid Request object")
	MethodNotFound     = NewError(-32601, "The method does not exist / is not available")
	InvalidMethodParam = NewError(-32602, "Invalid method parameter(s)")
	InternalError      = NewError(-32602, "Internal JSON-RPC error")

	/* Server Errors
	-32000 to -32099	Server error	Reserved for implementation-defined server-errors.
	*/

	ExecutionTimeoutError    = NewError(-32001, "Execution Timeout")
	RequestBodyIsEmpty       = NewError(-32002, "Request body is empty")
	RequestBodyTooLargeError = NewError(-32003, "Request body too large")
	InvalidRpcVersion        = NewError(-32004, "Invalid RPC Version")
)

// Error is a JSON RPC Spec Error Object that hold the error code and the message.
// Spec - https://www.jsonrpc.org/specification#error_object
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewError creates and Error from a code and a message
func NewError(code int, msg string) Error {
	return Error{
		Code:    code,
		Message: msg,
	}
}

func (e Error) Error() string {
	return fmt.Sprintf("rpc error [%d] %s", e.Code, e.Message)
}
