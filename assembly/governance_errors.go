package assembly

import (
	"errors"
	"fmt"
)

// ErrRuntimeNotInitialized indicates runtime APIs were used before Init.
var ErrRuntimeNotInitialized = errors.New("assembly: runtime is not initialized")

// PolicyViolationError indicates a policy decision denied tool execution.
type PolicyViolationError struct {
	// ToolName is the name of the tool whose execution was denied.
	ToolName string
	// Reason is the human-readable explanation from the governance gateway.
	Reason string
}

// Error returns a formatted message including the tool name and denial reason
// when available.
func (e *PolicyViolationError) Error() string {
	if e == nil {
		return "assembly: policy violation"
	}

	if e.ToolName == "" && e.Reason == "" {
		return "assembly: policy violation"
	}

	if e.ToolName == "" {
		return fmt.Sprintf("assembly: policy violation: %s", e.Reason)
	}

	if e.Reason == "" {
		return fmt.Sprintf("assembly: policy violation: tool=%s", e.ToolName)
	}

	return fmt.Sprintf("assembly: policy violation: tool=%s reason=%s", e.ToolName, e.Reason)
}
