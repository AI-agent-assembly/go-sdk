# go-sdk

[![Go Reference](https://pkg.go.dev/badge/github.com/agent-assembly/go-sdk.svg)](https://pkg.go.dev/github.com/agent-assembly/go-sdk)
[![Go Report Card](https://goreportcard.com/badge/github.com/agent-assembly/go-sdk)](https://goreportcard.com/report/github.com/agent-assembly/go-sdk)
[![Codecov](https://codecov.io/gh/AI-agent-assembly/go-sdk/graph/badge.svg)](https://codecov.io/gh/AI-agent-assembly/go-sdk)

Go SDK for Agent Assembly.

## Status

Runtime architecture scaffold for ticket `AAASM-63`.

## Module

```go
module github.com/agent-assembly/go-sdk
```

## Layout

```text
assembly/
  init.go
  runtime.go
  options.go
  defaults.go
  validation.go
  governance_client.go
  policy_model.go
  governance_errors.go
  tool_wrapper.go
  wrap_tools.go
  sidecar.go
  interceptor.go
examples/minimal/
```

## Quick Start

```go
import (
    "context"

    "github.com/agent-assembly/go-sdk/assembly"
)

func main() {
    runtime := assembly.NewAssembly(
        assembly.WithGatewayURL("https://your-gateway.com"),
        assembly.WithAPIKey("xxx"),
        assembly.WithFailClosed(false),
    )

    _ = runtime.Init(context.Background())
    defer runtime.Close()
}
```

## Development

- `make fmt`
- `make lint`
- `make test`

## FFI Transport

- CGo bridge module lives in `internal/ffi/cgo_bridge.go` with `#cgo LDFLAGS: -laa_ffi_go`.
- Native FFI path is enabled with build tags: `-tags aa_ffi_go` (and `CGO_ENABLED=1`).
- Pure-Go UDS fallback is selected automatically when `aa_ffi_go` tag is not set.
- `CGO_ENABLED=0` is explicitly supported via fallback transport and CI matrix coverage.
- Optional memory harness test (1M sends): `AAASM_MEMORY_HARNESS=1 go test ./internal/ffi -run TestMemoryRegressionHarness`.

## SonarQube CI Setup

Configure these repository settings for the `SonarQube` workflow:

- Secret: `SONAR_TOKEN`
- Variable: `SONAR_HOST_URL` (for SonarCloud use `https://sonarcloud.io`)
- Variable: `SONAR_PROJECT_KEY`
- Variable: `SONAR_ORGANIZATION` (required for SonarCloud, optional for self-hosted SonarQube)
