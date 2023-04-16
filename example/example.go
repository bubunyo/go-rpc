package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/bubunyo/go-rpc"
)

type PingService struct{}

func (e PingService) Echo(_ context.Context, req *rpc.RequestParams) (any, error) {
	return "ok", nil
}

func main() {
	server := rpc.NewServer()
	server.ExecutionTimeout = 15 * time.Second // max time a function should execute for.
	server.MaxBytesRead = 1 << 20              // (1mb) - the maximum size of the total request payload

	ping := PingService{}
	pingService := rpc.NewService("PingService")
	pingService.RegisterMethod("Ping", ping.Echo)

	server.AddService(pingService)

	mux := http.NewServeMux()
	mux.Handle("/rpc", server)
	log.Fatalln(http.ListenAndServe(":8080", mux))
}
