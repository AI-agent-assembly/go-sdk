package assembly

// AuditSinkDisposition says what a [GovernanceClient] does with the hook-layer
// audit record the tool wrapper offers it — [GovernanceClient.RecordResult]
// (AAASM-5731).
//
// RecordResult returns only an error, so a client that retains the record and a
// client that throws it away are indistinguishable to the caller: both return
// nil. Every client this SDK *ships* therefore has to say which it is, because
// no downstream claim of attributability or after-the-fact review holds on a
// path that produces no evidence, and until this type existed there was no way
// to find that out short of reading the implementation.
//
// The vocabulary distinguishes *how* a record is or is not retained, not just
// that it is not, because the failures have different blast radii and the Node
// and Python SDKs sit on opposite sides of the split — see the constants. It is
// deliberately not a boolean, and deliberately has no "recorded" value: this SDK
// hands the record to the runtime and cannot itself observe what the runtime
// then keeps, so the strongest thing it may say for its own client is
// [AuditSinkForwarded] — and for anyone else's, the absence of a claim.
//
// **No value here earns ADR 0033 §6 *Observed*.** §6's evidence column requires
// *a durable event attributed to the action*, and this SDK cannot establish that
// from its side of the FFI — see [AuditSinkForwarded]. What the disposition
// reports is where the record was handed, not where it ended up:
//
// The gap is tracked as AAASM-5783: until report_event payloads reach the live
// stream and the durable entry, no SDK can claim *Observed*. Revisit these terms
// when it lands — not before.
//
//   - [AuditSinkForwarded] — the record crosses the native boundary to the
//     runtime. A handoff, not an arrival, and not evidence.
//   - [AuditSinkDiscarded] — the record is built and dropped, because the client
//     holds no event channel. The **action** is §6 *Unmeasured*: no durable event
//     attributed to it exists. "No sink in this configuration" is an availability
//     statement about the capability — a different question, which ADR 0034 §2.5
//     makes incomparable with the action term rather than a stronger form of it.
//   - [AuditSinkAbsent] — no record is attempted, because no governance client
//     was resolved. The action is *Unmeasured* for the same reason, more plainly.
//     Not *Degraded*: §6 evidences that term with a LayerDegradation event or an
//     ADR 0030 Degraded state carrying both the planned and the achieved level
//     (manifest row G6 is what earns it), and this SDK emits neither. A stderr
//     warning is not that pair.
//   - [AuditSinkCallerSupplied] — no claim; whatever the caller's client earns.
type AuditSinkDisposition string

const (
	// AuditSinkForwarded means the record crosses the native FFI event channel to
	// the runtime, on the same session and by the same primitive
	// (ffi.Client.SendEvent) that already carries the boot "register" event
	// (AAASM-5750).
	//
	// **The handoff is the whole of the claim, and it is weaker than it looks.**
	// It says "forwarded", not "recorded", because three separate things this SDK
	// cannot see stand between the send and any durable evidence: the send is
	// fire-and-forget with no acknowledgement, the record is dispatched on a
	// goroutine nothing joins (see [AssemblyTool.recordOutcome]), and what the
	// runtime and gateway retain is theirs to state. Do not read *Observed* off
	// this value — that needs a durable event attributed to the action, which is
	// not established here.
	//
	// **Not reachable from a released artifact.** The native binding is compiled
	// only under `-tags aa_ffi_go` with cgo; the default build selects the
	// fallback, whose sendEvent returns statusRuntimeUnavailable even against a
	// live listener, and `libaa_ffi_go` is not published anywhere (see
	// docs/quick-start.md). So today this value is reachable only from a
	// custom-built binary.
	AuditSinkForwarded AuditSinkDisposition = "forwarded"

	// AuditSinkDiscarded means the record is built and handed to the client,
	// which drops it, because that client holds no event channel to send it on.
	//
	// **It has no production population in this SDK.** newFFIGovernanceClient is
	// unexported and boot only ever calls it with the connected *ffi.Client, so
	// the only way to reach this value is a test double that implements
	// QueryPolicy and not SendEvent. It is kept because the disposition is
	// computed rather than fixed, and a computed value that cannot report the
	// negative case is not a measurement — but no released configuration
	// produces it.
	AuditSinkDiscarded AuditSinkDisposition = "discarded"

	// AuditSinkAbsent means no record is even attempted — there is no sink to
	// offer it to. Strictly worse than [AuditSinkDiscarded]: nothing constructs
	// the event, so wiring a sink is not sufficient on its own. In this SDK it
	// is the no-runtime path, where the governance client is nil and the tool
	// wrapper has nothing to call.
	AuditSinkAbsent AuditSinkDisposition = "absent"

	// AuditSinkCallerSupplied means the client did not come from this SDK, so
	// this SDK makes no claim either way. It is the **absence of a claim, not an
	// assurance** that the record is retained — whichever §6 term the caller's
	// own client earns is the caller's to establish, not this SDK's to assert.
	AuditSinkCallerSupplied AuditSinkDisposition = "caller-supplied"
)

// AuditSinkDeclarer is implemented by a [GovernanceClient] that declares what it
// does with hook-layer audit records.
//
// Kept separate from [GovernanceClient] rather than added to it because
// GovernanceClient is exported and callers implement it: adding a fourth method
// there would stop every existing caller implementation from satisfying the
// interface, which is a compile break introduced by a diagnostic. An optional
// interface plus a type assertion is the idiomatic Go equivalent of the optional
// field `node-sdk` added for the same reason, and it keeps the honest default —
// a client that says nothing is [AuditSinkCallerSupplied], not assumed to
// record.
//
// Every client this SDK ships MUST implement it; assembly's audit-sink test
// enumerates the package's own GovernanceClient implementations structurally and
// fails on one that does not.
type AuditSinkDeclarer interface {
	AuditSink() AuditSinkDisposition
}

// ResolveAuditSink reports what client does with hook-layer audit records.
//
// A nil client is [AuditSinkAbsent], not "unknown": that is the no-runtime path,
// where [AssemblyTool.Call] never reaches RecordResult at all because the client
// *is* the sink. A non-nil client that does not implement [AuditSinkDeclarer]
// came from the caller, so this SDK declines to claim anything about it.
func ResolveAuditSink(client GovernanceClient) AuditSinkDisposition {
	if client == nil {
		return AuditSinkAbsent
	}
	if declarer, ok := client.(AuditSinkDeclarer); ok {
		return declarer.AuditSink()
	}
	return AuditSinkCallerSupplied
}
