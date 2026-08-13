// AAASM-5731 — a shipped GovernanceClient must not drop the hook-layer audit
// record silently.
//
// RecordResult returns only an error, so a client that retains the record and a
// client that throws it away both return nil and are indistinguishable to the
// caller. The one this SDK ships throws it away, and before this suite there was
// no signal at all — not even an opt-in one.
//
// Three things are pinned separately, because any one of them alone passes while
// the defect is present:
//
//  1. every GovernanceClient this package ships *declares* a disposition;
//  2. the declaration matches what the client does — one that says it does not
//     retain must reach nothing, measured against a boundary a positive control
//     proves is reachable;
//  3. Init surfaces it on the DEFAULT path, with nothing opted into.
//
// Assertions are over the real shipped client, reached through the real Init /
// WrapTools path. The ffi binding is not a stand-in for the code under test — it
// is the downstream native boundary, and binding.sendEvent is the SDK's only
// outbound event channel (see internal/ffi/client.go), so "nothing was recorded"
// is decidable there and nowhere else. The original tests missed this defect by
// asserting over mocks of the thing under test; a mock that receives a call
// proves the mock's contract, not the product's.
package assembly

import (
	"context"
	"go/types"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ai-agent-assembly/go-sdk/internal/ffi"
	"golang.org/x/tools/go/packages"
)

// auditProbePayload is distinctive enough that finding it anywhere downstream is
// unambiguous.
const auditProbePayload = "AUDIT-PROBE-AAASM-5731"

type auditProbeTool struct {
	name   string
	result string
	calls  int
}

func (t *auditProbeTool) Name() string        { return t.name }
func (t *auditProbeTool) Description() string { return "audit probe" }
func (t *auditProbeTool) Call(_ context.Context, _ string) (string, error) {
	t.calls++
	return t.result, nil
}

// forwardingGovernanceClient is the OTHER direction of the pin: a client whose
// record path genuinely does cross the native boundary. Without it, "nothing
// crossed" would be consistent with a probe that cannot observe a crossing at
// all, and the test would pass for the wrong reason.
//
// It deliberately does NOT implement AuditSinkDeclarer, so ResolveAuditSink
// reports it as caller-supplied — which is also the point: this SDK declines to
// claim anything about a client it did not build, even one that plainly records.
type forwardingGovernanceClient struct {
	native *ffi.Client
}

func (c *forwardingGovernanceClient) Check(context.Context, CheckRequest) (Decision, error) {
	return Decision{}, nil
}

func (c *forwardingGovernanceClient) WaitForApproval(context.Context, ApprovalRequest) (Decision, error) {
	return Decision{}, nil
}

func (c *forwardingGovernanceClient) RecordResult(_ context.Context, request RecordRequest) error {
	return c.native.SendEvent("tool_result", request.Result)
}

func (c *forwardingGovernanceClient) Close() error { return nil }

// TestEveryShippedGovernanceClientDeclaresItsAuditSink is the exhaustiveness
// gate.
//
// It resolves the module's real type information rather than a hand-maintained
// list of clients, because a list is not a gate: a new shipped client would pass
// by omission, which is exactly how a silently-discarding sink got here.
//
// It uses go/types rather than an AST walk over method declarations. Review of
// #198 demonstrated why that distinction is load-bearing: go/parser sees only the
// methods DECLARED on a named type, never the ones PROMOTED into it by embedding,
// so a `retryingGovernanceClient{ffiGovernanceClient}` wrapper declaring nothing
// but RecordResult is a complete GovernanceClient with no AuditSink anywhere in
// its method set — and the AST version of this gate passed it. types.Implements
// resolves the full method set, promotion included.
//
// It also loads ./... rather than the working directory, so an implementation
// that lands in any package of this module is covered, not only assembly's.
func TestEveryShippedGovernanceClientDeclaresItsAuditSink(t *testing.T) {
	loaded, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedDeps | packages.NeedImports,
		// Tests:false deliberately — the gate covers what this module SHIPS. A
		// probe client living in a _test.go file is not shipped, and including
		// test binaries would make the gate fail on its own fixtures.
		Tests: false,
	}, "./...")
	if err != nil {
		t.Fatalf("load module packages: %v", err)
	}
	if packages.PrintErrors(loaded) > 0 {
		t.Fatal("packages loaded with errors; the gate cannot be trusted on partial type information")
	}

	govIface, declIface := lookupGateInterfaces(t, loaded)

	var implementations, undeclared []string
	for _, pkg := range loaded {
		if pkg.Types == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			named, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || types.IsInterface(named.Type()) {
				// An interface trivially "implements" itself; only concrete
				// clients are shipped.
				continue
			}
			// Method sets differ between T and *T, and a client is almost always
			// used through the pointer. Check both so a value-receiver
			// implementation cannot slip past.
			for _, candidate := range []types.Type{named.Type(), types.NewPointer(named.Type())} {
				if !types.Implements(candidate, govIface) {
					continue
				}
				// types.Type.String() is already fully qualified; prefixing
				// PkgPath again produced a doubled path in the failure message.
				label := types.TypeString(candidate, func(p *types.Package) string {
					return p.Name()
				})
				implementations = append(implementations, label)
				if !types.Implements(candidate, declIface) {
					undeclared = append(undeclared, label)
				}
				break // do not double-count T and *T
			}
		}
	}
	sort.Strings(implementations)
	sort.Strings(undeclared)

	// Positive control: the scan must find the client this module is known to
	// ship. An empty slice would make the undeclared check vacuously true — the
	// same "measured zero from a probe that never ran" the rest of this file
	// guards against.
	if len(implementations) == 0 {
		t.Fatal("found no GovernanceClient implementation anywhere in this module; the type " +
			"scan is broken, so its empty result proves nothing")
	}
	if len(undeclared) > 0 {
		t.Fatalf("GovernanceClient implementation(s) %v ship without an AuditSink() declaration; "+
			"RecordResult returning nil is indistinguishable from a retained record, so every "+
			"shipped client must say what it does with it (AAASM-5731). Found implementations: %v",
			undeclared, implementations)
	}
}

// lookupGateInterfaces resolves the two interfaces the gate compares against from
// the loaded type information, so the gate cannot silently degrade if either is
// renamed or moved.
func lookupGateInterfaces(t *testing.T, loaded []*packages.Package) (govIface, declIface *types.Interface) {
	t.Helper()
	const assemblyPkg = "github.com/ai-agent-assembly/go-sdk/assembly"
	for _, pkg := range loaded {
		if pkg.PkgPath != assemblyPkg || pkg.Types == nil {
			continue
		}
		govIface = asInterface(t, pkg.Types.Scope(), "GovernanceClient")
		declIface = asInterface(t, pkg.Types.Scope(), "AuditSinkDeclarer")
	}
	if govIface == nil || declIface == nil {
		t.Fatalf("could not resolve GovernanceClient / AuditSinkDeclarer in %s", assemblyPkg)
	}
	return govIface, declIface
}

func asInterface(t *testing.T, scope *types.Scope, name string) *types.Interface {
	t.Helper()
	obj := scope.Lookup(name)
	if obj == nil {
		t.Fatalf("type %q not found", name)
	}
	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		t.Fatalf("type %q is not an interface", name)
	}
	return iface
}

// TestShippedClientDeclaringNoRetentionReachesNothing measures the declaration
// against behaviour, end to end through Init -> WrapTools -> Call, on the
// allowed path and the denied one.
func TestShippedClientDeclaringNoRetentionReachesNothing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		decision int32
		reason   string
		// discriminator is the substring only the RECORD path can carry on this
		// branch. It MUST differ per branch: on the denied path recordOutcome is
		// handed result="" and the short-circuit error, so asserting on the tool
		// RESULT there is an assertion that cannot fail — review of #198 measured
		// the leaking mutation turning "allowed" red while "denied" stayed green.
		// The deny reason travels in RecordRequest.Error, so that is the
		// discriminator with real falsifying power on this branch.
		discriminator string
	}{
		{"allowed", ffi.DecisionAllow, "", auditProbePayload + "-RESULT"},
		{"denied", ffi.DecisionDeny, auditProbePayload + "-DENY-REASON", auditProbePayload + "-DENY-REASON"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capClient, crossings := ffi.NewRecordingClient(tc.decision, tc.reason)
			withCapturingFFIClient(t, capClient)

			a, err := Init(context.Background(),
				WithGatewayURL("https://gateway.example.com"),
				WithAPIKey("test-key"),
				withSidecarAddress("127.0.0.1:50051"),
				WithSelfAgentID("agent-5731"),
			)
			if err != nil {
				t.Fatalf("Init: %v", err)
			}

			if got := a.AuditSink(); got != AuditSinkDiscarded {
				t.Fatalf("Assembly.AuditSink() = %q, want %q — the client this SDK ships must "+
					"declare that it drops the record", got, AuditSinkDiscarded)
			}

			inner := &auditProbeTool{name: "web_search", result: auditProbePayload + "-RESULT"}
			wrapped := a.WrapTools([]Tool{inner})
			_, _ = wrapped[0].Call(context.Background(), `{"q":"`+auditProbePayload+`"}`)
			awaitRecordDispatch()

			// Positive control on the SAME boundary. Without it, an empty event
			// list is indistinguishable from a probe that never ran.
			if len(*crossings.Queries) == 0 {
				t.Fatal("no policy query crossed the native boundary; the probe never ran, so " +
					"an empty event list below would prove nothing")
			}
			if !strings.Contains((*crossings.Queries)[0].ArgsJSON, auditProbePayload) {
				t.Fatalf("positive control did not carry the probe payload: %q",
					(*crossings.Queries)[0].ArgsJSON)
			}

			// Assert on tc.discriminator, never on the ARGS: the check above
			// carries the args, so an args assertion would collide with the
			// positive control and be unfalsifiable. Both boundary channels are
			// swept, not just the event one — a leak that took the query channel
			// would otherwise go unseen.
			//
			// The discriminator must be reachable on this branch. Guard it: if
			// the deny reason ever stops reaching RecordRequest.Error, this
			// assertion silently becomes vacuous again, which is precisely the
			// defect being fixed here.
			if !strings.Contains(recordedDiscriminatorSource(tc.name, tc.reason), auditProbePayload) {
				t.Fatalf("the %s branch has no discriminator the record path could carry; "+
					"its leak assertion below cannot fail", tc.name)
			}
			for _, crossing := range boundaryCrossings(crossings) {
				if strings.Contains(crossing, tc.discriminator) {
					t.Errorf("a client declaring %q leaked the audit record across the native "+
						"boundary: %q — the declaration and the behaviour disagree",
						AuditSinkDiscarded, crossing)
				}
			}
		})
	}
}

// TestForwardingClientDoesReachTheBoundary is the control for the direction the
// test above cannot establish on its own: that a record WOULD be visible at this
// boundary if one were made.
func TestForwardingClientDoesReachTheBoundary(t *testing.T) {
	capClient, crossings := ffi.NewRecordingClient(ffi.DecisionAllow, "")
	if err := capClient.Connect("127.0.0.1:50051", "agent-5731", Version); err != nil {
		t.Fatalf("connect: %v", err)
	}

	client := &forwardingGovernanceClient{native: capClient}
	if got := ResolveAuditSink(client); got != AuditSinkCallerSupplied {
		t.Fatalf("ResolveAuditSink(caller's own client) = %q, want %q — this SDK must not claim "+
			"anything about a client it did not build", got, AuditSinkCallerSupplied)
	}

	inner := &auditProbeTool{name: "web_search", result: auditProbePayload + "-RESULT"}
	wrapped := WrapTools([]Tool{inner}, client)
	if _, err := wrapped[0].Call(context.Background(), `{"q":"`+auditProbePayload+`"}`); err != nil {
		t.Fatalf("allowed call: %v", err)
	}
	awaitRecordDispatch()

	var reached bool
	for _, crossing := range boundaryCrossings(crossings) {
		if strings.Contains(crossing, auditProbePayload+"-RESULT") {
			reached = true
		}
	}
	if !reached {
		t.Fatalf("a client whose RecordResult genuinely forwards reached nothing at the native "+
			"boundary (crossings: %v); the probe cannot observe a record, so every 'reached "+
			"nothing' assertion elsewhere in this file is unfalsifiable",
			boundaryCrossings(crossings))
	}
}

// recordedDiscriminatorSource returns the field of RecordRequest that carries
// this branch's discriminator, so the test can prove the discriminator is
// reachable rather than assume it. On the allowed path that is the tool result;
// on the denied path the tool never runs and the result is empty, so it is the
// short-circuit error text (AAASM-5731).
func recordedDiscriminatorSource(branch, reason string) string {
	if branch == "denied" {
		return (&PolicyViolationError{ToolName: "web_search", Reason: reason}).Error()
	}
	return auditProbePayload + "-RESULT"
}

// boundaryCrossings flattens every channel of the native boundary into strings,
// so a leak is caught whichever one it takes.
func boundaryCrossings(crossings ffi.NativeCrossings) []string {
	flattened := append([]string{}, *crossings.Events...)
	for _, query := range *crossings.Queries {
		flattened = append(flattened, query.ArgsJSON+"|"+query.ToolName+"|"+query.ActionType)
	}
	return flattened
}

// TestResolveAuditSinkOnNoRuntimeIsAbsentNotUnknown pins the distinction the
// warning depends on: with no client there is no sink to hand the record to, so
// nothing is even attempted — a different failure from a sink that drops it.
func TestResolveAuditSinkOnNoRuntimeIsAbsentNotUnknown(t *testing.T) {
	if got := ResolveAuditSink(nil); got != AuditSinkAbsent {
		t.Fatalf("ResolveAuditSink(nil) = %q, want %q", got, AuditSinkAbsent)
	}
}

// TestInitWarnsAboutTheAuditGapOnTheDefaultPath pins the signal itself: it must
// arrive with nothing opted into, because a caller who has to already suspect
// the problem in order to discover it has not been told.
func TestInitWarnsAboutTheAuditGapOnTheDefaultPath(t *testing.T) {
	capClient, _ := ffi.NewRecordingClient(ffi.DecisionAllow, "")
	withCapturingFFIClient(t, capClient)

	logged := captureLog(t)

	a, err := Init(context.Background(),
		WithGatewayURL("https://gateway.example.com"),
		WithAPIKey("test-key"),
		withSidecarAddress("127.0.0.1:50051"),
		WithSelfAgentID("agent-5731"),
	)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if a.AuditSink() != AuditSinkDiscarded {
		t.Fatalf("precondition: AuditSink() = %q", a.AuditSink())
	}

	text := logged.String()
	for _, want := range []string{"audit", "NOT retained", string(AuditSinkDiscarded), "AAASM-5731"} {
		if !strings.Contains(text, want) {
			t.Errorf("Init did not warn about the audit gap on the default path: %q missing from %q",
				want, text)
		}
	}
}

// awaitRecordDispatch gives recordOutcome's goroutine room to cross the boundary
// if it ever would. Sleeping is the honest instrument here: the production path
// dispatches the record asynchronously and offers no completion signal, so the
// test cannot synchronise on one without changing the thing it measures.
func awaitRecordDispatch() {
	time.Sleep(250 * time.Millisecond)
}
