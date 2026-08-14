# go-sdk

[![Go Reference](https://pkg.go.dev/badge/github.com/ai-agent-assembly/go-sdk.svg)](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk)
[![GitHub release](https://img.shields.io/github/v/tag/ai-agent-assembly/go-sdk?sort=semver&label=release&logo=github)](https://github.com/ai-agent-assembly/go-sdk/releases)
[![Tests](https://img.shields.io/github/actions/workflow/status/ai-agent-assembly/go-sdk/go-test.yml?branch=main&logo=githubactions&label=tests)](https://github.com/ai-agent-assembly/go-sdk/actions/workflows/go-test.yml)
[![Lint](https://img.shields.io/github/actions/workflow/status/ai-agent-assembly/go-sdk/lint.yml?branch=main&logo=go&label=lint)](https://github.com/ai-agent-assembly/go-sdk/actions/workflows/lint.yml)
[![Coverage](https://img.shields.io/codecov/c/github/ai-agent-assembly/go-sdk?logo=codecov)](https://codecov.io/gh/ai-agent-assembly/go-sdk)
[![Go Report Card](https://goreportcard.com/badge/github.com/ai-agent-assembly/go-sdk)](https://goreportcard.com/report/github.com/ai-agent-assembly/go-sdk)
[![Docs](https://img.shields.io/github/actions/workflow/status/ai-agent-assembly/go-sdk/docs-site.yml?branch=main&logo=readthedocs&label=docs)](https://docs.agent-assembly.com/go-sdk/)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache--2.0-blue?logo=apache)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/go-%E2%89%A51.26-00ADD8?logo=go)](https://go.dev/doc/devel/release)

Go SDK for [AI Agent Assembly](https://github.com/ai-agent-assembly) — runtime governance for AI agent tool calls.

The SDK initialises in a few lines, propagates agent identity through `context.Context`, wraps your agent's tool slice with policy enforcement, and forwards every policy check to the AAASM gateway over gRPC or HTTP.

> **The result record is handed to the runtime, and that is all this SDK can tell you.** The wrapper offers the outcome of every governed call — allowed or denied — to the `GovernanceClient`'s `RecordResult`, and the client this SDK ships writes it to the native event channel, the same one the boot registration event uses. That is a handoff, **not an audit guarantee**: the send is unacknowledged, the dispatch is not joined before shutdown (a `defer a.Close()` after a call loses the record deterministically), and the native binding is only compiled under `-tags aa_ffi_go` — so on a default build there is no channel at all and no audit evidence is produced. Enforcement is unaffected either way: a policy DENY still blocks the tool. `Init` warns when no record can be sent, and `Assembly.AuditSink()` reports which case you are in ([AAASM-5750](https://lightning-dust-mite.atlassian.net/browse/AAASM-5750)). Downstream of the handoff, [AAASM-5783](https://lightning-dust-mite.atlassian.net/browse/AAASM-5783) is open on `report_event` payloads reaching neither the live stream nor the durable entry — until it lands, no SDK can claim ADR 0033 §6 *Observed*.

## Project status

`go-sdk` is **pre-release** — see the [releases page](https://github.com/ai-agent-assembly/go-sdk/releases) (or the **release** badge above) for the current published tag. The [`VERSION`](VERSION) file pins the gateway **protocol version** the SDK is built against (currently <!-- BEGIN GENERATED: protocol-version -->`0.0.1-rc.6`<!-- END GENERATED: protocol-version -->). The public `assembly` package API may still change between pre-release tags — pin an exact tag in your `go.mod` and review the [release notes](https://github.com/ai-agent-assembly/go-sdk/releases) before upgrading. See [Core-runtime compatibility](docs/compatibility.md) for the version/protocol contract.

Anything outside the `assembly/` package (`internal/`, `examples/`) is not part of the public API and may change without notice.

## Framework compatibility

The first-class agent-framework adapter is **[LangChainGo](https://github.com/tmc/langchaingo)** (`github.com/tmc/langchaingo`), tested on the **`v0.1.x`** line (CI and the [`examples/go/langchaingo`](https://github.com/ai-agent-assembly/examples/tree/HEAD/go/langchaingo) sample pin `v0.1.14`). `assembly.WrapChain` governs a LangChainGo `chains.Chain`; `assembly.WrapTools` governs any tool.

The SDK requires **no framework by default** — `go.mod` imports neither `langchaingo` nor any other framework. `assembly.Tool` is structurally identical to LangChainGo's `tools.Tool` (`Name`/`Description`/`Call`), so `WrapTools` governs an arbitrary slice of `langchaingo/tools.Tool` values (or any other type with the same three methods) with no adapter. Go-side coverage is LangChainGo plus this generic tool-wrapping — there are no other per-framework adapters.

Go framework compatibility is documented authoritatively in [docs/compatibility.md](docs/compatibility.md#framework-compatibility) — the LangChainGo adapter and `WrapTools` live in this SDK. The core docs provide a cross-SDK **index/hub** that links to this page and the Python/Node equivalents at **<https://docs.agent-assembly.com/core/stable/reference/framework-compatibility.html>** (a `/stable/` link that 404s until GA by design).

## Prerequisites

- **Go ≥ 1.26** — the floor declared in `go.mod`.
  - **Security — build with Go ≥ 1.26.6.** Four Go **standard library** advisories are reachable from this SDK's own call graph when compiled with go1.26.5 — [GO-2026-6218](https://pkg.go.dev/vuln/GO-2026-6218), [GO-2026-6090](https://pkg.go.dev/vuln/GO-2026-6090), [GO-2026-5972](https://pkg.go.dev/vuln/GO-2026-5972) (`assembly.Sidecar.Start` reaches `asn1.Unmarshal`) and [GO-2026-5026](https://pkg.go.dev/vuln/GO-2026-5026) (`assembly.probeHealthz` reaches `http.Client.Do`) — all fixed in go1.26.6. [GO-2026-5856](https://pkg.go.dev/vuln/GO-2026-5856), a `crypto/tls` Encrypted-Client-Hello (ECH) privacy leak, was fixed earlier in go1.26.5. Because the vulnerable code lives in the stdlib, the fix ships with *your* compiler, not this module — the module deliberately keeps a broad `go 1.26.0` floor (and pins `toolchain go1.26.6` for its own build and `govulncheck` gate), so a consumer compiling go-sdk with Go 1.26.0–1.26.5 still links a vulnerable stdlib. Security-conscious builds should use Go ≥ 1.26.6 to pick up the fixes.
- For production: an operator-issued gateway URL and API key. For local development you can skip both — `Init` discovers a gateway on `http://localhost:7391` (see [Configuration](docs/configuration.md#gateway-and-credential-resolution)).
- *(Optional)* a C compiler — only needed if you build with `-tags aa_ffi_go` to enable the native FFI transport. The default transport is pure-Go and runs cleanly with `CGO_ENABLED=0`.

## Installation

```bash
go get github.com/ai-agent-assembly/go-sdk
```

### Metadata at a glance

<!-- BEGIN GENERATED: sdk-metadata -->
<!-- GENERATED BY scripts/gen-metadata.go — DO NOT EDIT. -->

| Field | Value |
| --- | --- |
| Module | `github.com/ai-agent-assembly/go-sdk` |
| Protocol version | `0.0.1-rc.6` |
| Go floor | `>= 1.26` |
| Docs | <https://docs.agent-assembly.com/go-sdk/> |
| Releases | <https://github.com/ai-agent-assembly/go-sdk/releases> |

Install:

```bash
go get github.com/ai-agent-assembly/go-sdk
```
<!-- END GENERATED: sdk-metadata -->

### Supply-chain verification

The **only** canonical module is:

```text
github.com/ai-agent-assembly/go-sdk
```

Anything else (a different owner, a typosquat, or a vendored copy you did not pull yourself) is **not** us. Verify what you actually pulled:

- **Checksum DB** — `go get` and `go mod download` authenticate every module version against the public Go checksum database (`sum.golang.org`) and record the hashes in `go.sum`. A mismatch fails the build, so a tampered or substituted module cannot install silently.
- **Re-verify on demand** — run `go mod verify` to confirm the modules in your local cache still match `go.sum`. To force a clean re-fetch and re-check against the checksum DB, clear the cache (`go clean -modcache`) then `go mod download github.com/ai-agent-assembly/go-sdk`. Keep the default checksum settings (do not set `GOFLAGS=-insecure` or add the module to `GONOSUMDB`/`GOPRIVATE`), which would disable this check.
- **SBOM** — every release tag attaches a CycloneDX SBOM (`sbom.cdx.json`) to its [GitHub Release](https://github.com/ai-agent-assembly/go-sdk/releases), generated by the tag-triggered release workflow. Cross-check it against your advisory feed to audit the dependency set.
- **Release gate** — a release tag only publishes after `go mod verify` + `govulncheck` pass in CI, so a release cannot ship on a known-vuln dependency.

## Layout

```text
assembly/
  init.go
  runtime.go
  options.go
  defaults.go
  validation.go
  governance_client.go
  policy_model.go
  governance_errors.go
  tool_wrapper.go
  wrap_tools.go
  sidecar.go
  interceptor.go
examples/minimal/
```

## Quick Start

> [!WARNING]
> **`Init` requires a reachable sidecar address or a managed binary, or it returns `ErrSidecarUnavailable`.** The snippet below passes `WithSidecarAddress` pointing at a running gateway's gRPC endpoint (default `127.0.0.1:50051`) — without it (or `WithSidecarBinary` to have the SDK manage one), `Init` falls through to `ErrSidecarUnavailable` and the `log.Fatal` below fires. And even with a reachable address, agent registration itself is not reachable from a plain `go get` today — following this quick start will not make your agent appear in the dashboard. The register handshake runs *only* under the opt-in native cgo binding (`-tags aa_ffi_go`, `CGO_ENABLED=1`), and that native library (`libaa_ffi_go`) is **not published anywhere** yet. The default pure-Go build has no native transport, so it does not register even when `WithSidecarAddress` is set. See [docs/quick-start.md](docs/quick-start.md) for the full explanation; track status in [AAASM-4547](https://lightning-dust-mite.atlassian.net/browse/AAASM-4547) and [AAASM-4469](https://lightning-dust-mite.atlassian.net/browse/AAASM-4469).

```go
import (
    "context"
    "log"

    "github.com/ai-agent-assembly/go-sdk/assembly"
)

ctx := assembly.WithAgentID(context.Background(), "my-agent")
a, err := assembly.Init(ctx,
    assembly.WithGatewayURL(url),
    assembly.WithAPIKey(key),
    assembly.WithSidecarAddress("127.0.0.1:50051"), // required — see warning above
)
if err != nil {
    log.Fatal(err)
}
defer a.Close()
```

`WithAgentID` attaches the calling agent's identity to `ctx`; the SDK stamps it (and any `WithTraceID` / `WithRunID` values) onto every `Check` and `RecordResult`. The `Check` carries them to the gateway, and the `RecordResult` is written to the runtime's event channel when one is connected — a handoff, not retained evidence (see the note at the top). See [Context Propagation](#context-propagation) below for the full set of context helpers.

Then wrap your agent's tools so every call is governed:

```go
governed := assembly.WrapTools(myTools, nil)
```

The second argument is the `GovernanceClient` that talks to the gateway. Under the default fail-closed enforce posture, passing `nil` denies every wrapped call (`ErrGovernanceUnavailable`) rather than running it unchecked (AAASM-3109) — wire in a real client when you're ready to enforce. To get a passthrough wrapper instead (tools run, no gateway calls) while you're wiring one up, pass `assembly.WithFailClosed(false)` as an option. Each call against a tool in `governed` is then checked against the gateway policy before it runs, and its outcome is offered to `RecordResult` after — the shipped client forwards that record to a connected runtime (see the note at the top). Hand `governed` to your agent in place of the originals. See [Quick Start](docs/quick-start.md) for the end-to-end walk-through.

## Documentation

- **Live site** — [docs.agent-assembly.com/go-sdk](https://docs.agent-assembly.com/go-sdk/) (Hugo, Hextra theme; built and deployed from `main`).
- **API reference** — [pkg.go.dev/github.com/ai-agent-assembly/go-sdk](https://pkg.go.dev/github.com/ai-agent-assembly/go-sdk) (auto-generated from godoc; preview locally with `godoc -http=:6060`).
- **Core concepts** — [docs/core-concepts.md](docs/core-concepts.md) and [docs/api-reference.md](docs/api-reference.md).
- **Contributing** — [CONTRIBUTING.md](CONTRIBUTING.md).

## AI Agent Assembly ecosystem

- **Organization** — [github.com/ai-agent-assembly](https://github.com/ai-agent-assembly): org profile and the full production-repo map.
- **Core runtime** — [ai-agent-assembly/agent-assembly](https://github.com/ai-agent-assembly/agent-assembly): the gateway, policy engine, proxy, and eBPF layers this SDK talks to.
- **Protocol spec** — the gateway wire protocol this SDK is pinned to lives in the core monorepo at [docs/src/protocol](https://github.com/ai-agent-assembly/agent-assembly/tree/HEAD/docs/src/protocol).
- **Canonical docs** — the org-wide documentation site at [docs.agent-assembly.com](https://docs.agent-assembly.com/).
- **Runnable examples** — [ai-agent-assembly/examples](https://github.com/ai-agent-assembly/examples): learn by running small, framework-specific Go (and Python/Node) samples for policy enforcement, approvals, audit, trace, and runtime workflows.
- **Release notes** — [github.com/ai-agent-assembly/go-sdk/releases](https://github.com/ai-agent-assembly/go-sdk/releases).

## Support & Security

- **Questions / bugs / feature requests** — open an issue at [github.com/ai-agent-assembly/go-sdk/issues](https://github.com/ai-agent-assembly/go-sdk/issues).
- **Security vulnerabilities** — do **not** file a public issue; report privately via the [security policy](https://github.com/ai-agent-assembly/go-sdk/security/policy).
- **Troubleshooting** — common errors and fixes are in [docs/troubleshooting.md](docs/troubleshooting.md).

## Development

```bash
make fmt              # gofmt -w on all .go files
make lint             # golangci-lint run ./...
make test             # go test ./...

go vet ./...
go test ./assembly                            # one package
go test ./assembly -run TestRegisterAgent     # one test
go test -count=1 -race ./...                  # race detector
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contributor workflow, including the optional CGo native FFI build (`-tags aa_ffi_go`) and the memory regression harness.

## Context Propagation

- `assembly.WithAgentID` / `assembly.AgentIDFromContext` propagate and read agent identity.
- `assembly.WithTraceID` / `assembly.TraceIDFromContext` propagate explicit trace IDs and fallback to OpenTelemetry span context trace ID when unset.
- `assembly.WithRunID` / `assembly.RunIDFromContext` propagate run identity.
- `assembly.EnsureRunID` guarantees a stable run ID within the same context tree.

## Gateway Context Handling

- `GatewayClient.Check` fails fast when `ctx` is already cancelled.
- If `ctx` has no deadline, `GatewayClient.Check` applies SDK timeout defaults (`500ms` unless overridden by `WithTimeout`).
- The final effective context (values + timeout/deadline) is passed to the gateway transport check call.

## FFI Transport

- CGo bridge module lives in `internal/ffi/cgo_bridge.go` with `#cgo LDFLAGS: -laa_ffi_go`.
- Native FFI path is enabled with build tags: `-tags aa_ffi_go` (and `CGO_ENABLED=1`).
- Pure-Go UDS fallback is selected automatically when `aa_ffi_go` tag is not set.
- `CGO_ENABLED=0` is explicitly supported via fallback transport and CI matrix coverage.
- Optional memory harness test (1M sends): `AAASM_MEMORY_HARNESS=1 go test ./internal/ffi -run TestMemoryRegressionHarness`.

## SonarQube CI Setup

Configure these repository settings for the `SonarQube` workflow:

- Secret: `SONAR_TOKEN`
- Variable: `SONAR_HOST_URL` (for SonarCloud use `https://sonarcloud.io`)
- Variable: `SONAR_PROJECT_KEY`
- Variable: `SONAR_ORGANIZATION` (required for SonarCloud, optional for self-hosted SonarQube)
