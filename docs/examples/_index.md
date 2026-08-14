---
title: Examples
weight: 8
---

# Examples

Complete, runnable Go examples live in the
[examples](https://github.com/ai-agent-assembly/examples/tree/HEAD/go)
repository. Each is a self-contained Go module with its own `README.md`. This
section walks through every Go example in depth: what it demonstrates, how the
governance flow works, the real commands to run it, and an annotated tour of the
code.

Start with [Preparing the runtime environment]({{< relref "/examples/setup" >}})
for the one-time setup every example shares, then pick the example that matches
what you're building.

## How the SDK wraps tool calls

Every example follows the same shape as the [Quick Start]({{< relref "/quick-start" >}}):
tag the context with an agent ID, then wrap your tools with a
`GovernanceClient` so each call is checked against a policy *before* it runs and
its outcome offered to `RecordResult` *after* it finishes. The client this SDK
ships forwards that record to a connected runtime — see the note on the
[documentation home]({{< relref "/" >}}) (AAASM-5750).

```go
import (
    "context"
    "fmt"

    "github.com/ai-agent-assembly/go-sdk/assembly"
)

func main() {
    ctx := assembly.WithAgentID(context.Background(), "my-agent")

    // The examples in this section use an in-process mock GovernanceClient so
    // they run offline. In production you pass a transport-backed client that
    // reaches your gateway.
    client := &mockClient{}

    governed := assembly.WrapTools([]assembly.Tool{myTool}, client)
    // hand `governed` to your agent — every Call is now governed
    result, err := governed[0].Call(ctx, "input")
    fmt.Println(result, err)
}
```

> **These examples run offline.** To keep them runnable in CI without a live
> gateway, every example here passes an in-process mock `GovernanceClient` to
> `WrapTools`. The wrapping logic is identical to production — swap the mock for
> a transport-backed client and the same code governs real tool access. See
> [Configuration]({{< relref "/configuration" >}}) for connecting to a real
> gateway.

## In this section

| Page | What it covers |
|---|---|
| [Preparing the runtime environment]({{< relref "/examples/setup" >}}) | Shared prerequisites: install the SDK, clone the examples repo, and the commands every example uses. |
| [Basic agent]({{< relref "/examples/basic-agent" >}}) | A minimal governed agent — wrap one tool and run a single governed call. |
| [Tool policy]({{< relref "/examples/tool-policy" >}}) | Per-tool allow/deny with no framework — one tool completes, one is denied with `PolicyViolationError`. |
| [LangChainGo]({{< relref "/examples/langchaingo" >}}) | Govern a [LangChainGo](https://github.com/tmc/langchaingo) agent's tool calls — wrapped tools stay valid LangChainGo tools. |
| [CLI runtime integration]({{< relref "/examples/cli-runtime-integration" >}}) | Auto-start the `aasm` sidecar from a Go workflow and fall back gracefully when it's absent. |

Each example's `README.md` in the
[examples repo](https://github.com/ai-agent-assembly/examples/tree/HEAD/go)
has the canonical step-by-step instructions; these pages add the walkthrough.

## Where to next

- [Quick Start]({{< relref "/quick-start" >}}) — the three-step path these examples build on.
- [Guides]({{< relref "/guides" >}}) — task-first walkthroughs of wrapping tools, framework integration, and handling decisions.
- [Configuration]({{< relref "/configuration" >}}) — every `Init` option, defaults, and enforcement modes.
