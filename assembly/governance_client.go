package assembly

import "context"

// GovernanceClient describes policy gateway operations used by the runtime.
type GovernanceClient interface {
	Check(ctx context.Context, request CheckRequest) (Decision, error)
	WaitForApproval(ctx context.Context, request ApprovalRequest) (Decision, error)
	RecordResult(ctx context.Context, request RecordRequest) error
	Close() error
}
