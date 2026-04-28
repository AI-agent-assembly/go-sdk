package assembly

// CheckRequest is sent before tool execution.
type CheckRequest struct {
	ToolName string
	Args     string
	AgentID  string
	TraceID  string
	RunID    string
}

// ApprovalRequest is used while waiting for out-of-band approval.
type ApprovalRequest struct {
	ToolName string
	TraceID  string
	RunID    string
}

// RecordRequest stores execution results for governance/audit purposes.
type RecordRequest struct {
	ToolName string
	TraceID  string
	RunID    string
	Result   string
	Error    string
}

// Decision captures gateway policy outcomes.
type Decision struct {
	Denied  bool
	Pending bool
	Reason  string
}
