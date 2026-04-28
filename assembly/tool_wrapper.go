package assembly

import (
	"context"
	"fmt"
)

// Tool is the minimal tool contract used by this SDK package.
type Tool interface {
	Name() string
	Description() string
	Call(ctx context.Context, input string) (string, error)
}

// AssemblyTool wraps a Tool with governance hooks.
type AssemblyTool struct { //nolint:revive // Keep API name aligned with AAASM-63 contract.
	inner  Tool
	client GovernanceClient
	opts   runtimeOptions
}

// NewAssemblyTool constructs a governance wrapper around a tool.
func NewAssemblyTool(inner Tool, client GovernanceClient, opts runtimeOptions) *AssemblyTool {
	return &AssemblyTool{
		inner:  inner,
		client: client,
		opts:   opts,
	}
}

// Name passes through the wrapped tool name.
func (t *AssemblyTool) Name() string {
	if t.inner == nil {
		return ""
	}
	return t.inner.Name()
}

// Description passes through the wrapped tool description.
func (t *AssemblyTool) Description() string {
	if t.inner == nil {
		return ""
	}
	return t.inner.Description()
}

// Call executes governance hooks before and after tool execution.
func (t *AssemblyTool) Call(ctx context.Context, input string) (string, error) {
	if t.inner == nil {
		return "", ErrRuntimeNotInitialized
	}

	if t.client != nil {
		ctxWithRunID, runID := EnsureRunID(ctx)
		ctx = ctxWithRunID

		decision, err := t.client.Check(ctx, CheckRequest{
			ToolName: t.inner.Name(),
			Args:     input,
			AgentID:  AgentIDFromContext(ctx),
			TraceID:  TraceIDFromContext(ctx),
			RunID:    runID,
		})
		if err != nil {
			if t.opts.failClosed {
				return "", fmt.Errorf("governance check failed: %w", err)
			}
		} else {
			if decision.Denied {
				return "", &PolicyViolationError{ToolName: t.inner.Name(), Reason: decision.Reason}
			}
			if decision.Pending {
				decision, err = t.client.WaitForApproval(ctx, ApprovalRequest{
					ToolName: t.inner.Name(),
					TraceID:  TraceIDFromContext(ctx),
					RunID:    RunIDFromContext(ctx),
				})
				if err != nil {
					return "", fmt.Errorf("approval wait failed: %w", err)
				}
				if decision.Denied {
					return "", &PolicyViolationError{ToolName: t.inner.Name(), Reason: decision.Reason}
				}
			}
		}
	}

	result, err := t.inner.Call(ctx, input)

	if t.client != nil {
		recordCtx := context.WithoutCancel(ctx)
		go func() {
			_ = t.client.RecordResult(recordCtx, RecordRequest{
				ToolName: t.inner.Name(),
				TraceID:  TraceIDFromContext(recordCtx),
				RunID:    RunIDFromContext(recordCtx),
				Result:   result,
				Error:    errString(err),
			})
		}()
	}

	return result, err
}

var _ Tool = (*AssemblyTool)(nil)

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
