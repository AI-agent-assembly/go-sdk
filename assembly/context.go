package assembly

import (
	"context"
	"fmt"

	"github.com/oklog/ulid/v2"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	agentIDContextKey contextKey = "assembly.agent_id"
	traceIDContextKey contextKey = "assembly.trace_id"
	runIDContextKey   contextKey = "assembly.run_id"
)

// WithAgentID returns a new context containing the assembly agent ID.
func WithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDContextKey, agentID)
}

// AgentIDFromContext returns the assembly agent ID, or an empty string if absent.
func AgentIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	agentID, _ := ctx.Value(agentIDContextKey).(string)
	return agentID
}

// WithTraceID returns a new context containing the assembly trace ID.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey, traceID)
}

// TraceIDFromContext returns the assembly trace ID, or an empty string if absent.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	traceID, _ := ctx.Value(traceIDContextKey).(string)
	if traceID != "" {
		return traceID
	}

	spanCtx := oteltrace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}

	return ""
}

// WithRunID returns a new context containing the assembly run ID.
func WithRunID(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, runIDContextKey, runID)
}

// RunIDFromContext returns the assembly run ID, generating one when absent.
func RunIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return generateRunID()
	}

	runID, _ := ctx.Value(runIDContextKey).(string)
	if runID != "" {
		return runID
	}

	return generateRunID()
}

// EnsureRunID returns a context with a stable run ID for the current context tree.
func EnsureRunID(ctx context.Context) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}

	runID, _ := ctx.Value(runIDContextKey).(string)
	if runID != "" {
		return ctx, runID
	}

	runID = generateRunID()
	return WithRunID(ctx, runID), runID
}

func generateRunID() string {
	return fmt.Sprintf("run_%s", ulid.Make().String())
}
