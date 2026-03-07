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

// Describe implements rpc.ServiceDescriptor, providing AI-readable metadata so
// that AI agents can discover and understand this service's methods via
// the built-in rpc.discover call.
func (s PingService) Describe() map[string]rpc.MethodMeta {
	return map[string]rpc.MethodMeta{
		"Ping": {
			Description: "Pings the server. Returns 'ok' when the service is healthy.",
			Params:      nil,
			Result:      map[string]any{"type": "string", "example": "ok"},
		},
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

	// AI agents can now discover all methods by calling the built-in rpc.discover method:
	//
	//   POST /rpc
	//   {"jsonrpc":"2.0","id":1,"method":"rpc.discover","params":null}
	//
	// Response:
	//   {"jsonrpc":"2.0","id":1,"result":[
	//     {"name":"PingService.Ping","description":"Pings the server...","result":{"example":"ok","type":"string"}}
	//   ]}

	mux := http.NewServeMux()
	mux.Handle("/rpc", server)
	log.Fatalln(http.ListenAndServe(":8080", mux))
}
