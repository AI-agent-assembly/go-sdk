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
// Under ADR 0033 §6 the term each value earns differs, so the disposition is
// what a claim must be read off:
//
//   - [AuditSinkForwarded] — the record crosses the native boundary into the
//     runtime's evidence pipeline, which is what *Observed* requires: a durable
//     event attributed to the action.
//   - [AuditSinkDiscarded] — the record is built and dropped. No evidence, and
//     the sink that would carry it is not present in this configuration:
//     *Unsupported*.
//   - [AuditSinkAbsent] — no record is attempted, because no governance client
//     was resolved. The control is configured and unavailable: *Degraded*.
//   - [AuditSinkCallerSupplied] — no claim; whatever the caller's client earns.
type AuditSinkDisposition string

const (
	// AuditSinkForwarded means the record crosses the native FFI event channel
	// into the runtime, on the same session and by the same primitive
	// (ffi.Client.SendEvent) that already carries the boot "register" event. The
	// runtime enriches, re-scans and admits it to its audit pipeline, so the
	// event reaches the evidence pipeline rather than stopping in this process
	// (AAASM-5750).
	//
	// It says "forwarded", not "recorded", on purpose. This SDK observes that the
	// event crossed the boundary; retention past that point belongs to the
	// runtime and the gateway behind it, and claiming their outcome from here
	// would be broadening what this layer can see (ADR 0034).
	AuditSinkForwarded AuditSinkDisposition = "forwarded"

	// AuditSinkDiscarded means the record is built and handed to the client,
	// which drops it. The call site is correct and the sink is not: swapping in
	// a retaining client is all that is missing. In this SDK it is what
	// [GovernanceClient.RecordResult] falls back to when the governance client
	// was built over a policy querier that carries no event channel — a shape
	// the shipped runtime path does not produce, but which a partially-wired
	// client can.
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
