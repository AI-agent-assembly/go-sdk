package assembly

import (
	"context"
	"fmt"

	"github.com/ai-agent-assembly/go-sdk/internal/ffi"
)

// actionTypeToolCall is the snake_case proto action name used when querying the
// runtime for a tool-call policy decision. It matches the shared enforcement
// contract used by the Python and Node SDKs.
const actionTypeToolCall = "tool_call"

// policyQuerier is the subset of *ffi.Client used to query the runtime for a
// policy decision. It is an interface so tests can substitute a fake without a
// live native binding.
type policyQuerier interface {
	QueryPolicy(agentID, actionType, toolName, argsJSON string) (decision int32, reason string, err error)
}

// ffiGovernanceClient is the production GovernanceClient. Its Check delegates to
// the native aa_query_policy primitive (AAASM-3048) through the FFI client and
// maps the returned decision onto a Decision per the shared enforcement
// contract:
//
//	deny                          -> Decision{Denied: true, Reason}
//	allow                         -> Decision{}
//	redact                        -> error (fail-closed under enforce): the native
//	                                 aa_query_policy ABI returns only
//	                                 (decision, reason) and carries no redacted
//	                                 content, so the SDK cannot honour the
//	                                 redaction. Folding it onto a plain allow would
//	                                 run the tool with the unredacted args the
//	                                 policy wanted scrubbed (AAASM-4788); full
//	                                 field-level redaction needs a native-shim ABI
//	                                 change (AAASM-1920)
//	unspecified                   -> error (fail-closed under enforce): the
//	                                 proto3 zero value is not an authoritative
//	                                 allow (AAASM-4166)
//	pending                       -> Decision{Pending: true}; the wrapper then
//	                                 calls WaitForApproval, which denies (no
//	                                 approval channel exists — AAASM-3920)
//
// The native query is fail-closed: an unreachable, slow, or closed runtime
// surfaces a non-OK status, which QueryPolicy returns as an error rather than a
// silent allow (see internal/ffi/query_policy.go and internal/ffi/status.go).
// That error is returned to the caller; the tool wrapper then honours
// WithFailClosed to decide whether the call proceeds or is denied.
type ffiGovernanceClient struct {
	querier policyQuerier
}

// newFFIGovernanceClient builds a GovernanceClient backed by the FFI client.
func newFFIGovernanceClient(querier policyQuerier) *ffiGovernanceClient {
	return &ffiGovernanceClient{querier: querier}
}

// Check queries the runtime for a policy decision on a tool call. It fails
// fast when the context is already cancelled (AAASM-4194), avoiding a blocking
// FFI call that cannot be interrupted once started.
func (c *ffiGovernanceClient) Check(ctx context.Context, request CheckRequest) (Decision, error) {
	// Check context cancellation before the FFI call: the native aa_query_policy
	// primitive is synchronous and cannot be interrupted once started, so an
	// already-cancelled context must short-circuit here (AAASM-4194).
	if err := ctx.Err(); err != nil {
		return Decision{}, fmt.Errorf("assembly: context cancelled before policy check: %w", err)
	}

	if c.querier == nil {
		// No querier means there is no runtime to render a verdict. Returning a
		// plain allow here (the previous behaviour) would silently pass every tool
		// call unchecked — the exact fail-open this client exists to prevent.
		// Surface an error so the tool wrapper applies its posture (deny under the
		// fail-closed enforce default) rather than allowing (AAASM-4811).
		return Decision{}, fmt.Errorf("assembly: no policy querier configured; cannot render a verdict (fail-closed)")
	}

	decision, reason, err := c.querier.QueryPolicy(
		request.AgentID,
		actionTypeToolCall,
		request.ToolName,
		request.Args,
	)
	if err != nil {
		return Decision{}, err
	}

	switch decision {
	case ffi.DecisionDeny:
		return Decision{Denied: true, Reason: reason}, nil
	case ffi.DecisionPending:
		return Decision{Pending: true, Reason: reason}, nil
	case ffi.DecisionAllow:
		// A plain allow proceeds.
		return Decision{}, nil
	case ffi.DecisionRedact:
		// REDACT means "allow, but with the policy's sensitive fields redacted".
		// The native aa_query_policy ABI hands back only (decision, reason) — it
		// carries no redacted args/fields — so this SDK has nothing to substitute
		// and cannot honour the redaction. Folding REDACT onto a plain allow (the
		// previous behaviour) ran the tool with the *unredacted* arguments the
		// policy explicitly wanted scrubbed, silently downgrading REDACT to full
		// ALLOW (AAASM-4788). Surface an error instead so the tool wrapper applies
		// its posture: deny under the fail-closed enforce default, advisory-allow
		// (logged) only under observe/disabled or when fail-open was opted into —
		// exactly as it handles a non-authoritative UNSPECIFIED verdict. Honouring
		// the redaction properly needs a native-shim ABI change to return the
		// redacted payload (tracked with AAASM-1920).
		return Decision{}, fmt.Errorf("assembly: REDACT verdict cannot be honoured client-side: the native shim provides no redacted content (AAASM-4788; needs AAASM-1920)")
	case ffi.DecisionUnspecified:
		// The proto3 zero value UNSPECIFIED means "no decision rendered" — a
		// non-authoritative verdict. It must NOT proceed as a silent allow: surface
		// an error so the tool wrapper applies its posture (deny under the default
		// enforce; allow only when fail-closed is disabled or under observe /
		// disabled), matching the Node SDK's fail-closed handling (AAASM-4166).
		return Decision{}, fmt.Errorf("assembly: non-authoritative UNSPECIFIED policy verdict")
	default:
		// A decision code this SDK does not recognise — an out-of-range value
		// or a variant a newer gateway added after this SDK was built (version
		// skew). Do not silently allow it: surface an error so the tool wrapper
		// applies its fail-open / fail-closed posture (deny under the default
		// enforce; allow only when fail-closed is disabled or under observe /
		// disabled), exactly as it does for a transport error (AAASM-4019). The
		// native shim already folds an unrecognised proto verdict onto deny;
		// this is the Go-side defence-in-depth for any other querier.
		return Decision{}, fmt.Errorf("assembly: unrecognized policy decision code %d", decision)
	}
}

// WaitForApproval resolves a pending (requires-approval) decision. There is no
// native approval-polling primitive yet, so there is no channel through which an
// operator could ever approve the held call. Returning allow here (the previous
// behaviour) silently downgraded every requires-approval verdict to allow,
// defeating the gateway's hold (AAASM-3920). With no way to obtain approval the
// only safe resolution is to deny, so a pending decision blocks the tool rather
// than running it unapproved. The runtime / proxy / eBPF layers remain
// authoritative; this is the SDK's defence-in-depth fail-closed posture.
//
// Context cancellation is checked first (AAASM-4194): a cancelled context should
// abort the wait rather than proceeding to a denial verdict.
func (c *ffiGovernanceClient) WaitForApproval(ctx context.Context, _ ApprovalRequest) (Decision, error) {
	if err := ctx.Err(); err != nil {
		return Decision{}, fmt.Errorf("assembly: context cancelled while waiting for approval: %w", err)
	}

	return Decision{
		Denied: true,
		Reason: "tool call requires approval but no approval channel is available; denying (fail-closed)",
	}, nil
}

// RecordResult discards the record and reports success (AAASM-5731).
//
// The previous comment here said tool results are "reported to the runtime
// through the FFI event channel by the Assembly runtime, not through this
// client". Measured against the native boundary, that is not what happens for a
// tool call: ffi.Client.SendEvent — the SDK's only outbound event channel, see
// the binding interface in internal/ffi/client.go — has exactly one non-test
// call site, runtime.go's boot "register" event. No tool-call result crosses it,
// on the allowed path or the denied one. Both parameters are dropped and the nil
// return is indistinguishable from a retained record.
//
// This is not an enforcement gap: [AssemblyTool.Call] still denies on a runtime
// DENY, and the runtime / proxy / eBPF layers remain authoritative. It is an
// evidence gap, and it is declared rather than left implicit — see AuditSink
// below. Giving this client a sink that retains the record is a new capability
// (an FFI record path), not a fix to apply here; under ADR 0033 §6 that is
// **Planned** (AAASM-5750), not *Observed*.
func (c *ffiGovernanceClient) RecordResult(_ context.Context, _ RecordRequest) error {
	return nil
}

// AuditSink declares that this client drops the hook-layer audit record it is
// given, so [ResolveAuditSink] can surface the gap to the caller and a test can
// catch a shipped client that discards without saying so (AAASM-5731).
func (c *ffiGovernanceClient) AuditSink() AuditSinkDisposition {
	return AuditSinkDiscarded
}

// Close releases no resources; the underlying FFI client lifecycle is owned by
// the Assembly runtime.
func (c *ffiGovernanceClient) Close() error {
	return nil
}

var (
	_ GovernanceClient  = (*ffiGovernanceClient)(nil)
	_ AuditSinkDeclarer = (*ffiGovernanceClient)(nil)
)
