# go-rpc

[![Build Status](https://github.com/bubunyo/go-rpc/workflows/test-unit/badge.svg)](https://github.com/bubunyo/go-rpc/actions?query=branch%3Amaster+workflow%3Atest-unit)
![GoDoc](https://godoc.org/github.com/bubunyo/go-rpc?status.svg)

A simple resource for creating JSON RPC servers to comply with
the [JSON RPC Spec](https://www.jsonrpc.org/specification)

## Features

1. Bootstrap your json RPC Server with ease.
2. Handle multiple request concurrently.
3. Add your own custom error

## Example Usage

1. Setup a Ping service

```go
package main

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
```

You can consume a single ping service resource with this curl request

```shell
curl -X POST  localhost:8080/rpc \
-d '{
        "jsonrpc": "2.0",
        "id": null,
        "method": "PingService.Ping",
        "params": null
}'
```
with the following outpur
```shell
{
	"jsonrpc": "2.0",
	"id": null,
	"result": "ok"
}

```

or multiple resources using 
```shell
~ curl -X POST  localhost:8080/rpc -d '[{
        "jsonrpc": "2.0",
        "id": null,
        "method": "PingService.Ping",
        "params": null
}, {
        "jsonrpc": "2.0",
        "id": null,
        "method": "PingService.Ping",
        "params": null
}, {
        "jsonrpc": "2.0",
        "id": null,
        "method": "PingService.Ping",
        "params": null
}]'
```
with the following output 
```shell
[{
	"jsonrpc": "2.0",
	"id": null,
	"result": "ok"
}, {
	"jsonrpc": "2.0",
	"id": null,
	"result": "ok"
}, {
	"jsonrpc": "2.0",
	"id": null,
	"result": "ok"
}]
```
