package assembly

type contextKey string

const (
	agentIDContextKey contextKey = "assembly.agent_id"
	traceIDContextKey contextKey = "assembly.trace_id"
	runIDContextKey   contextKey = "assembly.run_id"
)
