//go:build integration

package assembly

import (
	"context"
	"sync"
	"testing"
	"time"
)

// recordingGovernanceClient captures all governance calls for assertion.
type recordingGovernanceClient struct {
	mu            sync.Mutex
	checkCalled   bool
	checkRequest  CheckRequest
	recordCalled  bool
	recordRequest RecordRequest
	recordDone    chan struct{}
	checkDecision Decision
}

func newRecordingGovernanceClient() *recordingGovernanceClient {
	return &recordingGovernanceClient{
		recordDone:    make(chan struct{}, 1),
		checkDecision: Decision{Denied: false},
	}
}

func (r *recordingGovernanceClient) Check(_ context.Context, req CheckRequest) (Decision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checkCalled = true
	r.checkRequest = req
	return r.checkDecision, nil
}

func (r *recordingGovernanceClient) WaitForApproval(_ context.Context, _ ApprovalRequest) (Decision, error) {
	return Decision{}, nil
}

func (r *recordingGovernanceClient) RecordResult(_ context.Context, req RecordRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recordCalled = true
	r.recordRequest = req
	select {
	case r.recordDone <- struct{}{}:
	default:
	}
	return nil
}

func (r *recordingGovernanceClient) Close() error {
	return nil
}

func TestEndToEnd_AgentToolCallEventCapture(t *testing.T) {
	// 1. Init assembly (fallback UDS bridge succeeds for any address)
	a, err := Init(context.Background(),
		WithGatewayURL("https://gateway.test.local"),
		WithAPIKey("integration-test-key"),
		withSidecarAddress("unix:///tmp/aa-integration-test.sock"),
	)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 2. Create mock governance client
	govClient := newRecordingGovernanceClient()

	// 3. Create dummy tool and wrap with WrapTools
	dummy := stubTool{
		name:        "calculator",
		description: "returns the answer",
		result:      "42",
	}
	wrapped := WrapTools([]Tool{dummy}, govClient)
	if len(wrapped) != 1 {
		t.Fatalf("expected 1 wrapped tool, got %d", len(wrapped))
	}

	// 4. Invoke the wrapped tool with context metadata
	ctx := WithAgentID(context.Background(), "test-agent")
	ctx = WithTraceID(ctx, "trace-integration-001")

	result, err := wrapped[0].Call(ctx, `{"expression":"6*7"}`)
	if err != nil {
		t.Fatalf("tool call failed: %v", err)
	}
	if result != "42" {
		t.Fatalf("expected result %q, got %q", "42", result)
	}

	// 5. Assert: governance Check() was called with valid payload
	govClient.mu.Lock()
	if !govClient.checkCalled {
		t.Fatal("expected governance Check to be called")
	}
	if govClient.checkRequest.ToolName != "calculator" {
		t.Fatalf("expected check tool name %q, got %q", "calculator", govClient.checkRequest.ToolName)
	}
	if govClient.checkRequest.Args != `{"expression":"6*7"}` {
		t.Fatalf("expected check args %q, got %q", `{"expression":"6*7"}`, govClient.checkRequest.Args)
	}
	if govClient.checkRequest.AgentID != "test-agent" {
		t.Fatalf("expected check agent id %q, got %q", "test-agent", govClient.checkRequest.AgentID)
	}
	if govClient.checkRequest.TraceID != "trace-integration-001" {
		t.Fatalf("expected check trace id %q, got %q", "trace-integration-001", govClient.checkRequest.TraceID)
	}
	if govClient.checkRequest.RunID == "" {
		t.Fatal("expected non-empty run id on check request")
	}
	govClient.mu.Unlock()

	// 6. Assert: RecordResult was called (fires in background goroutine)
	select {
	case <-govClient.recordDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RecordResult call")
	}

	govClient.mu.Lock()
	if !govClient.recordCalled {
		t.Fatal("expected RecordResult to be called")
	}
	if govClient.recordRequest.ToolName != "calculator" {
		t.Fatalf("expected record tool name %q, got %q", "calculator", govClient.recordRequest.ToolName)
	}
	if govClient.recordRequest.Result != "42" {
		t.Fatalf("expected record result %q, got %q", "42", govClient.recordRequest.Result)
	}
	if govClient.recordRequest.Error != "" {
		t.Fatalf("expected empty record error, got %q", govClient.recordRequest.Error)
	}
	if govClient.recordRequest.TraceID != "trace-integration-001" {
		t.Fatalf("expected record trace id %q, got %q", "trace-integration-001", govClient.recordRequest.TraceID)
	}
	if govClient.recordRequest.RunID == "" {
		t.Fatal("expected non-empty run id on record request")
	}
	govClient.mu.Unlock()

	// 7. Close assembly
	if err := a.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
