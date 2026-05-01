//go:build integration

package assembly

import (
	"context"
	"sync"
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
