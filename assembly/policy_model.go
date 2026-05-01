package assembly

// CheckRequest is sent to the governance gateway before tool execution.
type CheckRequest struct {
	// ToolName is the name of the tool being invoked.
	ToolName string
	// Args is the raw input argument string passed to the tool.
	Args string
	// AgentID identifies the agent making the tool call.
	AgentID string
	// TraceID is the distributed trace identifier for correlation.
	TraceID string
	// RunID is a stable identifier for the current execution run.
	RunID string
}

// ApprovalRequest is sent while waiting for out-of-band human approval.
type ApprovalRequest struct {
	// ToolName is the name of the tool awaiting approval.
	ToolName string
	// TraceID is the distributed trace identifier for correlation.
	TraceID string
	// RunID is a stable identifier for the current execution run.
	RunID string
}

// RecordRequest stores tool execution results for governance and audit.
type RecordRequest struct {
	// ToolName is the name of the tool that was executed.
	ToolName string
	// TraceID is the distributed trace identifier for correlation.
	TraceID string
	// RunID is a stable identifier for the current execution run.
	RunID string
	// Result is the string output returned by the tool.
	Result string
	// Error is the error message if the tool call failed, or empty on success.
	Error string
}

// Decision captures the governance gateway's policy outcome for a tool call.
type Decision struct {
	// Denied is true when the policy gateway has rejected the tool call.
	Denied bool
	// Pending is true when the tool call requires out-of-band approval.
	Pending bool
	// Reason provides a human-readable explanation for the decision.
	Reason string
}
