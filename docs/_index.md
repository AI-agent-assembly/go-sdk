---
title: go-sdk
toc: false
---

# go-sdk · AI Agent Assembly

AI agents take real actions — calling APIs, running code, spending money, touching
data. This SDK lets a team decide, in advance, which of those actions an agent is
allowed to take. It is the Go on-ramp to that control.

The **Go SDK for AI Agent Assembly** lets you put a governance checkpoint in
front of the tools your AI agent calls — without rewriting the agent. You
initialise the runtime once, wrap your tool slice, and from then on every tool
call is checked against your gateway's policy *before* it runs, and its outcome
is offered to the governance client *after* it finishes.

{{< callout type="warning" >}}
**The SDK hands the record to the runtime; it does not give you an audit trail.**
The wrapper offers the outcome of every governed call — allowed or denied — to
`GovernanceClient.RecordResult`, and the client this SDK ships writes it to the
native event channel, the same one the boot registration event uses.

Three things stand between that and evidence you could cite, and none of them is
under this SDK's control: the send is **unacknowledged**, so arrival is never
reported back; the dispatch runs on a goroutine nothing joins, so the
`defer a.Close()` this guide recommends **loses the record deterministically**
(measured: 200 of 200); and the native binding is compiled only under
`-tags aa_ffi_go`, which no published artifact ships — so on a default build
there is no channel at all.

`Init` warns when no record can be sent, and `Assembly.AuditSink()` reports which
case a run is in
([AAASM-5750](https://lightning-dust-mite.atlassian.net/browse/AAASM-5750)).
{{< /callout >}}

It is written in idiomatic Go: functional options, context-first APIs, typed
errors, and a pure-Go default that builds with `CGO_ENABLED=0`.

## What it is

Concretely, the SDK is two things working together:

- **A thin governance client.** It opens one connection to the AI Agent
  Assembly **gateway** (the policy brain, which lives in the
  [agent-assembly](https://github.com/ai-agent-assembly/agent-assembly) core
  repo) and speaks its wire protocol over gRPC/HTTP — or, in local
  development, auto-discovers and starts a gateway for you.
- **An in-process interception shim.** `WrapTools` decorates your existing
  `Tool` values so each `Call` runs a policy `Check` first and offers the outcome
  to `RecordResult` after (which the shipped client hands to a connected runtime
  — see above). Your agent code keeps calling tools the way it always did; the
  wrapper does the governance.

For the platform as a whole — what the gateway is, how policy and budgets are
authored, and how the three interception layers fit together — see the
[core agent-assembly documentation](https://docs.agent-assembly.com/core/)
and the shared [docs hub](https://docs.agent-assembly.com/).

## Who it's for

- **Go developers** building or operating AI agents who need allow/deny, budget,
  and topology governance over what their agents can do — and want to add it as a
  library, not a rewrite. Note that this SDK layer does not deliver an audit
  trail (see above).
- **Platform teams** standardising agent governance across services: the same
  gateway and policy back several languages (there are sibling
  [Python](https://docs.agent-assembly.com/python-sdk/) and
  [Node](https://docs.agent-assembly.com/node-sdk/) SDKs), so a Go service
  joins the same control plane.

## Quick look

```go
package main

import (
    "context"
    "log"

    "github.com/ai-agent-assembly/go-sdk/assembly"
)

func main() {
    ctx := assembly.WithAgentID(context.Background(), "my-agent")

    a, err := assembly.Init(ctx,
        assembly.WithGatewayURL("https://gateway.example.com"),
        assembly.WithAPIKey("..."), // optional for local dev
    )
    if err != nil {
        log.Fatal(err)
    }
    defer a.Close()

    governed := assembly.WrapTools(myTools, nil)
    _ = governed // hand these to your agent in place of the originals
}
```

[Get started in 3 steps →]({{< relref "/quick-start" >}})

## Documentation map

| Section | What's inside |
|---|---|
| [Quick Start]({{< relref "/quick-start" >}}) | Install, configure, and govern your first agent's tools — copy-paste. |
| [Core Concepts]({{< relref "/core-concepts" >}}) | How the SDK talks to the gateway, the client lifecycle, modes, and enforcement. |
| [Guides]({{< relref "/guides" >}}) | Task-first walkthroughs: wrap an agent's tools, integrate a framework, handle allow/deny and errors. |
| [Examples]({{< relref "/examples" >}}) | Runnable end-to-end examples: basic agent, tool-policy, LangChainGo, and `aasm` CLI runtime integration. |
| [Configuration]({{< relref "/configuration" >}}) | Gateway/API-key resolution, every `Init` option, enforcement modes, context helpers. |
| [API Reference]({{< relref "/api-reference" >}}) | The authoritative godoc on pkg.go.dev, plus a curated summary of the key exported API. |
| [Compatibility & Versioning]({{< relref "/compatibility" >}}) | Gateway protocol pin, the core↔SDK matrix, toolchain floor, and the release process. |
| [Troubleshooting]({{< relref "/troubleshooting" >}}) | Typed errors, timeouts, build/transport gotchas, and where to get help. |

> Pure-Go by default (`CGO_ENABLED=0`); the native FFI transport is opt-in via
> `-tags aa_ffi_go`. See [Core Concepts]({{< relref "/core-concepts#the-ffi-transport-bridge" >}}).
