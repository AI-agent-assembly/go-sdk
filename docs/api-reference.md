---
title: API Reference
weight: 5
---

# API Reference

The **authoritative, version-pinned** API reference for `go-sdk` lives on
**[pkg.go.dev](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk)**. It is
auto-generated from the godoc comments in the source on every released tag, so
there's no separate publish step to maintain — push a `vX.Y.Z` tag and
pkg.go.dev renders it within minutes. That page is the source of truth for every
exported symbol, its full signature, and its godoc.

The summary below is a curated map of the **key public API** so you can find the
right entry point fast. Signatures here are quoted from the `assembly` package
(`github.com/ai-agent-assembly/go-sdk/assembly`); pkg.go.dev has the rest.

## Entry point

```go
func Init(ctx context.Context, options ...Option) (*Assembly, error)
```

Configures and boots the runtime, resolving the gateway URL and API key, then
registering the agent with the gateway. Returns the `*Assembly` handle. See
[Configuration]({{< relref "/configuration" >}}) for the resolution order.

```go
type Assembly struct{ /* unexported */ }

func (a *Assembly) Close() error
```

`Assembly` is the runtime handle. `Close` releases the runtime and stops any
managed sidecar; always `defer a.Close()`.

## Tool governance

```go
type Tool interface {
    Name() string
    Description() string
    Call(ctx context.Context, input string) (string, error)
}

func WrapTools(toolList []Tool, client GovernanceClient, options ...Option) []Tool
```

`WrapTools` returns a new slice the same length as `toolList`, where each tool is
governed: a policy `Check` runs before `Call`, a `RecordResult` after. Whether
that record is *retained* is a property of `client`, not of the wrapper — the
client this SDK ships drops it (`Assembly.AuditSink()` reports which you have,
AAASM-5731). Pass `nil` for `client` to get a passthrough wrapper.

```go
type AssemblyTool struct{ /* unexported */ }
```

`AssemblyTool` is the concrete governed-tool type `WrapTools` returns behind
the `Tool` interface; it has no exported constructor. There is no single-tool
path separate from `WrapTools` — to wrap one tool, call `WrapTools` with a
one-element slice and take the first result.

## The governance client contract

```go
type GovernanceClient interface {
    Check(ctx context.Context, request CheckRequest) (Decision, error)
    WaitForApproval(ctx context.Context, request ApprovalRequest) (Decision, error)
    RecordResult(ctx context.Context, request RecordRequest) error
    Close() error
}

type Decision struct {
    Denied  bool
    Pending bool
    Reason  string
}
```

The wrapper calls this interface around each tool. `CheckRequest`,
`ApprovalRequest`, and `RecordRequest` are the request payloads (see
`policy_model.go` / pkg.go.dev for fields).

## Options

All configuration is functional options of type `Option`:

```go
type Option func(*runtimeOptions)
```

| Option | Signature |
|---|---|
| `WithGatewayURL` | `func(gatewayURL string) Option` |
| `WithAPIKey` | `func(apiKey string) Option` — optional; empty is allowed |
| `WithFailClosed` | `func(failClosed bool) Option` |
| `WithTimeout` | `func(timeout time.Duration) Option` |
| `WithEnforcementMode` | `func(mode EnforcementMode) Option` |
| `WithSelfAgentID` | `func(agentID string) Option` |
| `WithParentAgentID` | `func(parentAgentID string) Option` |
| `WithTeamID` | `func(teamID string) Option` |
| `WithDelegationReason` | `func(reason string) Option` — ≤ 256 chars |
| `WithSpawnedByTool` | `func(tool string) Option` |
| `WithSidecarBinary` | `func(path string) Option` |

See [Configuration]({{< relref "/configuration" >}}) for defaults and behaviour.

## Enforcement modes

```go
type EnforcementMode string

const (
    EnforcementModeEnforce  EnforcementMode = "enforce"
    EnforcementModeObserve  EnforcementMode = "observe"
    EnforcementModeDisabled EnforcementMode = "disabled"
)
```

## Context helpers

```go
func WithAgentID(ctx context.Context, agentID string) context.Context
func AgentIDFromContext(ctx context.Context) string

func WithTraceID(ctx context.Context, traceID string) context.Context
func TraceIDFromContext(ctx context.Context) string

func WithRunID(ctx context.Context, runID string) context.Context
func RunIDFromContext(ctx context.Context) string
func EnsureRunID(ctx context.Context) (context.Context, string)
```

## Interceptors

```go
func HTTPMiddleware(next http.RoundTripper) http.RoundTripper
func UnaryClientInterceptor() grpc.UnaryClientInterceptor
func StreamClientInterceptor() grpc.StreamClientInterceptor
```

## Framework integration

```go
type Chain interface {
    Call(ctx context.Context, inputs map[string]any) (map[string]any, error)
}

func WrapChain(a *Assembly, chain Chain) Chain
```

See [Integrate with a framework]({{< relref "/guides/framework-integration" >}}).

## Errors

```go
var ErrInvalidGateway        error // no gateway URL resolved
var ErrRuntimeNotInitialized error // used before Init / after Close
var ErrSidecarUnavailable    error // sidecar mode, no reachable sidecar
var ErrBinaryNotFound        error // aasm / sidecar binary missing

type PolicyViolationError struct { ToolName, Reason string } // gateway denied
type ConfigurationError    struct { Message string }         // can't resolve gateway
type GatewayError          struct { Message string }         // gateway unreachable
```

Match sentinels with `errors.Is` and structured types with `errors.As`. See
[Handle allow/deny decisions and errors]({{< relref "/guides/handle-decisions-and-errors" >}}) and
[Troubleshooting]({{< relref "/troubleshooting" >}}).

## Local preview

To read the same godoc against your working tree offline:

```bash
go install golang.org/x/tools/cmd/godoc@latest
godoc -http=:6060
```

Then open
<http://localhost:6060/pkg/github.com/ai-agent-assembly/go-sdk/assembly/>.

## See also

- [Core Concepts]({{< relref "/core-concepts" >}}) — *why* these APIs are shaped the way they are.
- [Quick Start]({{< relref "/quick-start" >}}) — install, init, wrap your tools.
- [Contributing](https://github.com/ai-agent-assembly/go-sdk/blob/main/CONTRIBUTING.md) —
  the conventions enforced in review (context-first, `%w` wrapping, `io.Closer`,
  functional options, `internal/` boundary).
