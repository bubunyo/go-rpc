package rpc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestError_Error(t *testing.T) {
	e := Error{
		Code:    500,
		Message: "Standard Error",
	}
	assert.Equal(t, "rpc error [500] Standard Error", e.Error())
}
