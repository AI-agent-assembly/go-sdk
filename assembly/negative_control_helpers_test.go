package assembly

// Reusable enforcement-truth negative-control fixture (AAASM-5529).
//
// A test that only asserts errors.As(err, &PolicyViolationError{}) proves the
// SDK produced a refusal, not that the refusal prevented anything; a test that
// asserts inner.calls == 0 proves the wrapper did not invoke a func value it
// holds. Neither shows that the effect the tool exists to produce did not
// happen. These helpers give a denied tool a real, externally-observable
// effect — a file on disk, an HTTP request delivered to a live httptest
// listener — so a deny can be asserted as the absence of that effect and the
// matching allow as its presence.
//
// Every control built on this fixture is used as a pair: without the positive
// control, "no file on disk" is equally well explained by "the tool never ran
// at all", and the negative control proves nothing.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fileSideEffectTool is a Tool whose Call really writes a file. Nothing is
// mocked, so an assertion over occurred() is an assertion about the filesystem
// rather than about the SDK's own bookkeeping.
type fileSideEffectTool struct {
	path string
}

func newFileSideEffectTool(t *testing.T) *fileSideEffectTool {
	t.Helper()
	return &fileSideEffectTool{path: filepath.Join(t.TempDir(), "denied-write.txt")}
}

func (f *fileSideEffectTool) Name() string        { return "write_to_disk" }
func (f *fileSideEffectTool) Description() string { return "writes its input to a file" }

func (f *fileSideEffectTool) Call(_ context.Context, input string) (string, error) {
	if err := os.WriteFile(f.path, []byte(input), 0o600); err != nil {
		return "", err
	}
	return f.path, nil
}

// occurred reports whether the side effect is observable on disk right now.
func (f *fileSideEffectTool) occurred() bool {
	_, err := os.Stat(f.path)
	return err == nil
}

// content returns what was actually written, or "" when nothing was.
func (f *fileSideEffectTool) content(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(f.path)
	if err != nil {
		return ""
	}
	return string(data)
}

// networkSideEffectTool is a Tool whose Call really performs an HTTP POST
// against a live loopback listener that records every request it receives. A
// denied tool must leave requests() empty; because the positive control drives
// the same listener, an empty log is evidence the egress did not happen rather
// than evidence it could not have.
type networkSideEffectTool struct {
	server *httptest.Server

	mu       sync.Mutex
	received []string
}

func newNetworkSideEffectTool(t *testing.T) *networkSideEffectTool {
	t.Helper()
	tool := &networkSideEffectTool{}
	tool.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		tool.mu.Lock()
		tool.received = append(tool.received, string(body))
		tool.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(tool.server.Close)
	return tool
}

func (n *networkSideEffectTool) Name() string        { return "send_http_request" }
func (n *networkSideEffectTool) Description() string { return "POSTs its input to a fixed URL" }

func (n *networkSideEffectTool) Call(ctx context.Context, input string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.server.URL, strings.NewReader(input))
	if err != nil {
		return "", err
	}
	resp, err := n.server.Client().Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.Status, nil
}

// requests returns the bodies the listener actually received, in arrival order.
func (n *networkSideEffectTool) requests() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.received...)
}

func (n *networkSideEffectTool) occurred() bool {
	return len(n.requests()) > 0
}

// policyGovernanceClient denies exactly the named tools and records the
// identity each decision was made against, so a control can assert that the
// deny is attributable rather than anonymous — the AAASM-5529 acceptance
// criterion for audit evidence.
type policyGovernanceClient struct {
	denyTools map[string]string

	mu       sync.Mutex
	checks   []CheckRequest
	records  []RecordRequest
	approves []ApprovalRequest
}

func newPolicyGovernanceClient(denied map[string]string) *policyGovernanceClient {
	if denied == nil {
		denied = map[string]string{}
	}
	return &policyGovernanceClient{denyTools: denied}
}

func (c *policyGovernanceClient) Check(_ context.Context, request CheckRequest) (Decision, error) {
	c.mu.Lock()
	c.checks = append(c.checks, request)
	c.mu.Unlock()
	if reason, denied := c.denyTools[request.ToolName]; denied {
		return Decision{Denied: true, Reason: reason}, nil
	}
	return Decision{}, nil
}

func (c *policyGovernanceClient) WaitForApproval(_ context.Context, request ApprovalRequest) (Decision, error) {
	c.mu.Lock()
	c.approves = append(c.approves, request)
	c.mu.Unlock()
	return Decision{}, nil
}

func (c *policyGovernanceClient) RecordResult(_ context.Context, request RecordRequest) error {
	c.mu.Lock()
	c.records = append(c.records, request)
	c.mu.Unlock()
	return nil
}

func (c *policyGovernanceClient) Close() error { return nil }
