# go-sdk

Go SDK for Agent Assembly.

## Status

Project scaffold for ticket `AAASM-17`.

## Module

```go
module github.com/agent-assembly/go-sdk
```

## Layout

```text
assembly/
  init.go
  sidecar.go
  interceptor.go
examples/minimal/
```

## Quick Start

```go
import "github.com/agent-assembly/go-sdk/assembly"

func main() {
    _ = assembly.InitAssembly(assembly.Config{
        Gateway:        "https://your-gateway.com",
        APIKey:         "xxx",
        SidecarAddress: "127.0.0.1:50051",
    })
}
```

## Development

- `make fmt`
- `make lint`
- `make test`
