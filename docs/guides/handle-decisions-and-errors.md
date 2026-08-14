---
title: Handle allow/deny decisions and errors
weight: 3
---

# Handle allow/deny decisions and errors

A governed tool call has more outcomes than "it worked". The gateway may **deny**
it, **hold it for approval**, or be **unreachable** — and the SDK signals each
with a typed value you can match on. This guide covers what each outcome looks
like and how to react.

## The decision the gateway returns

Before a wrapped tool runs, the SDK calls `Check` and gets back a `Decision`:

```go
type Decision struct {
    Denied  bool   // policy rejected the call
    Pending bool   // call needs out-of-band (human) approval
    Reason  string // human-readable explanation
}
```

You normally don't inspect this yourself — the wrapper acts on it for you:

- **Allowed** (`Denied == false`, `Pending == false`) — the inner tool runs, its
  result is returned, and a `RecordResult` is offered to the governance client
  afterward — carried onward only if that client can, which the shipped one does
  over a connected runtime (AAASM-5750).
- **Denied** (`Denied == true`) — the inner tool does **not** run; `Call` returns
  a `*PolicyViolationError`.
- **Pending** (`Pending == true`) — the wrapper calls `WaitForApproval` and
  blocks until a human decides. If the resolved decision is a deny, you get a
  `*PolicyViolationError`; otherwise the tool runs.

## Reacting to a denial

`*PolicyViolationError` carries the tool name and the gateway's reason. Match it
with `errors.As`:

```go
out, err := governed[0].Call(ctx, input)
if err != nil {
    var denied *assembly.PolicyViolationError
    if errors.As(err, &denied) {
        log.Printf("blocked %q: %s", denied.ToolName, denied.Reason)
        // surface a friendly message to the user, pick a different tool, etc.
        return
    }
    // some other error — see below
    return
}
use(out)
```

## Choosing a failure posture

When the SDK can't reach the gateway to *get* a decision, `WithFailClosed`
decides what happens:

```go
a, err := assembly.Init(ctx,
    assembly.WithFailClosed(true), // gateway unreachable => block the call
)
```

| Setting | On a gateway/check failure |
|---|---|
| `WithFailClosed(true)` *(default)* | The call is **blocked** — `Call` returns the wrapped check error (fail-safe). |
| `WithFailClosed(false)` | The call **proceeds** to the inner tool (fail-open). |

Pick fail-closed when an ungoverned action is unacceptable; pick fail-open when
availability matters more than strict enforcement. The two are independent of the
gateway's [enforcement mode]({{< relref "/core-concepts#modes-and-enforcement" >}}).

## Initialisation errors

`Init` returns typed errors, not strings — match the sentinels with `errors.Is`
and the structured types with `errors.As`:

```go
a, err := assembly.Init(ctx, opts...)
switch {
case errors.Is(err, assembly.ErrInvalidGateway):
    // no gateway URL from any source, and the local auto-start failed
case err != nil:
    var cfgErr *assembly.ConfigurationError
    var gwErr *assembly.GatewayError
    switch {
    case errors.As(err, &cfgErr):
        // e.g. local default needed but `aasm` is not on PATH
    case errors.As(err, &gwErr):
        // gateway URL known but unreachable / didn't become ready
    }
}
```

| Error | Meaning |
|---|---|
| `ErrInvalidGateway` (sentinel) | No gateway URL resolved from option, env, config, or local default. |
| `ErrRuntimeNotInitialized` (sentinel) | The runtime was used before a successful `Init`, or after `Close`. |
| `*ConfigurationError` | The SDK couldn't resolve a gateway — e.g. local default needed but `aasm` is missing from `PATH`. |
| `*GatewayError` | A gateway URL is known but the SDK can't talk to it (auto-start didn't become ready in time). |

> An **empty API key is not an error**. Local mode accepts unauthenticated
> agents, so `Init` never fails just because no API key was set — supply one only
> when your gateway requires authentication.

## A note on wrapping

Every error the SDK returns preserves its chain with `%w`, so `errors.Is` and
`errors.As` see through the wrapping. Always match on the typed value rather than
comparing error strings.

## Next

- [Troubleshooting]({{< relref "/troubleshooting" >}}) — symptom → cause → fix tables for the
  errors above.
- [Configuration]({{< relref "/configuration" >}}) — timeouts, enforcement modes, and the
  resolution chain.
