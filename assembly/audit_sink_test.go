// AAASM-5731 — a shipped GovernanceClient must not drop the hook-layer audit
// record silently.
//
// RecordResult returns only an error, so a client that retains the record and a
// client that throws it away both return nil and are indistinguishable to the
// caller. The one this SDK ships used to throw it away and now forwards it to the
// runtime; before this suite there was no signal of either — not even an opt-in
// one.
//
// Three things are pinned separately, because any one of them alone passes while
// the defect is present:
//
//  1. every GovernanceClient this package ships *declares* a disposition;
//  2. the declaration matches what the client does, in BOTH directions — one
//     that says it forwards must reach the boundary with the record, one that
//     says it does not retain must reach nothing, each measured against a
//     boundary a positive control proves is reachable;
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
// The load pattern is the MODULE path, not "./...". go test sets the working
// directory to the package under test, so "./..." expanded from assembly/ and
// loaded 1 of this module's 6 packages — review of #198 measured a full
// GovernanceClient in examples/minimal passing uncaught. The comment that used
// to sit here claimed module-wide coverage the pattern did not deliver: the
// rewrite changed the mechanism and left the scope. Naming it because it is a
// fourth instance of the pattern this PR names three times — a comment
// describing a mechanism that does not carry what it claims — and this one
// appeared inside the fix for the first three.
func TestEveryShippedGovernanceClientDeclaresItsAuditSink(t *testing.T) {
	loaded, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedDeps | packages.NeedImports,
		// Tests:false deliberately — the gate covers what this module SHIPS. A
		// probe client living in a _test.go file is not shipped, and including
		// test binaries would make the gate fail on its own fixtures.
		Tests: false,
	}, "github.com/ai-agent-assembly/go-sdk/...")
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

// TestShippedClientForwardsTheRecordAcrossTheBoundary measures the declaration
// against behaviour, end to end through Init -> WrapTools -> Call, on the
// allowed path and the denied one (AAASM-5750).
//
// It is the inversion of the assertion this file shipped with. Until the record
// path was wired, the shipped client declared AuditSinkDiscarded and this test
// asserted that nothing crossed; now it declares AuditSinkForwarded and the same
// measurement, over the same boundary and the same discriminator, has to find
// the record on the far side. The structure is unchanged on purpose — the
// positive control and the observed discriminator are what make either direction
// falsifiable, and swapping the expected outcome must not quietly cost either.
//
// Nothing here injects a sink. The client under measurement is the one Init
// resolves, which is precisely the failure AAASM-5749 found one row over: a test
// that supplies its own recording client proves that client records, and stays
// green over a shipped client that is wired to nothing. Reverting
// ffiGovernanceClient.RecordResult to `return nil` turns both subtests red.
func TestShippedClientForwardsTheRecordAcrossTheBoundary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		decision int32
		reason   string
		// The discriminator is OBSERVED, not declared: recordDecision and
		// recordField say which decision to replay through the wrapper and which
		// RecordRequest field to read the answer out of.
		//
		// It MUST differ per branch. On the denied path recordOutcome is handed
		// result="", so asserting on the tool RESULT there is an assertion that
		// cannot fail — review of #198 measured the leaking mutation turning
		// "allowed" red while "denied" stayed green. The deny reason travels in
		// RecordRequest.Error, so that is the discriminator with real falsifying
		// power on that branch.
		recordDecision Decision
		recordField    string
	}{
		{"allowed", ffi.DecisionAllow, "", Decision{}, "Result"},
		{
			"denied", ffi.DecisionDeny, auditProbePayload + "-DENY-REASON",
			Decision{Denied: true, Reason: auditProbePayload + "-DENY-REASON"}, "Error",
		},
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

			if got := a.AuditSink(); got != AuditSinkForwarded {
				t.Fatalf("Assembly.AuditSink() = %q, want %q — the client this SDK ships must "+
					"declare that it forwards the record", got, AuditSinkForwarded)
			}

			inner := &auditProbeTool{name: "web_search", result: auditProbePayload + "-RESULT"}
			wrapped := a.WrapTools([]Tool{inner})
			_, _ = wrapped[0].Call(context.Background(), `{"q":"`+auditProbePayload+`"}`)
			awaitRecordDispatch()

			// Positive control on the SAME boundary. Without it, an empty event
			// list is indistinguishable from a probe that never ran.
			if len(crossings.Queries()) == 0 {
				t.Fatal("no policy query crossed the native boundary; the probe never ran, so " +
					"an empty event list below would prove nothing")
			}
			if !strings.Contains(crossings.Queries()[0].ArgsJSON, auditProbePayload) {
				t.Fatalf("positive control did not carry the probe payload: %q",
					crossings.Queries()[0].ArgsJSON)
			}

			// Assert on tc.discriminator, never on the ARGS: the policy check
			// above already carries the args across this same boundary, so an
			// args assertion would be satisfied by the positive control alone
			// and would pass with the record path deleted.
			//
			// The discriminator must be reachable on this branch, and
			// reachability is MEASURED against the production wrapper rather
			// than assumed. If the probe payload ever stops reaching this
			// RecordRequest field — because the deny short-circuits with a
			// generic error, or because the error text drops the reason — the
			// guard fires here instead of the assertion below quietly becoming
			// unfalsifiable.
			discriminator := observedRecordField(t, tc.recordDecision, tc.recordField)
			if !strings.Contains(discriminator, auditProbePayload) {
				t.Fatalf("the %s branch's RecordRequest.%s is %q, which does not carry the probe "+
					"payload; nothing the record path emits on this branch is distinguishable, so "+
					"the assertion below cannot fail", tc.name, tc.recordField, discriminator)
			}

			// The EVENT channel specifically, not every channel: the query
			// channel is the positive control and already contains the probe.
			// Searching both would let the control satisfy the assertion.
			var carried string
			for _, event := range crossings.Events() {
				if strings.Contains(event, discriminator) {
					carried = event
				}
			}
			if carried == "" {
				t.Fatalf("a client declaring %q sent no audit event carrying the %s branch's "+
					"record across the native boundary; events seen: %v — the declaration and "+
					"the behaviour disagree", AuditSinkForwarded, tc.name, crossings.Events())
			}
			// The event must be the tool-result one, not the boot register event
			// that also rides this channel — otherwise a register payload that
			// happened to echo the discriminator would satisfy the assertion.
			if !strings.Contains(carried, `"event_type":"`+eventTypeToolResult+`"`) {
				t.Errorf("the record crossed as %q, which is not a %q event; the audit record and "+
					"the boot register event must not be conflated", carried, eventTypeToolResult)
			}
		})
	}
}

// TestAGovernanceClientWithNoEventChannelDeclaresDiscarded is the other half of
// the declaration/behaviour bind: the disposition is COMPUTED, so it has to be
// shown moving.
//
// Without this, AuditSink() could return the forwarded literal unconditionally
// and every assertion above would still pass — the shipped path always has the
// channel. A querier that carries no event channel is the input that makes the
// two branches disagree, so it is the input that proves the branch exists.
func TestAGovernanceClientWithNoEventChannelDeclaresDiscarded(t *testing.T) {
	t.Parallel()

	client := newFFIGovernanceClient(&fakeQuerier{})
	if got := client.AuditSink(); got != AuditSinkDiscarded {
		t.Fatalf("AuditSink() over a querier with no event channel = %q, want %q", got, AuditSinkDiscarded)
	}
	if err := client.RecordResult(context.Background(), RecordRequest{ToolName: "web_search"}); err != nil {
		t.Fatalf("RecordResult with no event channel returned %v, want nil — a missing sink is "+
			"declared, not raised", err)
	}

	// The control: the same constructor over a querier that DOES carry the
	// channel resolves the other way. Two literals compared against each other
	// would be a tautology; this compares two constructions of the same code.
	capClient, _ := ffi.NewRecordingClient(ffi.DecisionAllow, "")
	if got := newFFIGovernanceClient(capClient).AuditSink(); got != AuditSinkForwarded {
		t.Fatalf("AuditSink() over the connected FFI client = %q, want %q", got, AuditSinkForwarded)
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

// observedRecordField runs one governed call through the PRODUCTION wrapper
// against a capturing client and returns what recordOutcome actually put in the
// requested field of RecordRequest.
//
// It observes rather than reconstructs, and that is the whole point. The
// previous version built a PolicyViolationError itself and compared against
// that, so it could only catch a change to PolicyViolationError.Error() — not
// the failure mode its own comment promised to catch. Review of #198 measured
// the gap: make the deny short-circuit hand recordOutcome a GENERIC error and
// the reason never reaches RecordRequest.Error, yet the guard stayed silent and
// the denied branch went back to asserting nothing. A control that does not
// move with the step under test is the same defect as the vacuous assertion it
// replaced, one level up.
//
// The capturing client is the downstream boundary here, not a stand-in for the
// code under test: what is being observed is the WRAPPER's output, which is the
// same whichever client receives it. The production client cannot be used to
// observe it for a different reason than it once could not — it no longer
// discards, but it emits to the native boundary rather than exposing the
// RecordRequest struct, and the struct is what this helper needs.
func observedRecordField(t *testing.T, decision Decision, field string) string {
	t.Helper()

	records := make(chan RecordRequest, 1)
	client := &coverageGovernanceClient{checkDecision: decision, recordRequests: records}
	inner := &auditProbeTool{name: "web_search", result: auditProbePayload + "-RESULT"}
	wrapped := WrapTools([]Tool{inner}, client)
	_, _ = wrapped[0].Call(context.Background(), `{"q":"`+auditProbePayload+`"}`)

	select {
	case request := <-records:
		if field == "Error" {
			return request.Error
		}
		return request.Result
	case <-time.After(2 * time.Second):
		t.Fatalf("the wrapper never called RecordResult for a %+v decision; the branch has no "+
			"observable record at all, so its discriminator cannot be derived", decision)
		return ""
	}
}

// boundaryCrossings flattens every channel of the native boundary into strings,
// so a leak is caught whichever one it takes.
func boundaryCrossings(crossings ffi.NativeCrossings) []string {
	flattened := append([]string{}, crossings.Events()...)
	for _, query := range crossings.Queries() {
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

// TestInitWarnsAboutTheAuditGapOnlyWhenThereIsOne pins the signal in both
// directions (AAASM-5750).
//
// The warning previously fired for every disposition that was not the caller's
// own, which was correct while both remaining values meant "no evidence". With a
// forwarding path it is no longer correct: a warning on a run whose records DO
// reach the runtime is a false alarm, and a caller who is warned every time
// stops reading the warning — which costs the absent case its signal too. So the
// two are asserted together: the same Init, run over a connected runtime and
// over none, must differ here.
func TestInitWarnsAboutTheAuditGapOnlyWhenThereIsOne(t *testing.T) {
	t.Run("silent when the record is forwarded", func(t *testing.T) {
		capClient, _ := ffi.NewRecordingClient(ffi.DecisionAllow, "")
		withCapturingFFIClient(t, capClient)
		logged := captureLog(t)

		a, err := Init(context.Background(),
			WithGatewayURL("https://gateway.example.com"),
			WithAPIKey("test-key"),
			withSidecarAddress("127.0.0.1:50051"),
			WithSelfAgentID("agent-5750"),
		)
		if err != nil {
			t.Fatalf("Init: %v", err)
		}
		if got := a.AuditSink(); got != AuditSinkForwarded {
			t.Fatalf("precondition: AuditSink() = %q, want %q", got, AuditSinkForwarded)
		}
		if text := logged.String(); strings.Contains(text, "NOT retained") {
			t.Errorf("Init warned that records are NOT retained on a run that forwards them: %q", text)
		}
	})

	t.Run("warns when no sink was resolved", func(t *testing.T) {
		// No FFI client at all, so boot takes the sidecar fallback and leaves
		// governance nil — AuditSinkAbsent. This is the arm that proves the
		// silence above is a decision and not a deleted warning.
		withCapturingFFIClient(t, nil)
		originalConnector := sidecarConnector
		t.Cleanup(func() { sidecarConnector = originalConnector })
		sidecarConnector = func(context.Context, string) (SidecarClient, error) {
			return stubSidecarClient{}, nil
		}
		logged := captureLog(t)

		a, err := Init(context.Background(),
			WithGatewayURL("https://gateway.example.com"),
			WithAPIKey("test-key"),
			WithSelfAgentID("agent-5750"),
			withSidecarAddress("127.0.0.1:50051"),
		)
		if err != nil {
			t.Fatalf("Init: %v", err)
		}
		if got := a.AuditSink(); got != AuditSinkAbsent {
			t.Fatalf("precondition: AuditSink() = %q, want %q", got, AuditSinkAbsent)
		}

		text := logged.String()
		for _, want := range []string{"audit", "NOT retained", string(AuditSinkAbsent), "AAASM-5731"} {
			if !strings.Contains(text, want) {
				t.Errorf("Init did not warn about the audit gap when no sink was resolved: "+
					"%q missing from %q", want, text)
			}
		}
	})
}

// TestARecordDispatchedThenClosedIsDeterministicallyLost measures the shutdown
// race, because the quick-start documents a caveat and a documented caveat with
// no control is just prose (AAASM-5750).
//
// recordOutcome dispatches on a goroutine nothing joins, and Assembly.Close
// tears the native handle down without waiting for it. So the `defer a.Close()`
// the quick-start itself recommends races every dispatch — and loses. This is
// not a flaky-timing test in the usual sense: the loss is the reliable outcome,
// and the assertion is written that way rather than as a tolerance, so if a
// future flush makes records survive this goes red and the caveat gets removed
// rather than quietly becoming false.
func TestARecordDispatchedThenClosedIsDeterministicallyLost(t *testing.T) {
	const runs = 50

	lost := 0
	for range runs {
		capClient, crossings := ffi.NewRecordingClient(ffi.DecisionAllow, "")
		withCapturingFFIClient(t, capClient)

		a, err := Init(context.Background(),
			WithGatewayURL("https://gateway.example.com"),
			WithAPIKey("test-key"),
			withSidecarAddress("127.0.0.1:50051"),
			WithSelfAgentID("agent-5750"),
		)
		if err != nil {
			t.Fatalf("Init: %v", err)
		}

		inner := &auditProbeTool{name: "web_search", result: auditProbePayload + "-RESULT"}
		wrapped := a.WrapTools([]Tool{inner})
		_, _ = wrapped[0].Call(context.Background(), `{"q":"x"}`)
		// Exactly the sequence the quick-start's `defer a.Close()` produces.
		_ = a.Close()

		found := false
		for _, event := range crossings.Events() {
			if strings.Contains(event, auditProbePayload) {
				found = true
			}
		}
		if !found {
			lost++
		}
	}

	// A positive control on the same harness: with the dispatch given room, the
	// record does cross. Without it, "lost" is indistinguishable from a probe
	// that could never observe a record at all.
	capClient, crossings := ffi.NewRecordingClient(ffi.DecisionAllow, "")
	withCapturingFFIClient(t, capClient)
	a, err := Init(context.Background(),
		WithGatewayURL("https://gateway.example.com"),
		WithAPIKey("test-key"),
		withSidecarAddress("127.0.0.1:50051"),
		WithSelfAgentID("agent-5750"),
	)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	wrapped := a.WrapTools([]Tool{&auditProbeTool{name: "web_search", result: auditProbePayload + "-RESULT"}})
	_, _ = wrapped[0].Call(context.Background(), `{"q":"x"}`)
	awaitRecordDispatch()

	survived := false
	for _, event := range crossings.Events() {
		if strings.Contains(event, auditProbePayload) {
			survived = true
		}
	}
	if !survived {
		t.Fatal("the control did not cross either; the probe cannot observe a record, so the " +
			"loss measured above proves nothing")
	}

	if lost != runs {
		t.Errorf("%d of %d records survived Call-then-Close; the quick-start's caveat says the "+
			"record is lost there. If a flush was added, remove the caveat and this test — do not "+
			"loosen the assertion", runs-lost, runs)
	}
}

// awaitRecordDispatch gives recordOutcome's goroutine room to cross the boundary
// if it ever would. Sleeping is the honest instrument here: the production path
// dispatches the record asynchronously and offers no completion signal, so the
// test cannot synchronise on one without changing the thing it measures.
func awaitRecordDispatch() {
	time.Sleep(250 * time.Millisecond)
}
