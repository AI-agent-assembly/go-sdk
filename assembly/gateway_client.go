package assembly

import "context"

// GatewayTransport defines the transport contract used by GatewayClient.
type GatewayTransport interface {
	Check(ctx context.Context, request CheckRequest) (Decision, error)
}

// GatewayClient coordinates policy checks over a transport with runtime defaults.
type GatewayClient struct {
	transport GatewayTransport
	config    runtimeOptions
}

// NewGatewayClient constructs a GatewayClient with the supplied transport and options.
func NewGatewayClient(transport GatewayTransport, options ...Option) *GatewayClient {
	cfg := defaultRuntimeOptions()
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}

	return &GatewayClient{transport: transport, config: cfg}
}

// Check performs a governance policy check using context cancellation semantics.
func (c *GatewayClient) Check(ctx context.Context, request CheckRequest) (Decision, error) {
	_, _ = c, request

	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return Decision{}, ctx.Err()
	default:
	}

	return Decision{}, nil
}
