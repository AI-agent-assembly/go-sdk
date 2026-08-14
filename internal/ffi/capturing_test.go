package ffi

import (
	"testing"
)

func TestCapturingClientRecordsSendEventPayloads(t *testing.T) {
	t.Parallel()

	client, events := NewCapturingClient()

	if err := client.Connect("unix:///tmp/aa-cap-test.sock", "", ""); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	type evt struct{ eventType, details string }
	sent := []evt{
		{"register", `{"parent":"a"}`},
		{"tool_call", `{"tool":"calc"}`},
	}
	for _, e := range sent {
		if err := client.SendEvent(e.eventType, e.details); err != nil {
			t.Fatalf("SendEvent(%q,%q) failed: %v", e.eventType, e.details, err)
		}
	}

	if len(*events) != len(sent) {
		t.Fatalf("expected %d events captured, got %d", len(sent), len(*events))
	}
	for i, e := range sent {
		if (*events)[i] != e.details {
			t.Errorf("events[%d] = %q, want %q", i, (*events)[i], e.details)
		}
	}
}

func TestConnectForwardsAgentIDAndSDKVersion(t *testing.T) {
	t.Parallel()

	// AAASM-3683: Client.Connect must forward the agent id and the Go-module SDK
	// version down to the binding so they are signed into the runtime handshake.
	b := &capturingBinding{}
	client := NewClient(b)

	if err := client.Connect("unix:///tmp/aa-version-test.sock", "agent-7", "go-9.8.7"); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if b.ConnectAgentID != "agent-7" {
		t.Errorf("ConnectAgentID = %q, want %q", b.ConnectAgentID, "agent-7")
	}
	if b.ConnectSDKVersion != "go-9.8.7" {
		t.Errorf("ConnectSDKVersion = %q, want %q", b.ConnectSDKVersion, "go-9.8.7")
	}
}

func TestCapturingClientDisconnect(t *testing.T) {
	t.Parallel()

	client, _ := NewCapturingClient()

	if err := client.Connect("unix:///tmp/aa-cap-test2.sock", "", ""); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if err := client.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
}

// TestRecordingClientExposesBothBoundaryChannels covers NewRecordingClient in
// this package's own test binary (AAASM-5731).
//
// It is exercised from assembly's audit-sink suite, but Go attributes coverage
// per package: code in internal/ffi run from another package's tests counts as
// uncovered here unless -coverpkg is set, which nothing in this repo does. That
// is a reporting artefact, not untested code — this test makes the two channels
// genuinely covered where the profile can see them, rather than arguing the
// point on a review.
func TestRecordingClientExposesBothBoundaryChannels(t *testing.T) {
	t.Parallel()

	client, crossings := NewRecordingClient(DecisionDeny, "blocked by policy")

	if err := client.Connect("unix:///tmp/aa-recording-test.sock", "agent-1", "go-1.2.3"); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	decision, reason, err := client.QueryPolicy("agent-1", "tool_call", "web_search", `{"q":"x"}`)
	if err != nil {
		t.Fatalf("QueryPolicy failed: %v", err)
	}
	if decision != DecisionDeny || reason != "blocked by policy" {
		t.Fatalf("QueryPolicy = (%d, %q), want (%d, %q)", decision, reason, DecisionDeny, "blocked by policy")
	}

	// The query channel records the arguments, which is what makes it usable as
	// the positive control for an audit measurement.
	if len(crossings.Queries()) != 1 {
		t.Fatalf("Queries = %d, want 1", len(crossings.Queries()))
	}
	query := crossings.Queries()[0]
	if query.AgentID != "agent-1" || query.ActionType != "tool_call" ||
		query.ToolName != "web_search" || query.ArgsJSON != `{"q":"x"}` {
		t.Errorf("recorded query = %+v, want the arguments passed to QueryPolicy", query)
	}

	// The event channel is separate, and empty until something sends.
	if len(crossings.Events()) != 0 {
		t.Fatalf("Events = %v, want empty before any SendEvent", crossings.Events())
	}
	if err := client.SendEvent("register", `{"event_type":"register"}`); err != nil {
		t.Fatalf("SendEvent failed: %v", err)
	}
	if len(crossings.Events()) != 1 || crossings.Events()[0] != `{"event_type":"register"}` {
		t.Fatalf("Events = %v, want the one payload sent", crossings.Events())
	}
}
