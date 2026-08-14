---
title: Govern an agent's tools
weight: 1
---

# Govern an agent's tools

This guide walks through the core job of the SDK: taking the tools your agent
already has and making every call to them pass through governance. By the end
you'll have a runnable program that initialises the runtime, wraps a tool slice,
and calls a governed tool.

## 1. Make your tools satisfy the `Tool` interface

The SDK governs anything that satisfies its small `Tool` contract:

```go
type Tool interface {
    Name() string
    Description() string
    Call(ctx context.Context, input string) (string, error)
}
```

If your tools already have these three methods, they satisfy the interface as-is
— no embedding, no registration. Otherwise, write a thin adapter:

```go
type searchTool struct{}

func (searchTool) Name() string        { return "web_search" }
func (searchTool) Description() string  { return "search the public web" }
func (searchTool) Call(ctx context.Context, query string) (string, error) {
    // ... your real implementation ...
    return doSearch(ctx, query)
}
```

## 2. Initialise the runtime

```go
ctx := assembly.WithAgentID(context.Background(), "research-agent")

a, err := assembly.Init(ctx,
    assembly.WithGatewayURL("https://gateway.example.com"),
    assembly.WithAPIKey("..."), // optional for local dev
)
if err != nil {
    log.Fatalf("init: %v", err)
}
defer a.Close()
```

`WithAgentID` stamps this agent's identity onto the context so the gateway can
attribute every check to `research-agent`. The same identity is stamped onto the
record, which the client this SDK ships forwards to the runtime when one is
connected (AAASM-5750).

## 3. Wrap the tools

```go
tools := []assembly.Tool{searchTool{}}
governed := assembly.WrapTools(tools, client)
```

`WrapTools` returns a *new* slice the same length as the input, where each tool
is an `*AssemblyTool` that runs a policy `Check` before `Call` and a
`RecordResult` after. The `RecordResult` reaches an audit trail only if `client`
carries it onward; the one this SDK ships forwards it to a connected runtime and
has nowhere to send it without one (AAASM-5750).

- The second argument is your `GovernanceClient` (the thing that talks to the
  gateway). Under the default fail-closed enforce posture, passing `nil` denies
  every wrapped call (`ErrGovernanceUnavailable`) rather than running it
  unchecked — pass `assembly.WithFailClosed(false)` for a true passthrough
  wrapper (the tools run, no gateway calls) while you wire in a real client.
- You can pass per-wrap options, e.g. `assembly.WithFailClosed(true)`, to make
  governance failures block the call.

## 4. Hand the governed tools to your agent

Use `governed` everywhere you previously used the raw tools. From your agent's
point of view nothing changed — it still calls `Name()`, `Description()`, and
`Call()`:

```go
out, err := governed[0].Call(ctx, "latest go release")
if err != nil {
    // could be a *PolicyViolationError if the gateway denied the call
    log.Printf("tool call failed: %v", err)
    return
}
fmt.Println(out)
```

## Full program

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/ai-agent-assembly/go-sdk/assembly"
)

type searchTool struct{}

func (searchTool) Name() string       { return "web_search" }
func (searchTool) Description() string { return "search the public web" }
func (searchTool) Call(_ context.Context, query string) (string, error) {
    return "results for: " + query, nil
}

func main() {
    ctx := assembly.WithAgentID(context.Background(), "research-agent")

    a, err := assembly.Init(ctx,
        assembly.WithGatewayURL("https://gateway.example.com"),
        assembly.WithAPIKey("..."),
    )
    if err != nil {
        log.Fatalf("init: %v", err)
    }
    defer a.Close()

    governed := assembly.WrapTools([]assembly.Tool{searchTool{}}, nil)

    out, err := governed[0].Call(ctx, "latest go release")
    if err != nil {
        log.Fatalf("tool call: %v", err)
    }
    fmt.Println(out)
}
```

## Wrapping a single tool

There is no separate single-tool constructor — `WrapTools` is the only
entry point. For the occasional case where a framework hands you one tool at a
time, call `WrapTools` with a one-element slice and take the first result:

```go
wrapped := assembly.WrapTools([]assembly.Tool{innerTool}, client)[0]
```

## Next

- [Handle allow/deny decisions and errors]({{< relref "/guides/handle-decisions-and-errors" >}}) —
  what happens on a `deny`, a pending approval, or an unreachable gateway.
- [Integrate with a framework]({{< relref "/guides/framework-integration" >}}) — propagate agent
  lineage when one agent spawns another.
