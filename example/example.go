package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/bubunyo/go-rpc"
)

type PingService struct{}

func (s PingService) Echo(_ context.Context, _ *rpc.RequestParams) (string, error) {
	return "ok", nil
}

func (s PingService) Registry() *rpc.ServiceRegistry {
	r := rpc.NewRegistry("PingService")
	rpc.Handle(r, "Ping", s.Echo)
	return r
}

func main() {
	// Create an rpc server
	server := rpc.NewServer(rpc.Opts{
		ExecutionTimeout: 15 * time.Second, // max time a function should execute for.
		MaxBytesRead:     1 << 20,          // (1mb) - the maximum size of the total request payload
	})
	// or use the default server with
	// server := rpc.NewDefaultServer()

	server.Register(PingService{})

	mux := http.NewServeMux()
	mux.Handle("/rpc", server)
	log.Fatalln(http.ListenAndServe(":8080", mux))
}
