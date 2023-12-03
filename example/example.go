package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/bubunyo/go-rpc"
)

type PingService struct{}

func (s PingService) Echo(_ context.Context, req *rpc.RequestParams) (any, error) {
	return "ok", nil
}

func (s PingService) Register() (string, rpc.RequestMap) {
	return "PingService", map[string]rpc.RequestFunc{
		"Ping": s.Echo,
	}
}

func main() {
	// Create an rpc server
	server := rpc.NewServer(rpc.Opts{
		ExecutionTimeout: 15 * time.Second, // max time a function should execute for.
		MaxBytesRead:     1 << 20,          // (1mb) - the maximum size of the total request payload
	})
	// or use the default servver with
	// server := rpc.NewDefaultServer()

	server.AddService(PingService{})

	mux := http.NewServeMux()
	mux.Handle("/rpc", server)
	log.Fatalln(http.ListenAndServe(":8080", mux))
}
