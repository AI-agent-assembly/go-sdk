package assembly

import (
	"context"
	"testing"

	oteltrace "go.opentelemetry.io/otel/trace"
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

func TestTraceIDRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := WithTraceID(context.Background(), "trace-123")
	if got := TraceIDFromContext(ctx); got != "trace-123" {
		t.Fatalf("expected trace id trace-123, got %q", got)
	}
}

func TestTraceIDFromContextFallsBackToOpenTelemetrySpanContext(t *testing.T) {
	t.Parallel()

	traceID := oteltrace.TraceID{
		0x01, 0x02, 0x03, 0x04,
		0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c,
		0x0d, 0x0e, 0x0f, 0x10,
	}
	spanID := oteltrace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}

	spanCtx := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
	})

	ctx := oteltrace.ContextWithSpanContext(context.Background(), spanCtx)
	if got := TraceIDFromContext(ctx); got != traceID.String() {
		t.Fatalf("expected otel trace id %q, got %q", traceID.String(), got)
	}

	ctx = WithTraceID(ctx, "assembly-trace")
	if got := TraceIDFromContext(ctx); got != "assembly-trace" {
		t.Fatalf("expected explicit assembly trace id to win, got %q", got)
	}
}
