# go-sdk

[![Go Reference](https://pkg.go.dev/badge/github.com/agent-assembly/go-sdk.svg)](https://pkg.go.dev/github.com/agent-assembly/go-sdk)
[![Go Report Card](https://goreportcard.com/badge/github.com/agent-assembly/go-sdk)](https://goreportcard.com/report/github.com/agent-assembly/go-sdk)
[![Codecov](https://codecov.io/gh/AI-agent-assembly/go-sdk/graph/badge.svg)](https://codecov.io/gh/AI-agent-assembly/go-sdk)

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

## SonarQube CI Setup

Configure these repository settings for the `SonarQube` workflow:

- Secret: `SONAR_TOKEN`
- Variable: `SONAR_HOST_URL` (for SonarCloud use `https://sonarcloud.io`)
- Variable: `SONAR_PROJECT_KEY`
- Variable: `SONAR_ORGANIZATION` (required for SonarCloud, optional for self-hosted SonarQube)
