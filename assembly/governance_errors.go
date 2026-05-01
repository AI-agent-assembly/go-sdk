package assembly

import (
	"errors"
	"fmt"
)

// ErrRuntimeNotInitialized indicates runtime APIs were used before Init.
var ErrRuntimeNotInitialized = errors.New("assembly: runtime is not initialized")

// PolicyViolationError indicates a policy decision denied tool execution.
type PolicyViolationError struct {
	ToolName string
	Reason   string
}

func (e *PolicyViolationError) Error() string {
	if e == nil {
		return "policy violation"
	}

	if e.ToolName == "" && e.Reason == "" {
		return "policy violation"
	}

	if e.ToolName == "" {
		return fmt.Sprintf("policy violation: %s", e.Reason)
	}

	if e.Reason == "" {
		return fmt.Sprintf("policy violation: tool=%s", e.ToolName)
	}

	return fmt.Sprintf("policy violation: tool=%s reason=%s", e.ToolName, e.Reason)
}
