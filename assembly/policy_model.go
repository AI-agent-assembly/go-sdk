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

// RecordRequest carries the outcome of one governed tool call to
// [GovernanceClient.RecordResult].
//
// Whether it reaches an audit trail is a property of the client, not of this
// struct: the client this SDK ships discards it (see [AuditSinkDisposition],
// AAASM-5731), so on the shipped path nothing built from this type is retained.
//
// Since AAASM-5665 the wrapper emits one for a call that was denied before
// execution as well as one that ran, so the struct no longer implies the tool
// body executed. A denied call carries an empty Result and the short-circuit
// error in Error. There is deliberately no discriminator field yet: adding one
// is a wire-contract change, so a consumer that must tell "denied before
// execution" from "ran and returned an error" has only the Error text to go on.
type RecordRequest struct {
	// ToolName is the name of the tool the call targeted.
	ToolName string
	// TraceID is the distributed trace identifier for correlation.
	TraceID string
	// RunID is a stable identifier for the current execution run.
	RunID string
	// Result is the string output returned by the tool, empty when the call was
	// denied before execution.
	Result string
	// Error is the tool's error message, the denial reason when the call was
	// denied before execution, or empty on success.
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
