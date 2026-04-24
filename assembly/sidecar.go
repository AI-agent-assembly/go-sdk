package assembly

import "context"

// SidecarClient is the local gRPC sidecar contract used by the SDK.
type SidecarClient interface {
	Ping(ctx context.Context) error
}
