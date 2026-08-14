---
title: Quick Start
weight: 1
---

# Quick Start

This walkthrough takes you from zero to a governed tool call in three steps:
install the SDK, initialise the runtime, and wrap your tools so every call is
checked against the AI Agent Assembly gateway. The whole thing is a single
`main` you can copy, paste, and run.

{{< callout type="warning" >}}
**Agent registration is not reachable from a plain `go get` today — following
this quick-start will not make your agent appear in the dashboard.** The steps
below wrap and govern tool calls, but the register handshake runs *only* under
the opt-in native cgo binding (`-tags aa_ffi_go`, `CGO_ENABLED=1`), and that
native library (`libaa_ffi_go`) is **not published anywhere** yet: building with
`-tags aa_ffi_go` fails with `ld: library 'aa_ffi_go' not found` outside a full
monorepo checkout. The default pure-Go build has no native transport, so it does
not register even when [`WithSidecarAddress`](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk/assembly#WithSidecarAddress)
is set (see that option's godoc). Publishing the native library — or dropping the
cgo requirement — is a separate product decision; track status in
[AAASM-4547](https://lightning-dust-mite.atlassian.net/browse/AAASM-4547) and
[AAASM-4469](https://lightning-dust-mite.atlassian.net/browse/AAASM-4469).
{{< /callout >}}

## Prerequisites

- **Go** ≥ 1.26 (the floor declared in `go.mod`).
- For **local development**: nothing else — `Init` auto-discovers a gateway on
  `http://localhost:7391`, and starts one for you if none is running (the
  [`aasm` CLI](https://github.com/ai-agent-assembly/agent-assembly) must be on
  your `PATH`).

  {{< callout type="note" >}}
  **Local-mode transports — `:7391` REST + `:50051` gRPC.** `Init` shells out
  to the following command to auto-start the gateway:

  ```bash
  aasm start --mode local --foreground
  ```

  The `:7391` auto-discovery above only resolves the REST gateway URL. Agent
  **registration** is a separate concern that talks to the gateway's gRPC
  endpoint (default `127.0.0.1:50051`) — `Init` does **not** auto-derive this
  address the way the Python and Node SDKs do. Reaching it requires an
  explicit
  [`WithSidecarAddress`](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk/assembly#WithSidecarAddress)
  (or `WithSidecarBinary`) option; without one, `Init` returns
  `ErrSidecarUnavailable`. And per the warning at the top of this page, the
  registration handshake itself only runs under the opt-in native cgo binding
  today.

  To confirm both surfaces are actually up rather than guessing from `Init`'s
  behavior, check them directly:

  ```bash
  curl http://localhost:7391/healthz   # REST — real JSON: mode, storage, version, uptime_secs
  nc -z localhost 50051 && echo "gRPC port open"   # gRPC has no health endpoint yet; this only confirms the port accepts connections
  ```
  {{< /callout >}}
- For **production**: a gateway URL and, if your gateway requires auth, an API
  key. Both can come from options, environment variables, or a config file —
  see [Configuration]({{< relref "/configuration" >}}).
- *(Optional)* a C compiler, only if you opt into the native FFI transport with
  `-tags aa_ffi_go`. The default transport is pure-Go and needs none.

## Step 1 — Install

```bash
go get github.com/ai-agent-assembly/go-sdk
```

## Step 2 — Initialise the runtime

`Init` returns an `*assembly.Assembly` — your runtime handle. Always `Close` it
when you're done so the connection (and any managed sidecar) is released.

```go
package main

import (
    "context"
    "log"

    "github.com/ai-agent-assembly/go-sdk/assembly"
)

func main() {
    // Stamp this agent's identity onto the context. The SDK stamps it onto
    // every check and record; the check is what reaches the gateway.
    ctx := assembly.WithAgentID(context.Background(), "my-agent")

    a, err := assembly.Init(ctx,
        assembly.WithGatewayURL("https://gateway.example.com"),
        assembly.WithAPIKey("..."), // optional — omit for local, unauthenticated dev
    )
    if err != nil {
        log.Fatalf("init assembly runtime: %v", err)
    }
    defer func() {
        if err := a.Close(); err != nil {
            log.Printf("close assembly runtime: %v", err)
        }
    }()

    log.Println("assembly runtime ready")
}
```

For **local development** you can drop both options entirely — `assembly.Init(ctx)`
resolves the gateway from the environment, then `~/.aasm/config.yaml`, then the
local default. See [Configuration]({{< relref "/configuration#gateway-and-credential-resolution" >}})
for the full resolution order.

## Step 3 — Wrap your tools

Your tools just need to satisfy the SDK's small `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Call(ctx context.Context, input string) (string, error)
}
```

`WrapTools` takes your `[]Tool` and a governance client, and returns a new
`[]Tool` where every `Call` is governed:

```go
governed := assembly.WrapTools(myTools, nil)
```

The second argument is the `GovernanceClient` that talks to the gateway. Under
the default fail-closed enforce posture, passing `nil` denies every wrapped call
(`ErrGovernanceUnavailable`) rather than running it unchecked — pass
`assembly.WithFailClosed(false)` for a true passthrough wrapper (the tools run,
no `Check`/`RecordResult` calls) while you wire in a real client, ready to
enforce policy (see
[Handle allow/deny decisions and errors]({{< relref "/guides/handle-decisions-and-errors" >}})).

Hand `governed` to your agent in place of the originals. From here on, each call
against a governed tool is checked against the gateway policy before execution,
and its outcome is offered to `RecordResult` after — the client this SDK ships
writes that record to the runtime's native event channel, which is a handoff and
not an audit guarantee. The send is unacknowledged, and because the dispatch is
never joined, a `defer a.Close()` immediately after a call loses the record every
time (inspect `Assembly.AuditSink()` to tell which run you are in, AAASM-5750).
Downstream of the handoff, [AAASM-5783](https://lightning-dust-mite.atlassian.net/browse/AAASM-5783)
is open on `report_event` payloads reaching neither the live stream nor the durable
entry, so no SDK can claim ADR 0033 §6 *Observed* until it lands.

### Govern your first agent

Pick your framework — each tab shows the governance slice copied verbatim from a
runnable example in the
[examples repo](https://github.com/ai-agent-assembly/examples/tree/HEAD/go)
(a CI drift check keeps them in lockstep). Go's per-framework
surface is thin today, so the tabs are **LangChainGo** (the framework path) and
**Plain** (the framework-agnostic path). Two more validated Go examples already
exist — **Tool Policy** and **CLI Runtime (sidecar)**. Those are patterns
(an allow/deny policy demo and sidecar wiring), not "first agent" frameworks.
They're intentionally left out of this quick-start; see
`metadata/quickstart/README.md` for the tab-selection rationale. A new tab
appears automatically once a new Go **framework** example lands.

<!-- BEGIN GENERATED: quickstart-tabs -->
<!-- GENERATED BY scripts/gen-quickstart-tabs.go — DO NOT EDIT. -->
<!-- Source data: metadata/quickstart/ (vendored from the examples repo). -->
{{< tabs >}}
{{< tab name="LangChainGo" >}}

_Governance slice from the runnable `go/langchaingo/main.go` example._

```go
// Wrap the LangChainGo tools with Agent Assembly governance. The wrapped
// values still satisfy langchaingo's tools.Tool, so they can be handed
// straight to a LangChainGo agent/executor.
governed := assembly.WrapTools(
	[]assembly.Tool{&searchTool{}, &sendEmailTool{}},
	&policyClient{},
)
```

{{< /tab >}}
{{< tab name="Plain" >}}

_Governance slice from the runnable `go/basic-agent/main.go` example._

```go
ctx := assembly.WithAgentID(context.Background(), "basic-agent-demo")

// Use the offline mock governance client.
// In production, replace mockClient with a client backed by a real gateway.
fmt.Println("[assembly] using offline mock governance client")
client := &mockClient{}

// Wrap the tool — every Call now goes through the governance client first.
tools := assembly.WrapTools([]assembly.Tool{&echoTool{}}, client)
```

{{< /tab >}}
{{< /tabs >}}
<!-- END GENERATED: quickstart-tabs -->

## Putting it together

```go
package main

import (
    "context"
    "log"

    "github.com/ai-agent-assembly/go-sdk/assembly"
)

// echoTool is a minimal Tool implementation.
type echoTool struct{}

func (echoTool) Name() string       { return "echo" }
func (echoTool) Description() string { return "returns its input unchanged" }
func (echoTool) Call(_ context.Context, input string) (string, error) {
    return input, nil
}

func main() {
    ctx := assembly.WithAgentID(context.Background(), "my-agent")

    a, err := assembly.Init(ctx,
        assembly.WithGatewayURL("https://gateway.example.com"),
        assembly.WithAPIKey("..."),
    )
    if err != nil {
        log.Fatalf("init: %v", err)
    }
    defer a.Close()

    tools := []assembly.Tool{echoTool{}}
    governed := assembly.WrapTools(tools, nil)

    out, err := governed[0].Call(ctx, "hello, governance")
    if err != nil {
        log.Fatalf("tool call: %v", err)
    }
    log.Println("result:", out) // result: hello, governance
}
```

### What to expect

- **`Init` succeeds** once a gateway is reachable (resolved or auto-started). If
  no gateway can be found and no `aasm` binary is on `PATH`, you'll get a typed
  `*assembly.ConfigurationError` — see [Troubleshooting]({{< relref "/troubleshooting" >}}).
- **Tool calls run** and return the inner tool's result. With a real governance
  client wired in, a `deny` decision surfaces as a
  `*assembly.PolicyViolationError` and the inner tool never runs.

## Where to next

- [Core Concepts]({{< relref "/core-concepts" >}}) — what's actually happening inside the SDK.
- **[Examples]({{< relref "/examples" >}})** — wire the SDK into the framework you actually use.
- [Guides]({{< relref "/guides" >}}) — wrap a real agent, integrate a framework, handle decisions.
- [Configuration]({{< relref "/configuration" >}}) — every `Init` option, defaults, and enforcement modes.
- [Troubleshooting]({{< relref "/troubleshooting" >}}) — what to do when `Init` or a check fails.
