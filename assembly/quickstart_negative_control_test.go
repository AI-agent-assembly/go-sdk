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
