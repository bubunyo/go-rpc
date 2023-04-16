package rpc

import (
	"context"
)

type TestService struct {
	ProcessFn func(_ context.Context, req *RequestParams) (any, error)
}

func NewTestService(ts *TestService) *Service {
	s := NewService("TestService")
	s.RegisterMethod("Exec", ts.Exec)
	return s
}

func (s TestService) MethodName() string {
	return "TestService.Exec"
}

func (s TestService) Exec(ctx context.Context, req *RequestParams) (any, error) {
	return s.ProcessFn(ctx, req)
}
