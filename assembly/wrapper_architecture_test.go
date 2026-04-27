package assembly

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWrapToolsPreservesLength(t *testing.T) {
	t.Parallel()

	tools := []Tool{
		stubTool{name: "first", description: "one", result: "ok"},
		stubTool{name: "second", description: "two", result: "ok"},
	}

	wrapped := WrapTools(tools, nil)
	if len(wrapped) != len(tools) {
		t.Fatalf("expected wrapped len %d, got %d", len(tools), len(wrapped))
	}
	if _, ok := wrapped[0].(*AssemblyTool); !ok {
		t.Fatal("expected wrapped tools to use AssemblyTool")
	}
}

func TestAssemblyToolPassthrough(t *testing.T) {
	t.Parallel()

	inner := stubTool{name: "calculator", description: "basic calculator", result: "42"}
	wrapped := NewAssemblyTool(inner, nil, defaultRuntimeOptions())

	if wrapped.Name() != inner.name {
		t.Fatalf("expected name %q, got %q", inner.name, wrapped.Name())
	}
	if wrapped.Description() != inner.description {
		t.Fatalf("expected description %q, got %q", inner.description, wrapped.Description())
	}

	result, err := wrapped.Call(context.Background(), "6*7")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "42" {
		t.Fatalf("expected result %q, got %q", "42", result)
	}
}

func TestAssemblyToolDenyDecision(t *testing.T) {
	t.Parallel()

	client := &stubGovernanceClient{
		checkDecision: Decision{Denied: true, Reason: "blocked"},
	}
	wrapped := NewAssemblyTool(stubTool{name: "web_search", result: "unused"}, client, defaultRuntimeOptions())

	_, err := wrapped.Call(context.Background(), "query")
	var violation *PolicyViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("expected PolicyViolationError, got %v", err)
	}
	if violation.ToolName != "web_search" {
		t.Fatalf("expected tool name web_search, got %q", violation.ToolName)
	}
}

func TestAssemblyToolPendingDecision(t *testing.T) {
	t.Parallel()

	client := &stubGovernanceClient{
		checkDecision: Decision{Pending: true},
		waitDecision:  Decision{Denied: false},
	}
	wrapped := NewAssemblyTool(stubTool{name: "calculator", result: "42"}, client, defaultRuntimeOptions())

	result, err := wrapped.Call(context.Background(), "6*7")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "42" {
		t.Fatalf("expected result %q, got %q", "42", result)
	}
}

func TestAssemblyToolPropagatesContextMetadataToGovernanceRequests(t *testing.T) {
	client := &metadataCaptureGovernanceClient{
		checkDecision: Decision{Pending: true},
		waitDecision:  Decision{},
		recordDone:    make(chan struct{}, 1),
	}

	wrapped := NewAssemblyTool(stubTool{name: "calculator", result: "42"}, client, defaultRuntimeOptions())
	ctx := WithAgentID(context.Background(), "agent-1")
	ctx = WithTraceID(ctx, "trace-1")

	result, err := wrapped.Call(ctx, `{"x":1}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "42" {
		t.Fatalf("expected result 42, got %q", result)
	}

	select {
	case <-client.recordDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for record result call")
	}

	if client.checkRequest.AgentID != "agent-1" {
		t.Fatalf("expected check request agent id agent-1, got %q", client.checkRequest.AgentID)
	}
	if client.checkRequest.TraceID != "trace-1" {
		t.Fatalf("expected check request trace id trace-1, got %q", client.checkRequest.TraceID)
	}
	if client.checkRequest.RunID == "" {
		t.Fatal("expected non-empty run id on check request")
	}
	if client.approvalRequest.RunID != client.checkRequest.RunID {
		t.Fatalf("expected approval run id %q, got %q", client.checkRequest.RunID, client.approvalRequest.RunID)
	}
	if client.recordRequest.RunID != client.checkRequest.RunID {
		t.Fatalf("expected record run id %q, got %q", client.checkRequest.RunID, client.recordRequest.RunID)
	}
	if client.recordRequest.TraceID != "trace-1" {
		t.Fatalf("expected record trace id trace-1, got %q", client.recordRequest.TraceID)
	}
}

type stubGovernanceClient struct {
	checkDecision Decision
	checkErr      error
	waitDecision  Decision
	waitErr       error
}

func (s *stubGovernanceClient) Check(context.Context, CheckRequest) (Decision, error) {
	return s.checkDecision, s.checkErr
}

func (s *stubGovernanceClient) WaitForApproval(context.Context, ApprovalRequest) (Decision, error) {
	return s.waitDecision, s.waitErr
}

func (s *stubGovernanceClient) RecordResult(context.Context, RecordRequest) error {
	return nil
}

func (s *stubGovernanceClient) Close() error {
	return nil
}

type metadataCaptureGovernanceClient struct {
	checkDecision   Decision
	waitDecision    Decision
	checkRequest    CheckRequest
	approvalRequest ApprovalRequest
	recordRequest   RecordRequest
	recordDone      chan struct{}
}

func (m *metadataCaptureGovernanceClient) Check(_ context.Context, request CheckRequest) (Decision, error) {
	m.checkRequest = request
	return m.checkDecision, nil
}

func (m *metadataCaptureGovernanceClient) WaitForApproval(_ context.Context, request ApprovalRequest) (Decision, error) {
	m.approvalRequest = request
	return m.waitDecision, nil
}

func (m *metadataCaptureGovernanceClient) RecordResult(_ context.Context, request RecordRequest) error {
	m.recordRequest = request
	select {
	case m.recordDone <- struct{}{}:
	default:
	}
	return nil
}

func (m *metadataCaptureGovernanceClient) Close() error {
	return nil
}

type stubTool struct {
	name        string
	description string
	result      string
	err         error
}

func (t stubTool) Name() string {
	return t.name
}

func (t stubTool) Description() string {
	return t.description
}

func (t stubTool) Call(context.Context, string) (string, error) {
	return t.result, t.err
}
