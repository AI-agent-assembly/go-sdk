package assembly

import "context"

// GovernanceClient describes policy gateway operations used by the runtime.
type GovernanceClient interface {
	Check(ctx context.Context, request any) (any, error)
	WaitForApproval(ctx context.Context, request any) (any, error)
	RecordResult(ctx context.Context, request any) error
}
