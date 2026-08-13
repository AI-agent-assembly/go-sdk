---
title: Basic agent
weight: 2
---

# Basic agent

The smallest possible governed agent: define one tool, wrap it for governance,
and run a single governed call. Start here to see the SDK's interception shape
with nothing else in the way.

## What this example demonstrates

- Importing and using the Agent Assembly Go SDK.
- Defining a tool that satisfies the `assembly.Tool` interface (`Name`,
  `Description`, `Call`).
- Wrapping a tool with `assembly.WrapTools` so each call is intercepted.
- Observing the **allow** decision path through console output.
- Using an offline mock `GovernanceClient` so the example runs with no live
  gateway.

## The framework / library

**No framework.** This example uses only the Go SDK
([`github.com/ai-agent-assembly/go-sdk`](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk))
and the standard library. It's the bare-metal view of how `WrapTools`
intercepts a call.

## How it works

The governance flow is the same one the [Quick Start]({{< relref "/quick-start" >}})
describes, reduced to its essentials:

1. An `echoTool` implements `assembly.Tool` — `Name`, `Description`, and a
   `Call` that returns its input unchanged.
2. `assembly.WrapTools` wraps the tool with a `GovernanceClient`.
3. Before each `Call`, the wrapper sends a `CheckRequest` to the client and gets
   back a `Decision`.
4. The mock client always returns `Decision{Denied: false}`, so the call is
   **allowed** and proceeds to the inner tool. A denied decision would instead
   surface as an `assembly.PolicyViolationError` (see
   [Tool policy]({{< relref "/examples/tool-policy" >}}) for that path).

## Prerequisites & running it

Complete the one-time [Preparing the runtime environment]({{< relref "/examples/setup" >}})
steps first. Then:

```bash
cd examples/go/basic-agent
go mod download
go run .
```

No gateway and no API key are needed — the example uses the offline mock client.

## Code walkthrough

The tool is a plain struct satisfying `assembly.Tool`:

```go
type echoTool struct{}

func (e *echoTool) Name() string        { return "echo" }
func (e *echoTool) Description() string { return "Returns its input string unchanged." }
func (e *echoTool) Call(_ context.Context, input string) (string, error) {
	return input, nil
}
```

`main` tags the context with an agent ID, builds the mock client, wraps the
tool, then calls it:

```go
func main() {
	// Tag the context so governance checks carry this agent's ID.
	ctx := assembly.WithAgentID(context.Background(), "basic-agent-demo")

	// Use the offline mock governance client.
	fmt.Println("[assembly] using offline mock governance client")
	client := &mockClient{}

	// Wrap the tool — every Call now goes through the governance client first.
	tools := assembly.WrapTools([]assembly.Tool{&echoTool{}}, client)

	input := "Hello, Agent Assembly!"
	result, err := tools[0].Call(ctx, input)
	if err != nil {
		log.Fatalf("[assembly] tool call failed: %v", err)
	}
	fmt.Printf("[assembly] tool result: %s\n", result)
}
```

The mock client in `policy.go` implements `GovernanceClient` and allows
everything:

```go
type mockClient struct{}

func (m *mockClient) Check(_ context.Context, req assembly.CheckRequest) (assembly.Decision, error) {
	return assembly.Decision{Denied: false, Reason: "allowed by offline mock"}, nil
}
// WaitForApproval, RecordResult, and Close round out the interface.
```

## Notes & caveats

> **No `Init` here.** This example never calls `assembly.Init` — it constructs
> the `GovernanceClient` directly. That's why no gateway is required. The
> trade-off is that nothing is enforced beyond the mock's always-allow rule.

> **To use a real gateway,** replace `mockClient` in `policy.go` with a
> transport-backed `GovernanceClient`. The `WrapTools` call and your tool code
> stay exactly the same.

## Expected behavior

```text
[assembly] using offline mock governance client
[assembly] governance: ALLOWED  tool=echo input="Hello, Agent Assembly!"
[assembly] tool result: Hello, Agent Assembly!
```

Running `go test ./...` exercises the same path offline.

## Links

- Example directory: [`go/basic-agent`](https://github.com/ai-agent-assembly/examples/tree/HEAD/go/basic-agent)
- [`README.md`](https://github.com/ai-agent-assembly/examples/blob/HEAD/go/basic-agent/README.md)
- Next: [Tool policy]({{< relref "/examples/tool-policy" >}}) — see a tool get **denied**.
