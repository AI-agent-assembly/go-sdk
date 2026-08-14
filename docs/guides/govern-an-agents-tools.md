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
switch {
case errors.Is(err, assembly.ErrSidecarUnavailable):
    // Expected on the default pure-Go build — see the full program below.
    log.Println("init:", err)
case err != nil:
    log.Fatalf("init: %v", err)
default:
    defer func() { _ = a.Close() }()
}
```

On the default pure-Go build `Init` returns `ErrSidecarUnavailable` whatever
options you pass, because that build links no native transport. Step 3 is
unaffected: the wrapper routes each call through the client you hand
`WrapTools`, not through the runtime handle.

`WithAgentID` stamps this agent's identity onto the context so the gateway can
attribute every check to `research-agent`. The same identity is stamped onto the
record, but the client this SDK ships drops the record, so there is nothing to
attribute on that side (AAASM-5731).

## 3. Wrap the tools

```go
tools := []assembly.Tool{searchTool{}}
governed := assembly.WrapTools(tools, client)
```

`WrapTools` returns a *new* slice the same length as the input, where each tool
is an `*AssemblyTool` that runs a policy `Check` before `Call` and a
`RecordResult` after. The `RecordResult` reaches an audit trail only if `client`
retains it; the one this SDK ships does not (AAASM-5731).

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
    "errors"
    "fmt"
    "log"

    "github.com/ai-agent-assembly/go-sdk/assembly"
)

type searchTool struct{}

func (searchTool) Name() string        { return "web_search" }
func (searchTool) Description() string { return "search the public web" }
func (searchTool) Call(_ context.Context, query string) (string, error) {
    return "results for: " + query, nil
}

// policyClient is the GovernanceClient step 3 wraps with. It decides in-process
// here so this program runs with no gateway; a deployment swaps in a
// gateway-backed client and leaves the WrapTools call untouched.
type policyClient struct{}

func (policyClient) Check(_ context.Context, req assembly.CheckRequest) (assembly.Decision, error) {
    if req.ToolName != "web_search" {
        return assembly.Decision{Denied: true, Reason: "only web_search is allowed here"}, nil
    }
    return assembly.Decision{Reason: "allowed by the in-process stand-in"}, nil
}

func (policyClient) WaitForApproval(_ context.Context, _ assembly.ApprovalRequest) (assembly.Decision, error) {
    return assembly.Decision{}, nil
}

func (policyClient) RecordResult(_ context.Context, _ assembly.RecordRequest) error { return nil }

func (policyClient) Close() error { return nil }

func main() {
    ctx := assembly.WithAgentID(context.Background(), "research-agent")

    a, err := assembly.Init(ctx,
        assembly.WithGatewayURL("https://gateway.example.com"),
        assembly.WithAPIKey("..."),
    )
    switch {
    case errors.Is(err, assembly.ErrSidecarUnavailable):
        // Expected on the default pure-Go build: it links no native transport,
        // so boot reaches no runtime. The wrapper below is unaffected — it
        // routes each call through the client passed to WrapTools.
        log.Println("init:", err)
    case err != nil:
        log.Fatalf("init: %v", err)
    default:
        defer func() { _ = a.Close() }()
    }

    tools := []assembly.Tool{searchTool{}}
    governed := assembly.WrapTools(tools, policyClient{})

    out, err := governed[0].Call(ctx, "latest go release")
    if err != nil {
        log.Fatalf("tool call: %v", err)
    }
    fmt.Println(out) // results for: latest go release
}
```

Run against a default `go get` install, this program reports
`ErrSidecarUnavailable`, prints `results for: latest go release`, and exits 0.
A drift gate in `assembly/documented_programs_test.go` executes it, so it cannot
rot back into a program that only looks runnable.

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
