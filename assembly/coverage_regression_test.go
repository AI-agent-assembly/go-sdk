package assembly

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAssemblyToolNilInner(t *testing.T) {
	wrapped := NewAssemblyTool(nil, nil, defaultRuntimeOptions())

	if wrapped.Name() != "" {
		t.Fatalf("expected empty name, got %q", wrapped.Name())
	}
	if wrapped.Description() != "" {
		t.Fatalf("expected empty description, got %q", wrapped.Description())
	}

	_, err := wrapped.Call(context.Background(), "input")
	if !errors.Is(err, ErrRuntimeNotInitialized) {
		t.Fatalf("expected ErrRuntimeNotInitialized, got %v", err)
	}
}

func TestAssemblyToolFailClosedOnCheckError(t *testing.T) {
	client := &coverageGovernanceClient{checkErr: errors.New("gateway down")}
	opts := defaultRuntimeOptions()
	opts.failClosed = true

	wrapped := NewAssemblyTool(coverageTool{name: "calculator", result: "42"}, client, opts)
	_, err := wrapped.Call(context.Background(), "6*7")
	if err == nil {
		t.Fatal("expected error when failClosed is true")
	}
	if !strings.Contains(err.Error(), "governance check failed") {
		t.Fatalf("expected governance check failure, got %v", err)
	}
}

func TestAssemblyToolFailOpenOnCheckError(t *testing.T) {
	client := &coverageGovernanceClient{checkErr: errors.New("gateway down")}
	opts := defaultRuntimeOptions()
	opts.failClosed = false

	wrapped := NewAssemblyTool(coverageTool{name: "calculator", result: "42"}, client, opts)
	result, err := wrapped.Call(context.Background(), "6*7")
	if err != nil {
		t.Fatalf("expected fail-open behavior, got error %v", err)
	}
	if result != "42" {
		t.Fatalf("expected tool result 42, got %q", result)
	}
}

func TestAssemblyToolPendingWaitError(t *testing.T) {
	client := &coverageGovernanceClient{
		checkDecision: Decision{Pending: true},
		waitErr:       errors.New("wait timeout"),
	}
	wrapped := NewAssemblyTool(coverageTool{name: "calculator", result: "42"}, client, defaultRuntimeOptions())

	_, err := wrapped.Call(context.Background(), "6*7")
	if err == nil {
		t.Fatal("expected wait error")
	}
	if !strings.Contains(err.Error(), "approval wait failed") {
		t.Fatalf("expected approval wait error, got %v", err)
	}
}

func TestAssemblyToolPendingDenied(t *testing.T) {
	client := &coverageGovernanceClient{
		checkDecision: Decision{Pending: true},
		waitDecision:  Decision{Denied: true, Reason: "requires approval"},
	}
	wrapped := NewAssemblyTool(coverageTool{name: "web_search", result: "unused"}, client, defaultRuntimeOptions())

	_, err := wrapped.Call(context.Background(), "query")
	var violation *PolicyViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("expected PolicyViolationError, got %v", err)
	}
	if violation.Reason != "requires approval" {
		t.Fatalf("expected denial reason to be propagated, got %q", violation.Reason)
	}
}

func TestAssemblyToolRecordResultCapturesErrorString(t *testing.T) {
	recorded := make(chan RecordRequest, 1)
	client := &coverageGovernanceClient{recordRequests: recorded}
	wrapped := NewAssemblyTool(
		coverageTool{name: "calculator", callErr: errors.New("call failed")},
		client,
		defaultRuntimeOptions(),
	)

	_, err := wrapped.Call(context.Background(), "6*7")
	if err == nil {
		t.Fatal("expected call error")
	}

	select {
	case request := <-recorded:
		if request.ToolName != "calculator" {
			t.Fatalf("expected tool name calculator, got %q", request.ToolName)
		}
		if request.Error != "call failed" {
			t.Fatalf("expected recorded error call failed, got %q", request.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected RecordResult to be called")
	}
}

func TestWrapToolsAppliesOptions(t *testing.T) {
	wrapped := WrapTools(
		[]Tool{coverageTool{name: "calculator", result: "42"}},
		nil,
		WithFailClosed(true),
	)

	if len(wrapped) != 1 {
		t.Fatalf("expected 1 wrapped tool, got %d", len(wrapped))
	}

	assemblyTool, ok := wrapped[0].(*AssemblyTool)
	if !ok {
		t.Fatalf("expected *AssemblyTool, got %T", wrapped[0])
	}
	if !assemblyTool.opts.failClosed {
		t.Fatal("expected failClosed option to be applied")
	}
}

func TestNewAssemblyIgnoresNilOption(t *testing.T) {
	runtime := NewAssembly(nil, WithGatewayURL("https://gateway.example.com"), WithAPIKey("key"))

	if runtime.opts.gatewayURL != "https://gateway.example.com" {
		t.Fatalf("expected gateway URL to be set, got %q", runtime.opts.gatewayURL)
	}
	if runtime.opts.apiKey != "key" {
		t.Fatalf("expected API key to be set, got %q", runtime.opts.apiKey)
	}
	if runtime.sidecarConnector == nil {
		t.Fatal("expected sidecar connector to be initialized")
	}
}

func TestPolicyViolationErrorFormatting(t *testing.T) {
	var nilErr *PolicyViolationError
	if got := nilErr.Error(); got != "assembly: policy violation" {
		t.Fatalf("expected nil receiver fallback, got %q", got)
	}

	testCases := []struct {
		err  *PolicyViolationError
		want string
	}{
		{err: &PolicyViolationError{}, want: "assembly: policy violation"},
		{err: &PolicyViolationError{Reason: "blocked"}, want: "assembly: policy violation: blocked"},
		{err: &PolicyViolationError{ToolName: "web_search"}, want: "assembly: policy violation: tool=web_search"},
		{err: &PolicyViolationError{ToolName: "web_search", Reason: "blocked"}, want: "assembly: policy violation: tool=web_search reason=blocked"},
	}

	for _, tc := range testCases {
		if got := tc.err.Error(); got != tc.want {
			t.Fatalf("expected %q, got %q", tc.want, got)
		}
	}
}

func TestErrString(t *testing.T) {
	if got := errString(nil); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}

	if got := errString(errors.New("boom")); got != "boom" {
		t.Fatalf("expected boom, got %q", got)
	}
}

type coverageTool struct {
	name    string
	result  string
	callErr error
}

func (t coverageTool) Name() string {
	return t.name
}

func (coverageTool) Description() string {
	return "desc"
}

func (t coverageTool) Call(context.Context, string) (string, error) {
	return t.result, t.callErr
}

type coverageGovernanceClient struct {
	checkDecision  Decision
	checkErr       error
	waitDecision   Decision
	waitErr        error
	recordRequests chan RecordRequest
}

func (c *coverageGovernanceClient) Check(context.Context, CheckRequest) (Decision, error) {
	return c.checkDecision, c.checkErr
}

func (c *coverageGovernanceClient) WaitForApproval(context.Context, ApprovalRequest) (Decision, error) {
	return c.waitDecision, c.waitErr
}

func (c *coverageGovernanceClient) RecordResult(_ context.Context, request RecordRequest) error {
	if c.recordRequests != nil {
		c.recordRequests <- request
	}
	return nil
}

func (c *coverageGovernanceClient) Close() error {
	return nil
}
