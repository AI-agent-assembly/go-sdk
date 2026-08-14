package assembly

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// fakeOpControl is a minimal OpController stand-in driving WaitForOp behavior
// without standing up a gRPC stream. A terminated op_id returns an
// *OpTerminatedError; a paused op_id blocks until resume() is called; any other
// op_id returns nil immediately (runnable).
type fakeOpControl struct {
	terminated map[string]bool

	mu      sync.Mutex
	awaited []string
	release chan struct{} // closed by resume(); blocks paused ops until then
	paused  map[string]bool
}

func newFakeOpControl() *fakeOpControl {
	return &fakeOpControl{
		terminated: map[string]bool{},
		paused:     map[string]bool{},
		release:    make(chan struct{}),
	}
}

func (f *fakeOpControl) resume() { close(f.release) }

func (f *fakeOpControl) WaitForOp(ctx context.Context, opID string) error {
	f.mu.Lock()
	f.awaited = append(f.awaited, opID)
	terminated := f.terminated[opID]
	paused := f.paused[opID]
	f.mu.Unlock()

	if terminated {
		return &OpTerminatedError{OpID: opID}
	}
	if paused {
		select {
		case <-f.release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (f *fakeOpControl) awaitedOps() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.awaited...)
}

// checkRecordingClient records whether Check ran so a test can assert the
// op-control gate short-circuited before the gateway was queried.
type checkRecordingClient struct {
	mu       sync.Mutex
	checked  bool
	decision Decision
}

func (c *checkRecordingClient) Check(context.Context, CheckRequest) (Decision, error) {
	c.mu.Lock()
	c.checked = true
	c.mu.Unlock()
	return c.decision, nil
}

func (c *checkRecordingClient) WaitForApproval(context.Context, ApprovalRequest) (Decision, error) {
	return Decision{}, nil
}

func (c *checkRecordingClient) RecordResult(context.Context, RecordRequest) error { return nil }

func (c *checkRecordingClient) Close() error { return nil }

func (c *checkRecordingClient) wasChecked() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.checked
}

func ctxWithTraceSpan(t *testing.T, traceHex, spanHex string) context.Context {
	t.Helper()
	traceID, err := oteltrace.TraceIDFromHex(traceHex)
	if err != nil {
		t.Fatalf("bad trace hex %q: %v", traceHex, err)
	}
	spanID, err := oteltrace.SpanIDFromHex(spanHex)
	if err != nil {
		t.Fatalf("bad span hex %q: %v", spanHex, err)
	}
	spanCtx := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
	})
	return oteltrace.ContextWithSpanContext(context.Background(), spanCtx)
}

func TestTerminatedOpDeniesBeforeGatewayCheck(t *testing.T) {
	t.Parallel()

	opControl := newFakeOpControl()
	opControl.terminated["trace-1:span-1"] = true
	client := &checkRecordingClient{decision: Decision{}}
	opts := defaultRuntimeOptions()
	opts.opControl = opControl
	wrapped := newAssemblyTool(stubTool{name: "web_search", result: "unused"}, client, opts)

	ctx := WithOpID(WithTraceID(context.Background(), "trace-1"), "trace-1:span-1")
	_, err := wrapped.Call(ctx, "query")

	var terminated *OpTerminatedError
	if !errors.As(err, &terminated) {
		t.Fatalf("expected OpTerminatedError, got %v", err)
	}
	if terminated.OpID != "trace-1:span-1" {
		t.Fatalf("expected op id trace-1:span-1, got %q", terminated.OpID)
	}
	if opControl.awaitedOps() == nil {
		t.Fatal("expected op control to be consulted")
	}
	// Short-circuit: a terminated op must halt before the gateway is queried.
	if client.wasChecked() {
		t.Fatal("expected gateway Check to be skipped for a terminated op")
	}
}

// TestTerminatedOpDenyCallsRecordResult pins the op-control branch of the
// AAASM-5665 fix. The governance-deny branch is covered by
// TestQuickStartDenyIsAttributable; without this, deleting the recordOutcome
// call from the op-control branch alone leaves the whole suite green, so half
// the fix would have no regression test.
//
// It asserts the wrapper's CALL to RecordResult, against a test double. That is
// the whole of what it establishes: the double is not the shipped sink, so a
// green here says nothing about whether the record leaves the process. What the
// production client does with it — forward it over the native event channel —
// is measured separately, against the native boundary, in
// TestShippedClientForwardsTheRecordAcrossTheBoundary (AAASM-5750). Keeping the
// two apart is deliberate: a suite that proves retention from a double it
// injected is the defect, not the evidence.
func TestTerminatedOpDenyCallsRecordResult(t *testing.T) {
	t.Parallel()

	opControl := newFakeOpControl()
	opControl.terminated["trace-9:span-9"] = true
	client := newPolicyGovernanceClient(nil)
	opts := defaultRuntimeOptions()
	opts.opControl = opControl
	wrapped := newAssemblyTool(stubTool{name: "web_search", result: "unused"}, client, opts)

	ctx := WithOpID(WithTraceID(context.Background(), "trace-9"), "trace-9:span-9")
	_, err := wrapped.Call(ctx, "query")

	var terminated *OpTerminatedError
	if !errors.As(err, &terminated) {
		t.Fatalf("expected OpTerminatedError, got %v", err)
	}

	client.awaitRecord(t)
	records := client.recordRequests()
	if len(records) != 1 {
		t.Fatalf("got %d records for the terminated-op deny, want 1", len(records))
	}
	if records[0].ToolName != "web_search" {
		t.Fatalf("record.ToolName = %q, want %q", records[0].ToolName, "web_search")
	}
	if !strings.Contains(records[0].Error, "terminated") {
		t.Fatalf("record.Error = %q, want it to carry the terminate reason", records[0].Error)
	}
	if records[0].Result != "" {
		t.Fatalf("record.Result = %q, want empty: the tool body never ran", records[0].Result)
	}
	// The gateway was never queried, so this record is the only thing the
	// wrapper emitted about the call at all.
	if len(client.checkRequests()) != 0 {
		t.Fatalf("got %d policy checks, want 0: a terminated op must short-circuit before the gateway",
			len(client.checkRequests()))
	}
}

func TestPausedOpBlocksThenProceedsOnResume(t *testing.T) {
	t.Parallel()

	opControl := newFakeOpControl()
	opControl.paused["trace-2:span-2"] = true
	client := &checkRecordingClient{decision: Decision{}}
	opts := defaultRuntimeOptions()
	opts.opControl = opControl
	wrapped := newAssemblyTool(stubTool{name: "calculator", result: "42"}, client, opts)

	ctx := WithOpID(WithTraceID(context.Background(), "trace-2"), "trace-2:span-2")

	type outcome struct {
		result string
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := wrapped.Call(ctx, "6*7")
		done <- outcome{result, err}
	}()

	// While paused, the call must not have completed nor reached the gateway.
	select {
	case <-done:
		t.Fatal("expected paused op to block before resume")
	case <-time.After(100 * time.Millisecond):
	}
	if client.wasChecked() {
		t.Fatal("expected gateway Check not to run while op is paused")
	}

	opControl.resume()

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("expected no error after resume, got %v", got.err)
		}
		if got.result != "42" {
			t.Fatalf("expected result 42 after resume, got %q", got.result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resumed op to complete")
	}
	if !client.wasChecked() {
		t.Fatal("expected gateway Check to run after resume")
	}
}

func TestNoTraceIdentitySkipsOpControl(t *testing.T) {
	t.Parallel()

	opControl := newFakeOpControl()
	opControl.terminated["trace-x:span-x"] = true // would deny if consulted
	client := &checkRecordingClient{decision: Decision{}}
	opts := defaultRuntimeOptions()
	opts.opControl = opControl
	wrapped := newAssemblyTool(stubTool{name: "calculator", result: "42"}, client, opts)

	// No trace ID and no explicit op ID — there is no tracked op to address.
	result, err := wrapped.Call(context.Background(), "6*7")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "42" {
		t.Fatalf("expected result 42, got %q", result)
	}
	if got := opControl.awaitedOps(); len(got) != 0 {
		t.Fatalf("expected op control not to be consulted, got %v", got)
	}
	if !client.wasChecked() {
		t.Fatal("expected the call to proceed to the gateway Check")
	}
}

func TestResolveOpIDComposition(t *testing.T) {
	t.Parallel()

	// Explicit op ID wins over trace/span derivation.
	explicit := WithOpID(WithTraceID(context.Background(), "trace"), "explicit-op")
	if got := resolveOpID(explicit); got != "explicit-op" {
		t.Fatalf("expected explicit-op, got %q", got)
	}

	// Composed from the assembly trace ID and the active OTel span ID.
	composed := ctxWithTraceSpan(t, "0102030405060708090a0b0c0d0e0f10", "1112131415161718")
	if got := resolveOpID(composed); got != "0102030405060708090a0b0c0d0e0f10:1112131415161718" {
		t.Fatalf("unexpected composed op id: %q", got)
	}

	// Trace present but no span → trailing colon (mirrors python-sdk #156).
	traceOnly := WithTraceID(context.Background(), "trace-only")
	if got := resolveOpID(traceOnly); got != "trace-only:" {
		t.Fatalf("expected trace-only:, got %q", got)
	}

	// No trace identity at all → empty (op control is skipped).
	if got := resolveOpID(context.Background()); got != "" {
		t.Fatalf("expected empty op id, got %q", got)
	}
}

func TestOpControlNotConsultedWhenUnwired(t *testing.T) {
	t.Parallel()

	client := &checkRecordingClient{decision: Decision{}}
	// No op control on opts — the default wrapper must behave exactly as before.
	wrapped := newAssemblyTool(stubTool{name: "t", result: "ok"}, client, defaultRuntimeOptions())

	ctx := WithTraceID(context.Background(), "trace-3")
	result, err := wrapped.Call(ctx, "i")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected ok, got %q", result)
	}
	if !client.wasChecked() {
		t.Fatal("expected the gateway Check to run when op control is unwired")
	}
}
