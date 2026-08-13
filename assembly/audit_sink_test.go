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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ai-agent-assembly/go-sdk/internal/ffi"
)

// auditProbePayload is distinctive enough that finding it anywhere downstream is
// unambiguous.
const auditProbePayload = "AUDIT-PROBE-AAASM-5731"

// governanceClientShape is the method set that makes a type a GovernanceClient.
// Membership is decided by SHAPE, not by name: a shipped client cannot escape
// the gate below by being called something else.
var governanceClientShape = []string{"Check", "WaitForApproval", "RecordResult", "Close"}

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
// It reads the package's own source rather than a hand-maintained list of
// clients, because a list is not a gate: a new shipped client would pass by
// omission, which is exactly how a silently-discarding sink got here. Any type
// declaring the full GovernanceClient method set must also declare AuditSink.
func TestEveryShippedGovernanceClientDeclaresItsAuditSink(t *testing.T) {
	methodsByType := declaredMethodsByType(t)

	var implementations, undeclared []string
	for typeName, methods := range methodsByType {
		if !hasAll(methods, governanceClientShape) {
			continue
		}
		implementations = append(implementations, typeName)
		if !methods["AuditSink"] {
			undeclared = append(undeclared, typeName)
		}
	}
	sort.Strings(implementations)
	sort.Strings(undeclared)

	// Positive control: the scan must actually find the one client this package
	// is known to ship. An empty implementations slice would otherwise make the
	// undeclared check vacuously true.
	if len(implementations) == 0 {
		t.Fatal("found no GovernanceClient implementation in this package's non-test sources; " +
			"the source scan is broken, so its empty result proves nothing")
	}
	if len(undeclared) > 0 {
		t.Fatalf("GovernanceClient implementation(s) %v ship without an AuditSink() declaration; "+
			"RecordResult returning nil is indistinguishable from a retained record, so every "+
			"shipped client must say what it does with it (AAASM-5731). Found implementations: %v",
			undeclared, implementations)
	}
}

// TestShippedClientDeclaringNoRetentionReachesNothing measures the declaration
// against behaviour, end to end through Init -> WrapTools -> Call, on the
// allowed path and the denied one.
func TestShippedClientDeclaringNoRetentionReachesNothing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		decision int32
		reason   string
	}{
		{"allowed", ffi.DecisionAllow, ""},
		{"denied", ffi.DecisionDeny, "blocked by policy"},
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

			// The tool RESULT is the discriminator: only the record path knows
			// it. The check above carries the ARGS, so asserting on the args
			// would collide with the positive control and be unfalsifiable.
			// Both boundary channels are swept, not just the event one — a leak
			// that took the query channel would otherwise go unseen.
			for _, crossing := range boundaryCrossings(crossings) {
				if strings.Contains(crossing, auditProbePayload+"-RESULT") {
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

// declaredMethodsByType parses this package's non-test sources and returns, per
// named type, the set of methods declared on it (value or pointer receiver).
func declaredMethodsByType(t *testing.T) map[string]map[string]bool {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	// Files are walked directly rather than through a package loader so that
	// build-constrained files are scanned too: a client shipped only on one GOOS
	// is still a shipped client, and a loader would silently drop it from the
	// gate on every other platform.
	fileSet := token.NewFileSet()
	methodsByType := map[string]map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
				continue
			}
			typeName := receiverTypeName(funcDecl.Recv.List[0].Type)
			if typeName == "" {
				continue
			}
			if methodsByType[typeName] == nil {
				methodsByType[typeName] = map[string]bool{}
			}
			methodsByType[typeName][funcDecl.Name.Name] = true
		}
	}
	return methodsByType
}

func receiverTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	case *ast.Ident:
		return typed.Name
	default:
		return ""
	}
}

func hasAll(methods map[string]bool, required []string) bool {
	for _, name := range required {
		if !methods[name] {
			return false
		}
	}
	return true
}
