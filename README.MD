# go-rpc

[![Build Status](https://github.com/bubunyo/go-rpc/workflows/test-unit/badge.svg)](https://github.com/bubunyo/go-rpc/actions?query=branch%3Amaster+workflow%3Atest-unit)
![GoDoc](https://godoc.org/github.com/bubunyo/go-rpc?status.svg)
[![Coverage Status](https://codecov.io/gh/bubunyo/go-rpc/branch/master/graph/badge.svg)](https://codecov.io/gh/bubunyo/go-rpc)
[![GoDevDoc](https://img.shields.io/badge/dev-doc-00ADD8?logo=go)](https://pkg.go.dev/github.com/bubunyo/go-rpc)
![Code lines](https://sloc.xyz/github/bubunyo/go-rpc/?category=code)

A simple resource for creating JSON RPC servers to comply with
the [JSON RPC Spec](https://www.jsonrpc.org/specification)

## Features

1. Bootstrap your json RPC Server with ease.
2. Handle multiple request concurrently with ease.
3. Add your own custom error

## Example Usage

```go
// 1. Create you service
type EchoService struct{}

func (e EchoService) Echo(_ context.Context, req *rpc.RequestParams) (any, error) {
return "echo: " + string(req.Payload),
}

// 2. Create Server
server := rpc.NewServer()
server.ExectuionTimeout = 15 * time.Second // max time a function should execute for.
server.MaxBytesRead = 1 << 20 // (1mb) - the maximum size of the total request payload 

// 3. Initiate Echo Service and register a method
echo := EchoService{}
echoService := rpc.NewService("EchoService")
echoService.RegisterMethod("Ping", echo.Ping)

// 4. Add Echo Service to rpc server
server.AddService(echoService)

// 5. Create and start your server. 
mux := http.NewServeMux()
mux.Handle("/rpc", server)
log.Fatalln(http.ListenAndServe(":8080", mux))
```
