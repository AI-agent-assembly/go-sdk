package assembly

import (
	"context"
	"errors"
	"testing"
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
