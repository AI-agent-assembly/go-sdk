---
title: Quick Start
weight: 1
---

# Quick Start

This is the complete, guided arc for the Go SDK: from `go get` to a governed tool
call you can watch get **allowed**, **denied**, and **held for approval** — then
where those actions show up for an operator, and how to change the policy that
drives them. Follow it top to bottom the first time through; each step links to
the page that owns the detail.

The same journey exists in the
[Python](https://docs.agent-assembly.com/python-sdk/) and
[Node](https://docs.agent-assembly.com/node-sdk/) SDKs — whichever language you
standardise on, the shape is identical.

## What you'll build

A **governed AI agent**: you wrap your existing tools once, and from then on every
tool call runs a policy **check before it executes** and a **record after it
finishes** — without changing the agent's logic. The governance is a checkpoint in
front of your tools, not a rewrite of your agent.

The SDK is two halves, and it's worth knowing which is which before you start:

| Half | What it does | Status today (published `go get`) |
|---|---|---|
| **Interception** — `WrapTools` + your `GovernanceClient` + the typed `Decision`/error model | Decides allow / deny / approval in front of each tool call | ✅ Works now, in-process, `CGO_ENABLED=0` |
| **Transport** — `Init` connecting to a live gateway, agent registration, dashboard visibility | Wires the interception half to a real gateway over the network | ⚠️ Not reachable from published artifacts yet — see [Current status](#current-status--limitations) |

This Quick Start walks the **interception half** with genuinely runnable code, then
is honest about the **transport half** at the step where it matters.

{{< callout type="warning" >}}
**Read [Current status & limitations](#current-status--limitations) before you rely
on a live gateway.** On the published pure-Go build, `assembly.Init` returns
`ErrSidecarUnavailable` — the real gateway transport ships in an opt-in native
binding that is not published yet. Everything in Steps 1–6 runs today; Steps 7–9
describe the operator surface those actions reach *once the transport lands*.
{{< /callout >}}

## Prerequisites

- **Go** ≥ 1.26 (the floor declared in `go.mod`).
- Nothing else to run Steps 1–6 — the governance client is in-process, so no
  gateway, API key, or C compiler is required.
- *(For the live-gateway path only)* a gateway URL and, if it requires auth, an API
  key — see [Configuration]({{< relref "/configuration" >}}). Note the current
  limitation below before wiring it in.

## Step 1 — Install

The default build is pure-Go and compiles with `CGO_ENABLED=0`:

```bash
go get github.com/ai-agent-assembly/go-sdk
```

## Step 2 — Connect (the runtime handle)

`Init` returns an `*assembly.Assembly` — your runtime handle — and is where you
point the SDK at a gateway and stamp your agent's identity. Always `Close` it when
you're done.

```go
package main

import (
    "context"
    "log"

    "github.com/ai-agent-assembly/go-sdk/assembly"
)

func main() {
    // Stamp this agent's identity onto the context. The SDK forwards it to the
    // gateway on every check and record.
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

The gateway URL and API key resolve from options, then environment
(`AA_GATEWAY_URL`), then `~/.aasm/config.yaml` — see
[Configuration]({{< relref "/configuration#gateway-and-credential-resolution" >}})
for the full order.

{{< callout type="warning" >}}
**On the published pure-Go build this `Init` returns `ErrSidecarUnavailable`.** The
default build has no native transport, so there is no reachable runtime to connect
to — see [Current status & limitations](#current-status--limitations). The
interception half below does **not** depend on `Init`: you can build and run a fully
governed tool call today by constructing a `GovernanceClient` directly, which is
exactly what Steps 3–5 do.
{{< /callout >}}

Once your agent registers with a running gateway, it appears in the operator dashboard's
**Overview** right away — the fleet count ticks up and the three-layer posture goes live.
(On the published pure-Go build registration isn't reachable yet — see [Current status &
limitations](#current-status--limitations); the view below is the surface your Go agent
lands in once it is.)

![Agent Assembly operator dashboard — Overview, light theme: three-layer posture rings above a fleet snapshot reading six total agents, four enforcing, two shadow, none flagged.](/images/dashboard/overview-light.png "Overview (light theme): posture rings and the fleet snapshot — where a newly registered agent shows up immediately. Shown with sample fixture data.")

![Agent Assembly operator dashboard — Overview, dark theme: the same posture rings and fleet snapshot re-themed to dark.](/images/dashboard/overview-dark.png "Overview (dark theme): the same at-a-glance registered / enforcing / flagged summary. Shown with sample fixture data.")

## Step 3 — Wrap a tool and watch a call get allowed

Your tools only need to satisfy the SDK's small `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Call(ctx context.Context, input string) (string, error)
}
```

`WrapTools` takes your `[]Tool` and a `GovernanceClient`, and returns a new
`[]Tool` where every `Call` is governed — a `Check` runs before the inner tool and
a `RecordResult` after:

```go
governed := assembly.WrapTools(myTools, client)
```

The `GovernanceClient` is what decides. In production it's backed by a live
gateway; while the transport half is pending you supply your own in-process
implementation — the interface is small and the `Decision` values are exactly the
ones a gateway returns, so **the code you write now is unchanged when the transport
lands**:

```go
type GovernanceClient interface {
    Check(ctx context.Context, request CheckRequest) (Decision, error)
    WaitForApproval(ctx context.Context, request ApprovalRequest) (Decision, error)
    RecordResult(ctx context.Context, request RecordRequest) error
    Close() error
}
```

Here is a complete, runnable program: one `echoTool`, an in-process client that
**allows** everything, and one governed call.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/ai-agent-assembly/go-sdk/assembly"
)

// echoTool is a minimal Tool implementation.
type echoTool struct{}

func (echoTool) Name() string        { return "echo" }
func (echoTool) Description() string { return "returns its input unchanged" }
func (echoTool) Call(_ context.Context, input string) (string, error) {
    return input, nil
}

// allowClient is an in-process GovernanceClient that allows every call. Swap it
// for a transport-backed client and the WrapTools call below stays identical.
type allowClient struct{}

func (allowClient) Check(context.Context, assembly.CheckRequest) (assembly.Decision, error) {
    return assembly.Decision{Denied: false, Reason: "allowed"}, nil
}
func (allowClient) WaitForApproval(context.Context, assembly.ApprovalRequest) (assembly.Decision, error) {
    return assembly.Decision{}, nil
}
func (allowClient) RecordResult(context.Context, assembly.RecordRequest) error { return nil }
func (allowClient) Close() error                                              { return nil }

func main() {
    ctx := assembly.WithAgentID(context.Background(), "my-agent")

    governed := assembly.WrapTools([]assembly.Tool{echoTool{}}, allowClient{})

    out, err := governed[0].Call(ctx, "hello, governance")
    if err != nil {
        log.Fatalf("tool call: %v", err)
    }
    fmt.Println("result:", out) // result: hello, governance
}
```

Run it — no gateway, no API key, no `Init`:

```bash
go run .
# result: hello, governance
```

The inner tool ran because the policy check returned **allow**. That's the whole
governed path, minus the "no" answer — which is next.

{{< callout type="note" >}}
**A missing client is not a silent allow.** `assembly.WrapTools(tools, nil)` under
the default fail-closed posture denies every call with `ErrGovernanceUnavailable`
rather than running it unchecked — governance fails safe. Pass
`assembly.WithFailClosed(false)` for a true passthrough wrapper while you wire in a
real client. See [Handle allow/deny decisions and errors]({{< relref "/guides/handle-decisions-and-errors" >}}).
{{< /callout >}}

## Step 4 — See a policy denial

Now make the client say **no**. When `Check` returns `Decision{Denied: true}`, the
wrapper stops the inner tool from running and returns a typed
`*assembly.PolicyViolationError` carrying the tool name and the gateway's reason —
this is governance actually taking effect:

```go
// denyClient blocks any tool named in blockedTools.
type denyClient struct{ allowClient }

var blockedTools = map[string]string{
    "delete-file": "delete operations are blocked by policy",
}

func (denyClient) Check(_ context.Context, req assembly.CheckRequest) (assembly.Decision, error) {
    if reason, blocked := blockedTools[req.ToolName]; blocked {
        return assembly.Decision{Denied: true, Reason: reason}, nil
    }
    return assembly.Decision{Denied: false}, nil
}
```

Match the denial by type with `errors.As` — never by string:

```go
out, err := governed[0].Call(ctx, "config.yaml")
if err != nil {
    var denied *assembly.PolicyViolationError
    if errors.As(err, &denied) {
        log.Printf("blocked %q: %s", denied.ToolName, denied.Reason)
        return // surface a friendly message, pick another tool, etc.
    }
    log.Fatalf("unexpected error: %v", err)
}
use(out)
```

A denied `delete-file` call never runs its inner `Call`; an allowed `read-file`
call runs normally. The runnable version of this two-tool allow/deny is the
[Tool policy example]({{< relref "/examples/tool-policy" >}}).

## Step 5 — Approvals (human-in-the-loop)

Some calls come back **pending** — they need an out-of-band human decision. When
`Check` returns `Decision{Pending: true}`, the wrapper calls `WaitForApproval` and
blocks until a human decides; then it either runs the tool or returns a
`*PolicyViolationError`:

```go
func (c approvalClient) Check(context.Context, assembly.CheckRequest) (assembly.Decision, error) {
    // Route this call to a human instead of deciding inline.
    return assembly.Decision{Pending: true, Reason: "needs reviewer sign-off"}, nil
}

func (c approvalClient) WaitForApproval(ctx context.Context, _ assembly.ApprovalRequest) (assembly.Decision, error) {
    // Block on your approval system (queue, webhook, chat-ops…), then return the
    // resolved decision. A live gateway resolves this from an operator's action
    // in the dashboard's approvals queue.
    return c.reviewer.await(ctx) // e.g. Decision{Denied: false} once approved
}
```

You don't inspect the `Decision` yourself in the common case — the wrapper acts on
it for you. The full pending/approval path, including how a resolved deny surfaces,
is in [Handle allow/deny decisions and errors]({{< relref "/guides/handle-decisions-and-errors#the-decision-the-gateway-returns" >}}).

## Step 6 — Interpret the errors

A governed call has more outcomes than "it worked". Each is a typed value you can
match on with `errors.Is` / `errors.As`:

| Outcome | What you get back |
|---|---|
| **Denied** | `*assembly.PolicyViolationError` (`ToolName`, `Reason`) |
| **Governance unreachable, fail-closed** | `ErrGovernanceUnavailable` (the call is blocked, not run) |
| **Gateway URL unresolved** | `ErrInvalidGateway` (sentinel) |
| **No reachable runtime transport** | `ErrSidecarUnavailable` (sentinel) — the current published-build case |
| **Config couldn't resolve** | `*assembly.ConfigurationError` |

Every error preserves its chain with `%w`, so `errors.Is`/`errors.As` see through
the wrapping. The full symptom → cause → fix tables live in
[Troubleshooting]({{< relref "/troubleshooting" >}}); the failure-posture choice
(fail-closed vs fail-open) is in
[Handle allow/deny decisions and errors]({{< relref "/guides/handle-decisions-and-errors" >}}).

## Step 7 — Observe your agent in the dashboard

Once the transport half is wired (see the status note below), every governed call
your agent makes is recorded and shows up on the **operator dashboard** — the
surface a platform team uses to watch, audit, and control the fleet. Your Go
agent's registration and its allow/deny/approval decisions land here.

The **Fleet** view lists every registered agent, its framework, its enforcement
mode (enforce / shadow), and live status:

![Agent Assembly operator dashboard — Fleet view listing registered agents with their framework, owner, enforcement mode and status, in light theme.](/images/dashboard/fleet-light.png "Fleet view (light theme): registered agents across frameworks, each with owner, enforcement mode, status and last-seen time.")

![Agent Assembly operator dashboard — Fleet view, dark theme, showing the same registered-agent table.](/images/dashboard/fleet-dark.png "Fleet view (dark theme): the same operator surface a governed Go agent appears in once registration is available.")

The **Topology** view lays the same fleet out as a graph, grouping agents into
per-team clusters — each cluster carrying its own budget bar, so you can see which
team an agent belongs to and how its spend rolls up:

![Agent Assembly operator dashboard — Topology view, light theme: agent cards grouped into two team clusters (support, analytics), each labelled with a per-team budget bar and connected by delegation edges; one card outlined red for an errored agent.](/images/dashboard/topology-light.png "Topology (light theme): the fleet as a team-clustered graph, with per-team budget bars.")

![Agent Assembly operator dashboard — Topology view, dark theme: the same team-clustered agent graph.](/images/dashboard/topology-dark.png "Topology (dark theme): agents grouped by team, each cluster showing its budget rollup.")

The **Audit Log** is the immutable governance trail — LLM calls, tool invocations,
file ops, network requests, and policy verdicts across all agents. Here a policy
**deny** is recorded (top row: an outbound `gmail/send` to an external recipient
blocked pending approval), exactly the kind of `PolicyViolationError` your code
handled in Step 4:

![Agent Assembly Audit Log, light theme, showing a governance trail of tool, LLM, file and network events with allow, deny and redact decisions.](/images/dashboard/audit-light.png "Audit Log (light theme): every governed action with its decision — note the deny row for a blocked external send.")

![Agent Assembly Audit Log, dark theme, with a policy-violation deny row highlighted at the top of the trail.](/images/dashboard/audit-dark.png "Audit Log (dark theme): the same trail — the deny verdict a governed tool call produces is recorded here for audit.")

{{< callout type="note" >}}
**These screenshots show the operator surface, populated by a running gateway.**
On the published pure-Go build your agent can't yet register into this view — see
[Current status & limitations](#current-status--limitations). The dashboard itself
is operated from the [docs hub — operator path](https://docs.agent-assembly.com/).
{{< /callout >}}

## Step 8 — Tune a policy

The point of governance is that behaviour changes when the **policy** changes — a
call that was allowed becomes denied (or the reverse) with no code change on your
side. In the in-process demo you edit your `Check` logic; against a live gateway an
operator authors the policy and your agent's next `Check` reflects it.

The SDK-side settings that interact with policy — the enforcement mode and
fail-closed vs fail-open posture — are in
[Configuration]({{< relref "/configuration" >}}); policy authoring and the policy
reference live on the [docs hub](https://docs.agent-assembly.com/).

To run a governed Go agent in a container (with the gateway/runtime alongside it),
see [Use the governed container base image]({{< relref "/guides/container-base-image" >}}).

## Step 9 — Govern a real agent (framework tabs)

Pick your framework — each tab shows the governance slice copied verbatim from a
runnable example in the
[examples repo](https://github.com/ai-agent-assembly/examples/tree/HEAD/go)
(a CI drift check keeps them in lockstep). Go's per-framework surface is thin
today, so the tabs are **LangChainGo** (the framework path) and **Plain** (the
framework-agnostic path). A new tab appears automatically once a new Go
**framework** example lands.

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

Two more validated Go examples exist — **Tool Policy** (an allow/deny policy demo)
and **CLI Runtime** (sidecar wiring) — kept out of the tabs as patterns rather than
"first agent" frameworks; see `metadata/quickstart/README.md` for the rationale.

## You've experienced the core value

You took a plain Go tool, wrapped it once, and saw a call **allowed**, a call
**denied** with a typed `PolicyViolationError`, and a call **held for approval** —
all without touching the tool's own logic, and all runnable today. That is the
whole point: **governance as a checkpoint in front of tool calls, not a rewrite of
your agent.**

## Current status & limitations

Honesty matters more than a tidy demo, so here is exactly what does and doesn't
work today on the published module.

**Works now, from a plain `go get` (pure-Go, `CGO_ENABLED=0`):**

- Installing and building the SDK.
- The full interception model: `WrapTools`, the `Tool` and `GovernanceClient`
  interfaces, the `Decision` model, allow / deny / pending / approval handling, the
  typed errors, the fail-closed vs fail-open posture, and context propagation
  (`WithAgentID`, `WithTraceID`, `WithRunID`). Steps 1–6 above are all runnable.

**Not reachable from published artifacts yet:**

- **Connecting to a live gateway.** `assembly.Init` returns
  `ErrSidecarUnavailable` on the pure-Go build regardless of options: the default
  build has **no native transport**, so there is no runtime to connect to, and the
  in-process fallback fails closed rather than allowing traffic unchecked.
- Consequently: **agent registration** (appearing in the dashboard's Fleet view)
  and a real gateway-driven allow/deny/approval over the network.

**Why:** the production transport is an **opt-in native cgo binding**
(`-tags aa_ffi_go`, `CGO_ENABLED=1`) that links `libaa_ffi_go`. That native library
is **not published** for a plain `go get` — building with `-tags aa_ffi_go` outside
a full monorepo checkout fails with `ld: library 'aa_ffi_go' not found`. Publishing
the native library, or dropping the cgo requirement, is a separate product
decision.

**Track status:**
[AAASM-4547](https://lightning-dust-mite.atlassian.net/browse/AAASM-4547) and
[AAASM-4469](https://lightning-dust-mite.atlassian.net/browse/AAASM-4469).

## Where to next

- [Core Concepts]({{< relref "/core-concepts" >}}) — what's actually happening inside the SDK.
- **[Examples]({{< relref "/examples" >}})** — runnable end-to-end code: [basic agent]({{< relref "/examples/basic-agent" >}}), [tool policy]({{< relref "/examples/tool-policy" >}}), LangChainGo, and CLI runtime.
- [Guides]({{< relref "/guides" >}}) — wrap a real agent, [integrate a framework]({{< relref "/guides/framework-integration" >}}), [handle decisions and errors]({{< relref "/guides/handle-decisions-and-errors" >}}), and the [container base image]({{< relref "/guides/container-base-image" >}}).
- [Configuration]({{< relref "/configuration" >}}) — every `Init` option, defaults, and enforcement modes.
- [Troubleshooting]({{< relref "/troubleshooting" >}}) — what to do when `Init` or a check fails.
</content>
</invoke>
