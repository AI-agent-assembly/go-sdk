---
title: "ADR 0001 — Go module path must be go-gettable"
weight: 99
excludeSearch: true
sidebar:
  exclude: true
---

<!--
Architecture Decision Record. Internal maintainer convention, not user-facing
SDK documentation. Excluded from the docs-site sidebar nav and search on purpose.
-->

# ADR 0001 — Go module path must be go-gettable

- **Status:** Proposed
- **Date:** 2026-06-27
- **Ticket:** AAASM-3836
- **Deciders:** go-sdk maintainers (Pioneer team)
- **Component:** go-sdk

## Context

Unlike PyPI (`agent-assembly`) and npm (`@agent-assembly/sdk`), where the package
name is decoupled from the source-host location, **a Go module's import path *is*
its download location.** `go get`, the public module proxy (`proxy.golang.org`),
and the checksum database (`sum.golang.org`) all resolve a module by performing a
VCS lookup against the host named in the `module` directive. There is no separate
registry that maps a brand name to a repository.

The repository physically lives at **`github.com/ai-agent-assembly/go-sdk`**
(GitHub org `ai-agent-assembly`, lowercased per the org-ID convention). The
product brand used for the other distribution identities is `agent-assembly`
(PyPI project, npm scope). At bootstrap, the Go module was declared as the brand
path `github.com/agent-assembly/go-sdk` to match that brand.

That brand path does not resolve:

| Path | `github.com/<path>` page | `proxy.golang.org/<path>/@v/list` |
|---|---|---|
| `github.com/agent-assembly/go-sdk` (brand) | `404` | `404` |
| `github.com/ai-agent-assembly/go-sdk` (repo) | `200` | `200` |

The GitHub org `agent-assembly` does not exist (`github.com/agent-assembly` → `404`),
so the brand path has **no backing VCS host**. Consequently:

- `go get github.com/agent-assembly/go-sdk` fails for every external consumer —
  the proxy cannot perform the VCS lookup, so no version (tagged or pseudo) is
  fetchable.
- No working consumer of the brand path can exist. Any consumer that compiled did
  so only against a local `replace` directive or a `GOFLAGS=-mod=mod` checkout —
  never via the public toolchain. There is no published, resolvable artifact to
  preserve backward compatibility with.

This is therefore an **identity/brand decision**, not a mechanical typo fix: do we
make the brand path resolve, or do we adopt the repo path as the module identity?

### Current state of the tree

For honesty of the record: the working module path on `master` has **already been
migrated** to the repo path. Git history shows the sequence
`agent-assembly` → `AI-agent-assembly` → `ai-agent-assembly`
(commits `e9ca2e6` → `d94b177` → `facc3af`). Today `go.mod` declares
`module github.com/ai-agent-assembly/go-sdk`, the `README.md` and
`docs/quick-start.md` install lines already read `go get github.com/ai-agent-assembly/go-sdk`,
and the tagged release `v0.0.1-rc.2` is fetchable from the public proxy (`200`).
This ADR records the decision behind that state and the rationale for keeping it,
rather than reverting to the brand path.

## Decision drivers

- **Resolvability (hard requirement):** the published module path must be
  go-gettable through the public proxy with zero per-consumer configuration.
- **Brand consistency:** ideally the Go import path would echo the `agent-assembly`
  brand used by the other SDKs.
- **Infrastructure cost & ownership:** any solution that requires standing up and
  *indefinitely owning* new infrastructure (a second GitHub org, or a vanity-domain
  web service) is a recurring liability.
- **Consumer impact / churn:** Go import paths are copied verbatim into downstream
  `import` blocks and `go.mod` files; changing the path later is a breaking change
  for consumers.

## Options considered

### Option A — Keep the brand path `github.com/agent-assembly/go-sdk`

Preserve the brand import path and make it resolve, via one of:

- **A1 — Create and own the `agent-assembly` GitHub org + a repo mirror.** Register
  the org, host `go-sdk` there (mirror or move), and publish matching tags. The
  module path then resolves directly.
- **A2 — `go-import` meta-tag vanity redirect on an `agent-assembly.com` host.**
  Serve `<meta name="go-import" content="agent-assembly.com/go-sdk git https://github.com/ai-agent-assembly/go-sdk">`
  (path rooted at `agent-assembly.com/go-sdk`, not the GitHub brand path) so the Go
  toolchain redirects the vanity path to the real repo.

**Consequences**
- (+) Import path carries the `agent-assembly` brand, matching PyPI/npm.
- (−) A1 requires acquiring and *permanently governing* a second GitHub org
  (squatting risk, CI/secrets/CODEOWNERS duplication, release-tag duplication,
  split discoverability). It also does **not** make the bare
  `github.com/agent-assembly/...` path work unless that exact org name is the one
  created — and that fragments the org identity away from `ai-agent-assembly`,
  which every other repo and the lowercased org-ID convention already use.
- (−) A2 introduces a **always-on web dependency** in the critical path of every
  `go get`: if `agent-assembly.com` is down, misconfigured, or its TLS lapses, the
  module becomes uninstallable. It also means the canonical import path is
  `agent-assembly.com/go-sdk` — a *third* distinct string, neither the brand
  GitHub path nor the repo path — adding cognitive load.
- (−) Either sub-option is net-new infrastructure to satisfy a cosmetic goal.

### Option B — Adopt the repo path `github.com/ai-agent-assembly/go-sdk`

Declare the module as its actual VCS location. (Already implemented on `master`.)

**Consequences**
- (+) Resolves with **zero new infrastructure** — the repo already exists and the
  public proxy already serves it (`v0.0.1-rc.2` fetches `200`).
- (+) No always-on runtime dependency for installation; nothing to keep alive.
- (+) Import path matches where the code actually lives — what a developer browsing
  GitHub expects, and what `go mod` tooling reports.
- (−) The Go import path reads `ai-agent-assembly`, diverging from the
  `agent-assembly` brand string used on PyPI/npm. This is an inherent property of
  Go (path = host); the brand is still conveyed by the repo/org name and docs.
- (−) Consumer churn **from the brand path is zero in practice** — the brand path
  never resolved, so there are no public consumers to break. Internal/local
  `replace`-based consumers update one line.

## Decision

**Adopt Option B — `github.com/ai-agent-assembly/go-sdk` is the canonical Go module
path.** Recommendation: **B.**

Rationale: resolvability is a hard requirement and Go ties the import path to the
VCS host, so the path *must* name a real, owned location. Option B satisfies that
with zero new infrastructure and zero ongoing operational liability, while Option A
buys brand-string consistency at the cost of either a second permanently-governed
GitHub org (A1) or an always-on vanity-redirect service in the install critical
path (A2). Brand identity is adequately carried by the org/repo name, the PyPI/npm
identities, and the docs at `docs.agent-assembly.com`; paying recurring
infrastructure cost to also force it into the Go import string is not justified.
The existing tree already reflects this decision, so adopting B also avoids a
gratuitous breaking re-rename.

## Consequences

- The canonical install instruction is `go get github.com/ai-agent-assembly/go-sdk`
  (already current in `README.md` and `docs/quick-start.md`).
- Future consumers depend on the `ai-agent-assembly` path; this path is now a
  stable public contract and must not be renamed again without a major-version /
  breaking-change process.
- Brand divergence between the Go import path (`ai-agent-assembly`) and the
  PyPI/npm brand (`agent-assembly`) is accepted and should be noted wherever
  cross-SDK install instructions are presented together.

## Follow-up implementation steps (for the chosen option, B)

The code-level migration to Option B is already on `master`; remaining work is
verification and guarding against regression — to be done under **separate tickets**,
not this ADR PR (this PR records the decision only):

1. **Verify (no source change here):** confirm `go.mod` `module` directive, all
   `*.go` import paths, `README.md`, and `docs/` consistently use
   `github.com/ai-agent-assembly/go-sdk`. (Confirmed at time of writing.)
2. **Release tooling** — ensure `.goreleaser.yaml` and release tagging reference the
   repo path. *Owned by **AAASM-3838**.*
3. **Skills/automation** — ensure `.claude/skills/**` references the repo path.
   *Owned by **AAASM-3841**.*
4. **Smoke test** — from a clean module cache, run
   `GOFLAGS= go get github.com/ai-agent-assembly/go-sdk@v0.0.1-rc.2` to confirm the
   public proxy + checksum DB path end-to-end.
5. **Promote this ADR to `Accepted`** once the above are confirmed across the
   sibling tickets.

## Consumer impact summary

| Option | New infra to own | Install critical-path dependency | Import path consumers write | Breakage vs. today |
|---|---|---|---|---|
| A1 (brand org) | A second GitHub org, permanently | none extra | `github.com/agent-assembly/go-sdk` | none public (brand path never resolved); re-rename churns the already-shipped `ai-agent-assembly` path |
| A2 (vanity redirect) | `agent-assembly.com` go-import host, permanently | the vanity host on every `go get` | `agent-assembly.com/go-sdk` | as A1, plus a new runtime dependency |
| **B (repo path)** | **none** | **none** | `github.com/ai-agent-assembly/go-sdk` | **none** (already current) |
