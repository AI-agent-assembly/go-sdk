---
title: Compatibility & Versioning
weight: 6
---

# Compatibility & Versioning

`go-sdk` is a **client** of the AI Agent Assembly gateway (the core runtime in
[ai-agent-assembly/agent-assembly](https://github.com/ai-agent-assembly/agent-assembly)).
It is built against a specific gateway **protocol version** and a minimum Go
toolchain, and it ships on its own release cadence. This page covers both the
compatibility contract and how releases are cut.

## Protocol version

The [`VERSION`](https://github.com/ai-agent-assembly/go-sdk/blob/main/VERSION)
file pins the gateway protocol version this SDK speaks (currently `0.0.1-rc.6`). It
is the wire-compatibility contract, separate from the module's release tags:

- A released tag such as `v0.0.1-alpha.3` is the **SDK version** you `go get`.
- `VERSION` is the **gateway protocol** the SDK build targets.

When the gateway proto changes, `VERSION` is bumped, the vendored conformance
vectors are updated, and `make test` confirms wire compatibility before the
change ships.

> **Cross-SDK matrix.** The core↔SDK compatibility matrix — which SDK versions
> speak which gateway protocol, across the Go, Python, and Node SDKs — is
> published on the shared
> [documentation hub](https://docs.agent-assembly.com/).
> Consult it when pairing an SDK release with a gateway deployment.

## Wire contract

- **Transports** — the SDK talks to the gateway over **gRPC** and **HTTP**.
- **Enforcement modes** — `EnforcementMode` mirrors `aa_core::EnforcementMode`;
  the `enforce` / `observe` / `disabled` tokens are sent verbatim on the wire.
  See [Configuration]({{< relref "/configuration" >}}) for the per-agent posture.
- **Identity propagation** — agent / trace / run IDs are stamped onto every
  `Check` and `RecordResult`, and reach the gateway on the `Check` (the shipped
  governance client discards the record, AAASM-5731); the trace ID falls back to
  the active OpenTelemetry span context when not set explicitly.

## Framework compatibility

`go-sdk` governs **AI-agent frameworks at the tool boundary**. The Go ecosystem
coverage is deliberately narrow:

| Framework | Module | Tested version line | Support |
| --- | --- | --- | --- |
| **LangChainGo** | `github.com/tmc/langchaingo` | **`v0.1.x`** (CI + examples pin `v0.1.14`) | First-class adapter (`WrapChain`) + tool-wrapping |
| *Any other tool* | — | — | Generic `WrapTools` over arbitrary `tools.Tool` values |

**LangChainGo is the only first-class framework adapter.** Everything else is
covered by the framework-agnostic `WrapTools` surface — there is no
per-framework adapter for CrewAI/Genkit/Eino-style stacks today.

### No framework dependency by design

The SDK takes **no framework dependency**: `go.mod` imports neither
`langchaingo` nor any other agent framework. Governance is structural, not
import-coupled:

- `assembly.Tool` is `Name() / Description() / Call(ctx, input)` — the exact
  shape of LangChainGo's `tools.Tool`. Any `langchaingo/tools.Tool` therefore
  satisfies `assembly.Tool` with **no adapter**, so `assembly.WrapTools` can
  govern an arbitrary slice of LangChainGo tools (and any other value that
  implements the same three methods).
- `assembly.WrapChain` adapts LangChainGo's `chains.Chain` shape
  (`Call(ctx, map[string]any) (map[string]any, error)`) the same way, again
  without importing the framework — it only propagates the assembly's agent ID
  to child agents.

Because the contract is satisfied structurally, you pin LangChainGo (or not) in
**your** `go.mod`; this SDK never pulls it in. The `v0.1.14` pin above is the
version exercised by the AAASM-3525 Go smoke driver and the
`examples/go/langchaingo` sample, not a hard floor.

### Cross-SDK index on the core docs

The **Framework compatibility** section of *this* page is the **authoritative**
source for Go — the LangChainGo adapter and `assembly.WrapTools` live in this
SDK, so its compatibility is documented here. The core docs site hosts a
**cross-SDK index/hub** that links to this page and to the Python/Node
equivalents:

> **<https://docs.agent-assembly.com/core/stable/reference/framework-compatibility.html>**

That index is the `/stable/` channel link; it **404s until GA** by design (the
`stable` channel activates at the first `vX.Y.0` release), consistent with the
core-side convention. Use the
[documentation hub](https://docs.agent-assembly.com/)
to reach the cross-SDK index in the meantime.

## Toolchain

| Requirement | Value |
| --- | --- |
| Minimum Go | **1.26** (the `go` directive in `go.mod`) |
| CGo | Not required — the pure-Go default builds with `CGO_ENABLED=0`. The native FFI transport is opt-in via `-tags aa_ffi_go`. |

## Release state

The SDK is **pre-release** on the `v0.0.1-alpha` line. The public `assembly`
package API may change between alpha tags; pin an exact tag and review the
[release notes](https://github.com/ai-agent-assembly/go-sdk/releases) before
upgrading.

## Release process

`go-sdk` follows the standard Go module release model: **a release is a git
tag**. There is no separate publish step to `go get` a tagged version — the Go
module proxy and [pkg.go.dev](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk)
index the tag automatically.

### Versioning

- **Semantic Versioning** (`vMAJOR.MINOR.PATCH`), with pre-release suffixes for
  the current alpha line (`v0.0.1-alpha.N`).
- The module path is stable — `github.com/ai-agent-assembly/go-sdk` — so a major
  bump past `v1` would add a `/vN` suffix per Go's import-compatibility rule.
- `VERSION` is **not** the release version; it pins the gateway protocol the SDK
  targets (see [Protocol version](#protocol-version) above).

### Cutting a release

1. Land all changes on `main` with CI green.
2. If the gateway protocol changed, bump
   [`VERSION`](https://github.com/ai-agent-assembly/go-sdk/blob/main/VERSION)
   and update the conformance vectors; confirm with `make test`.
3. Tag the commit and push the tag:

   ```bash
   git tag v0.0.1-alpha.4
   git push remote v0.0.1-alpha.4
   ```

4. Publish the **release notes** on the
   [GitHub Releases](https://github.com/ai-agent-assembly/go-sdk/releases) page
   for that tag.

### What happens after the tag

- `go get github.com/ai-agent-assembly/go-sdk@v0.0.1-alpha.4` resolves through
  the module proxy immediately.
- pkg.go.dev renders the godoc for the tag within minutes.
- The `goreleaser` config (`.goreleaser.yaml`) is validated on every push to
  `main` by the `goreleaser-check` job; it is set up for source archives, with
  binary builds skipped (this is a library, not a CLI).

### Consuming a pinned version

```go
require github.com/ai-agent-assembly/go-sdk v0.0.1-alpha.3
```

Pin an exact tag while the SDK is pre-release; review the release notes before
moving to a newer alpha.
