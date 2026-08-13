---
title: Configuration
weight: 4
---

# Configuration

`assembly.Init` takes a `context.Context` and a variadic list of functional
options. New configuration is always added as a `WithXxx(value)` option, so
call sites never break when an option is introduced.

```go
a, err := assembly.Init(ctx,
    assembly.WithGatewayURL("https://gateway.example.com"),
    assembly.WithAPIKey("..."),
    assembly.WithFailClosed(true),
    assembly.WithTimeout(750*time.Millisecond),
    assembly.WithEnforcementMode(assembly.EnforcementModeObserve),
)
```

## Gateway and credential resolution

The gateway URL and API key are **not** ordinary options — `Init` resolves each
one through a fixed precedence chain, so you can pass them explicitly in
production and omit them entirely for local development.

The gateway URL is resolved from, highest priority first:

1. `WithGatewayURL("…")` — the explicit option.
2. The `AA_GATEWAY_URL` environment variable.
3. The `agent.gateway_url` key in `~/.aasm/config.yaml`.
4. The local default `http://localhost:7391` — `Init` probes it and, if no
   gateway answers, auto-starts a local one (`aasm start --mode local
   --foreground`).

The `aasm` CLI and the gateway it manages are documented in the core
[agent-assembly docs](https://docs.agent-assembly.com/core/) —
see there for running a gateway, authoring policy, and the full `aasm`
command set.

If every source yields an empty URL, `Init` returns `ErrInvalidGateway`.

The API key follows the same chain — `WithAPIKey` → `AA_API_KEY` →
`agent.api_key` in the config file — but an **empty API key is allowed**:
local mode accepts unauthenticated calls, so no error is raised when the key
is unset. `WithAPIKey` is therefore **optional**; supply it only when your
gateway requires authentication.

> **Note:** `AAASM_GATEWAY_URL` / `AAASM_API_KEY` are accepted as deprecated
> aliases for backward compatibility and emit a one-time deprecation warning
> at runtime; use the canonical `AA_*` names in new configurations. (See
> `assembly/gateway_resolver.go`.)

```yaml
# ~/.aasm/config.yaml
agent:
  gateway_url: https://gateway.example.com
  api_key: your-operator-issued-key
```

## Optional options

| Option | Type | Default | Purpose |
| --- | --- | --- | --- |
| `WithFailClosed` | `bool` | `false` | When `true`, a gateway failure **blocks** the action (fail-closed). When `false`, the action is allowed if the gateway is unreachable (fail-open). |
| `WithTimeout` | `time.Duration` | `500ms` | Gateway check timeout applied when the call `ctx` carries no deadline. |
| `WithEnforcementMode` | `EnforcementMode` | `enforce` | Per-agent governance posture sent to the gateway at registration. |
| `WithSelfAgentID` | `string` | _(unset)_ | Records this agent's own ID for lineage tracking. |
| `WithParentAgentID` | `string` | _(unset)_ | Parent agent ID for topology tracking. |
| `WithTeamID` | `string` | _(unset)_ | Team ID for budget and policy scoping. |
| `WithDelegationReason` | `string` | _(unset)_ | Human-readable reason this agent was delegated work. |
| `WithSpawnedByTool` | `string` | _(unset)_ | Name of the tool that spawned this agent. |
| `WithSidecarBinary` | `string` | _(unset)_ | Path to a sidecar binary for managed-lifecycle (sidecar) mode. |

## Enforcement modes

`WithEnforcementMode` accepts the values mirrored from `aa_core::EnforcementMode`
on the wire:

| Constant | Token | Behavior |
| --- | --- | --- |
| `EnforcementModeEnforce` | `enforce` | Default. A `deny` blocks the action; `redact` strips secrets. |
| `EnforcementModeObserve` | `observe` | Dry-run. The gateway records what *would* have happened but does not block. |
| `EnforcementModeDisabled` | `disabled` | Policy evaluation is skipped entirely. |

## Per-call identity (context helpers)

Identity that varies per request is carried on `context.Context`, not on
`Init`. The SDK stamps these onto every `Check` and `RecordResult`; the `Check`
is what carries them to the gateway, since the shipped client discards the record
(AAASM-5731):

| Helper | Reader | Notes |
| --- | --- | --- |
| `WithAgentID` | `AgentIDFromContext` | The calling agent's identity. |
| `WithTraceID` | `TraceIDFromContext` | Falls back to the OpenTelemetry span-context trace ID when unset. |
| `WithRunID` | `RunIDFromContext` | Run identity for a single agent run. |
| `EnsureRunID` | — | Guarantees a stable run ID within the same context tree. |

```go
ctx = assembly.WithAgentID(ctx, "my-agent")
ctx = assembly.EnsureRunID(ctx)
```

## Where to next

- [Quick Start]({{< relref "/quick-start" >}}) — install, init, wrap your tools.
- [Troubleshooting]({{< relref "/troubleshooting" >}}) — what each configuration error means.
- [Core Concepts]({{< relref "/core-concepts" >}}) — how options flow into the runtime.
