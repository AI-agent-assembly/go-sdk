package assembly

// Enforcement-truth negative controls for the documented Go quick-start
// (AAASM-5529, Epic AAASM-5526).
//
// docs/quick-start.md tells a reader to call assembly.WrapTools(tools, client)
// and states that "a deny decision surfaces as a *assembly.PolicyViolationError
// and the inner tool never runs". Every existing test of that claim asserts it
// with a call counter on a stub tool. A counter proves the wrapper did not
// invoke a func value it holds; it does not prove that the effect the tool
// exists to produce was prevented.
//
// Each control here drives the exported WrapTools API over a tool with a real,
// externally-observable effect and asserts the deny as the absence of that
// effect. The side-effect assertion runs before the error assertion on purpose:
// asserting the error first would short-circuit the test when enforcement is
// removed, leaving the absence check unexercised, and the falsification run
// would then only ever prove "no error was returned".
//
// The Falsification subtests run the identical tool with governance removed
// (calling the original, unwrapped Tool). They must observe the side effect; if
// they stop doing so, every deny assertion here has become vacuous.

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const negativeControlAgentID = "quickstart-negative-control-agent"

func negativeControlContext() context.Context {
	return WithAgentID(context.Background(), negativeControlAgentID)
}

func TestQuickStartFilesystemNegativeControl(t *testing.T) {
	t.Run("PositiveControl_AllowedWriteCreatesTheFile", func(t *testing.T) {
		tool := newFileSideEffectTool(t)
		client := newPolicyGovernanceClient(nil)

		governed := WrapTools([]Tool{tool}, client)
		if _, err := governed[0].Call(negativeControlContext(), "allowed"); err != nil {
			t.Fatalf("allowed call returned an error: %v", err)
		}

		if !tool.occurred() {
			t.Fatal("allowed write did not create the file: the negative control below would be vacuous")
		}
		if got := tool.content(t); got != "allowed" {
			t.Fatalf("file content = %q, want %q", got, "allowed")
		}
	})

	t.Run("NegativeControl_DeniedWriteLeavesNoFile", func(t *testing.T) {
		tool := newFileSideEffectTool(t)
		client := newPolicyGovernanceClient(map[string]string{"write_to_disk": "policy forbids disk writes"})

		governed := WrapTools([]Tool{tool}, client)
		_, err := governed[0].Call(negativeControlContext(), "denied")

		// The load-bearing assertion: the effect the tool exists to produce is
		// absent from the filesystem, not merely that an error was returned.
		if tool.occurred() {
			t.Fatalf("denied write created %s: the tool body ran despite the deny", tool.path)
		}
		if got := tool.content(t); got != "" {
			t.Fatalf("denied write left content %q on disk", got)
		}

		var violation *PolicyViolationError
		if !errors.As(err, &violation) {
			t.Fatalf("err = %v, want *PolicyViolationError", err)
		}
		if violation.ToolName != "write_to_disk" {
			t.Fatalf("violation.ToolName = %q, want %q", violation.ToolName, "write_to_disk")
		}
	})

	t.Run("Falsification_TheSameWriteUngovernedCreatesTheFile", func(t *testing.T) {
		tool := newFileSideEffectTool(t)

		// No WrapTools, no client — enforcement removed. If this does not
		// write, the negative control above is vacuous.
		if _, err := tool.Call(context.Background(), "ungoverned"); err != nil {
			t.Fatalf("ungoverned call returned an error: %v", err)
		}

		if !tool.occurred() {
			t.Fatal("ungoverned write did not create the file")
		}
	})
}

func TestQuickStartNetworkNegativeControl(t *testing.T) {
	t.Run("PositiveControl_AllowedEgressReachesTheListener", func(t *testing.T) {
		tool := newNetworkSideEffectTool(t)
		client := newPolicyGovernanceClient(nil)

		governed := WrapTools([]Tool{tool}, client)
		if _, err := governed[0].Call(negativeControlContext(), "allowed-payload"); err != nil {
			t.Fatalf("allowed call returned an error: %v", err)
		}

		got := tool.requests()
		if len(got) != 1 || got[0] != "allowed-payload" {
			t.Fatalf("listener received %q, want exactly [allowed-payload]", got)
		}
	})

	t.Run("NegativeControl_DeniedEgressNeverReachesTheListener", func(t *testing.T) {
		tool := newNetworkSideEffectTool(t)
		client := newPolicyGovernanceClient(map[string]string{"send_http_request": "egress denied"})

		governed := WrapTools([]Tool{tool}, client)
		_, err := governed[0].Call(negativeControlContext(), "denied-payload")

		// The listener is live and was reachable throughout — the positive
		// control proves that on the same fixture — so zero received requests
		// is evidence the egress did not happen, not that it could not have.
		if tool.occurred() {
			t.Fatalf("listener received %q despite the deny", tool.requests())
		}

		var violation *PolicyViolationError
		if !errors.As(err, &violation) {
			t.Fatalf("err = %v, want *PolicyViolationError", err)
		}
	})

	t.Run("Falsification_TheSameEgressUngovernedReachesTheListener", func(t *testing.T) {
		tool := newNetworkSideEffectTool(t)

		if _, err := tool.Call(context.Background(), "ungoverned-payload"); err != nil {
			t.Fatalf("ungoverned call returned an error: %v", err)
		}

		if !tool.occurred() {
			t.Fatal("ungoverned egress did not reach the listener")
		}
	})
}

func TestQuickStartDenyIsAttributable(t *testing.T) {
	t.Run("TheDenyCarriesTheAgentAndToolIdentity", func(t *testing.T) {
		tool := newFileSideEffectTool(t)
		client := newPolicyGovernanceClient(map[string]string{"write_to_disk": "policy forbids disk writes"})

		governed := WrapTools([]Tool{tool}, client)
		_, err := governed[0].Call(negativeControlContext(), "denied")

		if tool.occurred() {
			t.Fatal("denied write created the file")
		}
		var violation *PolicyViolationError
		if !errors.As(err, &violation) {
			t.Fatalf("err = %v, want *PolicyViolationError", err)
		}

		checks := client.checkRequests()
		if len(checks) != 1 {
			t.Fatalf("got %d policy checks, want 1", len(checks))
		}
		// Identity as the SDK presented it to the policy gateway. An anonymous
		// deny is not usable audit evidence.
		if checks[0].ToolName != "write_to_disk" {
			t.Fatalf("check.ToolName = %q, want %q", checks[0].ToolName, "write_to_disk")
		}
		if checks[0].AgentID != negativeControlAgentID {
			t.Fatalf("check.AgentID = %q, want %q", checks[0].AgentID, negativeControlAgentID)
		}
		if checks[0].RunID == "" {
			t.Fatal("check.RunID is empty: the deny cannot be correlated with the rest of the trace")
		}
		if checks[0].Args != "denied" {
			t.Fatalf("check.Args = %q, want %q", checks[0].Args, "denied")
		}
	})

	t.Run("TheDeniedCallEmitsAnAuditRecordNamingTheToolAndRun", func(t *testing.T) {
		tool := newFileSideEffectTool(t)
		client := newPolicyGovernanceClient(map[string]string{"write_to_disk": "policy forbids disk writes"})

		governed := WrapTools([]Tool{tool}, client)
		_, err := governed[0].Call(negativeControlContext(), "denied")

		if tool.occurred() {
			t.Fatal("denied write created the file")
		}
		var violation *PolicyViolationError
		if !errors.As(err, &violation) {
			t.Fatalf("err = %v, want *PolicyViolationError", err)
		}

		// The load-bearing assertion for AAASM-5665, and the one the subtest
		// above cannot make: the *persisted audit record*, not the returned
		// error and not the policy query. Before this, a deny emitted nothing
		// — an auditor reading the record stream could not tell a denied call
		// from a call that was never attempted.
		client.awaitRecord(t)
		records := client.recordRequests()
		if len(records) != 1 {
			t.Fatalf("got %d audit records for the denied call, want 1", len(records))
		}
		if records[0].ToolName != "write_to_disk" {
			t.Fatalf("record.ToolName = %q, want %q", records[0].ToolName, "write_to_disk")
		}
		if records[0].RunID != client.checkRequests()[0].RunID {
			t.Fatalf("record.RunID = %q does not match the checked run %q",
				records[0].RunID, client.checkRequests()[0].RunID)
		}
		// Distinguishes "denied before execution" from "ran and returned an
		// empty string": the record carries the short-circuit error, and the
		// deny reason inside it, while Result stays empty.
		if !strings.Contains(records[0].Error, "policy forbids disk writes") {
			t.Fatalf("record.Error = %q, want it to carry the deny reason", records[0].Error)
		}
		if records[0].Result != "" {
			t.Fatalf("record.Result = %q, want empty: the tool body never ran", records[0].Result)
		}

		// Deliberately not asserted: agent identity. RecordRequest carries no
		// AgentID field, so the audit record names the tool and the run but
		// not the agent. Claiming otherwise here is exactly the over-claim
		// AAASM-5665 exists to remove; carrying identity onto this path is
		// tracked separately as a wire-contract change.
	})

	t.Run("AnAllowedCallIsAuditedUnderTheSameIdentity", func(t *testing.T) {
		tool := newFileSideEffectTool(t)
		client := newPolicyGovernanceClient(nil)

		governed := WrapTools([]Tool{tool}, client)
		if _, err := governed[0].Call(negativeControlContext(), "allowed"); err != nil {
			t.Fatalf("allowed call returned an error: %v", err)
		}
		client.awaitRecord(t)

		if !tool.occurred() {
			t.Fatal("allowed write did not create the file")
		}
		records := client.recordRequests()
		if len(records) != 1 {
			t.Fatalf("got %d audit records, want 1", len(records))
		}
		if records[0].ToolName != "write_to_disk" {
			t.Fatalf("record.ToolName = %q, want %q", records[0].ToolName, "write_to_disk")
		}
		if records[0].RunID != client.checkRequests()[0].RunID {
			t.Fatalf("record.RunID = %q does not match the checked run %q",
				records[0].RunID, client.checkRequests()[0].RunID)
		}
	})
}

func TestQuickStartDegradedPathCannotLookProtected(t *testing.T) {
	t.Run("NilClientUnderTheDefaultPostureDeniesRatherThanRunning", func(t *testing.T) {
		// docs/quick-start.md's "Putting it together" program passes a nil
		// client — WrapTools(tools, nil) — under the default fail-closed
		// posture. AAASM-5526 forbids a degraded path presenting as protected;
		// this holds it to the same standard as every other control here.
		tool := newFileSideEffectTool(t)

		governed := WrapTools([]Tool{tool}, nil)
		_, err := governed[0].Call(negativeControlContext(), "degraded")

		if tool.occurred() {
			t.Fatalf("no governance client was available, yet %s was written", tool.path)
		}
		if !errors.Is(err, ErrGovernanceUnavailable) {
			t.Fatalf("err = %v, want ErrGovernanceUnavailable", err)
		}
	})

	t.Run("NilClientWithFailOpenRunsTheToolAndSaysSo", func(t *testing.T) {
		// The documented opt-out. It must really pass through — otherwise the
		// control above would be indistinguishable from "the tool never works".
		tool := newFileSideEffectTool(t)

		governed := WrapTools([]Tool{tool}, nil, WithFailClosed(false))
		if _, err := governed[0].Call(negativeControlContext(), "passthrough"); err != nil {
			t.Fatalf("fail-open passthrough returned an error: %v", err)
		}

		if !tool.occurred() {
			t.Fatal("fail-open passthrough did not run the tool")
		}
	})
}

func TestQuickStartWrappingDoesNotDisarmTheOriginalTool(t *testing.T) {
	// docs/core-concepts.md is explicit that the in-process wrapper "is not a
	// trust boundary on its own". WrapTools returns a new slice and leaves the
	// caller holding a live reference to the unwrapped Tool, so the deny that
	// the negative controls above prove is scoped to the governed handle only.
	// Stating that as an executable control keeps the boundary honest.
	tool := newFileSideEffectTool(t)
	client := newPolicyGovernanceClient(map[string]string{"write_to_disk": "policy forbids disk writes"})
	original := []Tool{tool}

	governed := WrapTools(original, client)
	_, err := governed[0].Call(negativeControlContext(), "denied")

	// Absence of the effect first, as in every other control here: asserting
	// the error first would abort this test before the line below whenever
	// enforcement is removed, leaving the load-bearing check unexercised.
	if tool.occurred() {
		t.Fatal("the governed call ran the tool body")
	}
	if err == nil {
		t.Fatal("governed call was expected to be denied")
	}

	// Same policy, same process: calling the retained original bypasses it.
	if _, err := original[0].Call(context.Background(), "bypassed"); err != nil {
		t.Fatalf("direct call on the original tool returned an error: %v", err)
	}
	if !tool.occurred() {
		t.Fatal("the direct call did not run: the bypass claim above is unproven")
	}
	if got := tool.content(t); got != "bypassed" {
		t.Fatalf("file content = %q, want %q", got, "bypassed")
	}
	if len(client.checkRequests()) != 1 {
		t.Fatalf("got %d policy checks, want 1: the direct call must not be gated",
			len(client.checkRequests()))
	}
}
