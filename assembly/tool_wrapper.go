package assembly

import (
	"context"
	"errors"
	"fmt"
	"log"
)

// Tool is the minimal tool contract used by this SDK package.
type Tool interface {
	Name() string
	Description() string
	Call(ctx context.Context, input string) (string, error)
}

// AssemblyTool wraps a Tool with governance hooks.
type AssemblyTool struct { //nolint:revive // Keep API name aligned with AAASM-63 contract.
	inner     Tool
	client    GovernanceClient
	opts      runtimeOptions
	opControl OpController
}

// newAssemblyTool constructs a governance wrapper around a tool. When opts
// carries an op-control subscriber (via [WithOpControl]), the wrapper consults
// the gateway's live kill switch before each tool call (AAASM-3501).
func newAssemblyTool(inner Tool, client GovernanceClient, opts runtimeOptions) *AssemblyTool {
	return &AssemblyTool{
		inner:     inner,
		client:    client,
		opts:      opts,
		opControl: opts.opControl,
	}
}

// Name passes through the wrapped tool name.
func (t *AssemblyTool) Name() string {
	if t.inner == nil {
		return ""
	}
	return t.inner.Name()
}

// Description passes through the wrapped tool description.
func (t *AssemblyTool) Description() string {
	if t.inner == nil {
		return ""
	}
	return t.inner.Description()
}

// Call executes governance hooks before and after tool execution.
func (t *AssemblyTool) Call(ctx context.Context, input string) (string, error) {
	if t.inner == nil {
		return "", ErrRuntimeNotInitialized
	}

	if t.client == nil {
		// No governance client means no runtime was reachable at Init.
		// Under the fail-closed enforce posture, deny rather than run the
		// tool unchecked (AAASM-3109). In observe/disabled, pass through so
		// the proxy / eBPF layers remain authoritative.
		if t.shouldDenyOnUnavailable() {
			log.Printf("assembly: %v (tool=%s)", ErrGovernanceUnavailable, t.inner.Name())
			return "", ErrGovernanceUnavailable
		}
		return t.inner.Call(ctx, input)
	}

	ctxWithRunID, runID := EnsureRunID(ctx)
	ctx = ctxWithRunID

	// Consult the live op-control kill switch before the gateway Check
	// (AAASM-3491 / AAASM-3501): a terminated op short-circuits here — the
	// gateway is never even queried — and a paused op blocks cooperatively
	// until the operator resumes it. Skipped when no subscriber is wired or
	// the call carries no trace identity (so there is no tracked op).
	if err := t.runOpControlGate(ctx); err != nil {
		t.recordOutcome(ctx, "", err)
		return "", err
	}

	if err := t.runGovernanceGate(ctx, input, runID); err != nil {
		t.recordOutcome(ctx, "", err)
		return "", err
	}

	result, err := t.inner.Call(ctx, input)
	t.recordOutcome(ctx, result, err)

	return result, err
}

// recordOutcome calls the governance client's RecordResult for one governed
// call. A denied call carries an empty Result and the short-circuit error in
// Error.
//
// It runs on the denied paths as well as the executed one (AAASM-5665).
// Previously Call returned at the gate branches before RecordResult was
// invoked at all, so a deny could not reach an audit sink even in principle.
//
// Where the record then goes is the client's property, not this method's. On
// the production client it crosses the native FFI event channel into the
// runtime's audit pipeline (see [ffiGovernanceClient.RecordResult]), which under
// ADR 0033 §6 is what *Observed* requires — a durable event attributed to the
// action — for the allowed and denied paths alike (AAASM-5750). On a governance
// client that carries no event channel the record still stops here, and
// [AuditSinkDisposition] is what says which of the two a given run is in.
//
// The dispatch is deliberately fire-and-forget and its error deliberately
// discarded: a failing audit sink must not turn a policy deny into a transport
// error, and must not add the runtime's latency to a call the gate already
// decided. The cost is that a record in flight when the process exits can be
// lost — the sink is best-effort, which is why the term above is *Observed* for
// records that arrive rather than a guarantee that every one does.
//
// This is only reachable after [AssemblyTool.Call] has established a non-nil
// client, so it needs no nil guard. The nil-client path does not call
// RecordResult at all, for a different reason: the client *is* the sink, so
// when it is absent there is nothing to call — [AuditSinkAbsent] rather than
// [AuditSinkDiscarded]. Under the enforce posture that path still denies the
// call (see shouldDenyOnUnavailable), and that deny produces no evidence either.
func (t *AssemblyTool) recordOutcome(ctx context.Context, result string, callErr error) {
	recordCtx := context.WithoutCancel(ctx)
	go func() {
		_ = t.client.RecordResult(recordCtx, RecordRequest{
			ToolName: t.inner.Name(),
			TraceID:  TraceIDFromContext(recordCtx),
			RunID:    RunIDFromContext(recordCtx),
			Result:   result,
			Error:    errString(callErr),
		})
	}()
}

// runGovernanceGate runs the pre-execution policy check and returns a non-nil
// error when the call must be short-circuited (policy denial, approval failure,
// or a fail-closed check error). A nil return means the wrapped tool may run.
// A check transport error denies the call under the fail-closed enforce posture
// (the default, AAASM-3108); it is swallowed (allow) only when fail-open was
// opted into or the enforcement posture is observe/disabled.
func (t *AssemblyTool) runGovernanceGate(ctx context.Context, input, runID string) error {
	decision, err := t.client.Check(ctx, CheckRequest{
		ToolName: t.inner.Name(),
		Args:     input,
		AgentID:  AgentIDFromContext(ctx),
		TraceID:  TraceIDFromContext(ctx),
		RunID:    runID,
	})
	if err != nil {
		if t.shouldDenyOnUnavailable() {
			return fmt.Errorf("assembly: governance check failed: %w", err)
		}
		log.Printf("assembly: governance check failed, allowing tool call (fail-open posture): %v (tool=%s)", err, t.inner.Name())
		return nil
	}

	if decision.Denied {
		return t.policyViolation(decision)
	}
	if decision.Pending {
		return t.resolvePending(ctx)
	}
	return nil
}

// runOpControlGate consults the live op-control kill switch (AAASM-3491 /
// AAASM-3501) before the gateway Check and returns a non-nil error when the
// call must be short-circuited. It returns nil — letting the call proceed to
// the normal governance gate — when no subscriber is wired or the call carries
// no resolvable op ID (no trace identity, so no tracked op for the kill switch
// to address). A terminated op returns the subscriber's *OpTerminatedError
// before the gateway is ever queried; a paused op blocks here until the gateway
// resumes (or terminates) it.
func (t *AssemblyTool) runOpControlGate(ctx context.Context) error {
	if t.opControl == nil {
		return nil
	}
	opID := resolveOpID(ctx)
	if opID == "" {
		return nil
	}
	err := t.opControl.WaitForOp(ctx, opID)
	if err == nil {
		return nil
	}
	// A paused op whose control stream died can no longer be resumed by the
	// operator. Treat that as continue-blocking under the fail-closed enforce
	// posture (deny), but let observe/disabled proceed so those postures never
	// short-circuit a tool call (AAASM-4019). A terminate or ctx cancel is not
	// posture-gated — it always short-circuits.
	if errors.Is(err, ErrOpControlUnavailable) {
		if t.shouldDenyOnUnavailable() {
			return fmt.Errorf("assembly: op control: %w", err)
		}
		log.Printf("assembly: op control stream unavailable, allowing tool call (fail-open posture): %v (tool=%s)", err, t.inner.Name())
		return nil
	}
	return fmt.Errorf("assembly: op control: %w", err)
}

// shouldDenyOnUnavailable reports whether a governance check that could not
// produce a decision — a transport error, timeout, or a missing client — must
// deny the tool call. It denies only under the fail-closed posture and an
// enforcing mode: the observe and disabled postures always allow so the gateway
// can shadow-audit (observe) or skip governance entirely (disabled) without the
// SDK short-circuiting the call. The empty enforcement mode means "gateway
// default", which is live enforce, so it denies.
func (t *AssemblyTool) shouldDenyOnUnavailable() bool {
	if !t.opts.failClosed {
		return false
	}
	switch t.opts.enforcementMode {
	case EnforcementModeObserve, EnforcementModeDisabled:
		return false
	default:
		return true
	}
}

// resolvePending blocks on out-of-band approval and maps the resolved decision
// to a short-circuit error, or nil when the call is approved.
func (t *AssemblyTool) resolvePending(ctx context.Context) error {
	decision, err := t.client.WaitForApproval(ctx, ApprovalRequest{
		ToolName: t.inner.Name(),
		TraceID:  TraceIDFromContext(ctx),
		RunID:    RunIDFromContext(ctx),
	})
	if err != nil {
		return fmt.Errorf("assembly: approval wait failed: %w", err)
	}
	if decision.Denied {
		return t.policyViolation(decision)
	}
	return nil
}

func (t *AssemblyTool) policyViolation(decision Decision) error {
	return &PolicyViolationError{ToolName: t.inner.Name(), Reason: decision.Reason}
}

var _ Tool = (*AssemblyTool)(nil)

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
