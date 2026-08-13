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
// The vocabulary distinguishes *how* a record fails to be retained, not just
// that it does, because the two failures have different blast radii and the
// Node and Python SDKs sit on opposite sides of the split — see the constants.
// It is deliberately not a boolean, and deliberately has no "recorded" value:
// this SDK can only speak for clients it built, so the honest answer for
// anything else is the absence of a claim, not an assurance.
//
// Under ADR 0033 §6, a client declaring anything other than
// [AuditSinkCallerSupplied] makes SDK-side recording **Planned** (AAASM-5731),
// never *Observed* — *Observed* requires a durable event attributed to the
// action, and there is none.
type AuditSinkDisposition string

const (
	// AuditSinkDiscarded means the record is built and handed to the client,
	// which drops it. The call site is correct and the sink is not: swapping in
	// a retaining client is all that is missing. This is what
	// [GovernanceClient.RecordResult] does on the only client this SDK ships,
	// and it matches what `node-sdk` declares for its two shipped clients.
	AuditSinkDiscarded AuditSinkDisposition = "discarded"

	// AuditSinkAbsent means no record is even attempted — there is no sink to
	// offer it to. Strictly worse than [AuditSinkDiscarded]: nothing constructs
	// the event, so wiring a sink is not sufficient on its own. In this SDK it
	// is the no-runtime path, where the governance client is nil and the tool
	// wrapper has nothing to call; `python-sdk` is in this state on every path
	// because its adapters' audit hook does not resolve at all (AAASM-5731).
	AuditSinkAbsent AuditSinkDisposition = "absent"

	// AuditSinkCallerSupplied means the client did not come from this SDK, so
	// this SDK makes no claim either way. It is the **absence of a claim, not an
	// assurance** that the record is retained — an *Observed* claim for the hook
	// layer is available only on this branch, and only if the caller's own
	// client actually retains what it is given.
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
