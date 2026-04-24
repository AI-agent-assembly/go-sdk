package assembly

import (
	"context"
	"errors"
)

// SidecarClient is the local gRPC sidecar contract used by the SDK.
type SidecarClient interface {
	Ping(ctx context.Context) error
}

// ErrSidecarUnavailable indicates the local sidecar cannot be reached.
var ErrSidecarUnavailable = errors.New("assembly sidecar unavailable")

func connectToLocalSidecar(ctx context.Context, address string) (SidecarClient, error) {
	_, _ = ctx, address
	return nil, ErrSidecarUnavailable
}
