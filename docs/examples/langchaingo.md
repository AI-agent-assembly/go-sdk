---
title: LangChainGo
weight: 4
---

# LangChainGo

Govern a [LangChainGo](https://github.com/tmc/langchaingo) agent's tool calls
with Agent Assembly. The headline point: a wrapped tool is **still a valid
LangChainGo tool**, so adding governance is drop-in — no adapter required.

## What this example demonstrates

- Defining tools that implement LangChainGo's `tools.Tool` interface.
- Wrapping those tools with `assembly.WrapTools` so each tool call is checked
  against a policy before it runs.
- That a governed tool still satisfies `tools.Tool` — the interfaces share the
  same shape (`Name`, `Description`, `Call`).
- An **allowed** call (`search`) completing and a **denied** call (`send-email`)
  returning `assembly.PolicyViolationError`.
- Driving the agent with a **fake LLM**, so the example runs with no API key.

## The framework / library

[**LangChainGo**](https://github.com/tmc/langchaingo) (`v0.1.14`) — the Go port
of LangChain. The example uses its `tools.Tool` interface
([godoc](https://pkg.go.dev/github.com/tmc/langchaingo/tools#Tool)) and its
`llms/fake` fake LLM so it stays offline.

## How it works

The key insight is structural compatibility: LangChainGo's `tools.Tool` and
`assembly.Tool` have the same method set, so a single tool value satisfies both.

1. `tools.go` defines `searchTool` (safe) and `sendEmailTool` (side-effecting),
   both implementing `tools.Tool`. Compile-time assertions
   (`var _ tools.Tool = (*searchTool)(nil)`) prove it.
2. `policy.go` defines `policyClient` — an `assembly.GovernanceClient` that
   denies `send-email` and allows everything else.
3. `main.go` uses `fake.NewFakeLLM` to produce the agent's plan, then wraps both
   tools with `assembly.WrapTools` using `policyClient`. Because the wrapped
   tools still satisfy `tools.Tool`, they can be handed straight to a LangChainGo
   agent/executor.
4. Each tool is called: `search` is allowed and returns a result; `send-email`
   is denied and surfaces an `assembly.PolicyViolationError`.

## Prerequisites & running it

Complete [Preparing the runtime environment]({{< relref "/examples/setup" >}})
first. This example pins **Go ≥ 1.26**, **Go SDK v0.0.1-alpha.4**, and
**LangChainGo v0.1.14** in its `go.mod`. Then:

```bash
cd examples/go/langchaingo
go mod download
go run .
```

No live gateway and no LLM API key are required — a mock client applies the
policy and `fake.NewFakeLLM` stands in for the model.

## Code walkthrough

The tools implement LangChainGo's interface and prove it at compile time:

```go
type searchTool struct{}

func (s *searchTool) Name() string        { return "search" }
func (s *searchTool) Description() string { return "Looks up a topic and returns a short summary." }
func (s *searchTool) Call(_ context.Context, query string) (string, error) {
	return "(summary for " + query + ")", nil
}

// Compile-time proof that both tools satisfy langchaingo's tools.Tool.
var (
	_ tools.Tool = (*searchTool)(nil)
	_ tools.Tool = (*sendEmailTool)(nil)
)
```

`main` runs the fake LLM to produce a plan, then wraps the tools — the wrapped
values stay valid LangChainGo tools:

```go
// A fake LLM keeps the example offline. In production this is openai.New(), etc.
model := fake.NewFakeLLM([]string{
	"I should search for the topic, then email the result.",
})
plan, err := llms.GenerateFromSinglePrompt(ctx, model, "How do I summarize a topic and notify the user?")
if err != nil {
	log.Fatalf("[llm] generation failed: %v", err)
}
fmt.Printf("[llm] plan: %s\n\n", plan)

// Wrapped tools still satisfy tools.Tool, so a LangChainGo agent can use them.
governed := assembly.WrapTools(
	[]assembly.Tool{&searchTool{}, &sendEmailTool{}},
	&policyClient{},
)

runTool(ctx, governed[0], "agent governance")   // allowed
runTool(ctx, governed[1], "user@example.com")   // denied
```

## Notes & caveats

> **The wrapping is the integration.** Because `tools.Tool` and `assembly.Tool`
> are structurally identical, there is no adapter layer — the same value is both
> a LangChainGo tool and a governed Agent Assembly tool.

> **This example doesn't run a full LangChainGo agent executor.** It drives a
> fake LLM to show the reasoning step, then calls the governed tools directly so
> the allow/deny outcome is unambiguous. To make it production-ready, swap
> `fake.NewFakeLLM` for a real model (e.g. `openai.New()`) and `policyClient` for
> a gateway-backed client.

## Expected behavior

```text
[assembly] governing LangChainGo tools via an offline policy client
[policy] loaded: search=ALLOW, send-email=DENY

[llm] plan: I should search for the topic, then email the result.

[agent] calling tool: search  input="agent governance"
[policy] ALLOWED  tool=search
[agent] result: (summary for agent governance)

[agent] calling tool: send-email  input="user@example.com"
[policy] DENIED   tool=send-email  reason="outbound email is blocked by policy"
[agent] blocked: assembly: policy violation: tool=send-email reason=outbound email is blocked by policy
```

`go test ./...` runs the same paths offline — no gateway, no API key.

## Links

- Example directory: [`go/langchaingo`](https://github.com/ai-agent-assembly/examples/tree/HEAD/go/langchaingo)
- [`README.md`](https://github.com/ai-agent-assembly/examples/blob/HEAD/go/langchaingo/README.md)
- [LangChainGo](https://github.com/tmc/langchaingo) · [`tools.Tool`](https://pkg.go.dev/github.com/tmc/langchaingo/tools#Tool)
- Related: [Integrate with a framework]({{< relref "/guides/framework-integration" >}})
