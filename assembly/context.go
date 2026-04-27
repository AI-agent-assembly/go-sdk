package assembly

import "context"

import oteltrace "go.opentelemetry.io/otel/trace"

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
