package assembly

import (
	"context"
	"testing"
)

func TestAgentIDRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := WithAgentID(context.Background(), "data-analyst")
	if got := AgentIDFromContext(ctx); got != "data-analyst" {
		t.Fatalf("expected agent id data-analyst, got %q", got)
	}
}

func TestAgentIDFromContextMissingReturnsEmpty(t *testing.T) {
	t.Parallel()

	if got := AgentIDFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty agent id, got %q", got)
	}

	if got := AgentIDFromContext(nil); got != "" {
		t.Fatalf("expected empty agent id for nil context, got %q", got)
	}
}
