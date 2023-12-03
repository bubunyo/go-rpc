# go-rpc

[![Build Status](https://github.com/bubunyo/go-rpc/workflows/test-unit/badge.svg)](https://github.com/bubunyo/go-rpc/actions?query=branch%3Amaster+workflow%3Atest-unit)
[![GoDoc](https://godoc.org/github.com/bubunyo/go-rpc?status.svg)](https://pkg.go.dev/github.com/bubunyo/go-rpc)

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

func (s PingService) Echo(_ context.Context, req *rpc.RequestParams) (any, error) {
	return "ok", nil
}

func (s PingService) Register() (string, rpc.RequestMap) {
	return "PingService", map[string]rpc.RequestFunc{
		"Ping": s.Echo,
	}
}

func main() {
  server := rpc.NewDefaultServer()
	server.AddService(PingService{})

	mux := http.NewServeMux()
	mux.Handle("/rpc", server)
	log.Fatalln(http.ListenAndServe(":8080", mux))
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
with the following output
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
