package assembly

import "time"

// Option mutates runtime options during Assembly construction.
type Option func(*runtimeOptions)

type runtimeOptions struct {
	gatewayURL     string
	apiKey         string
	failClosed     bool
	timeout        time.Duration
	sidecarAddress string
}

// WithGatewayURL sets the governance gateway URL.
func WithGatewayURL(gatewayURL string) Option {
	return func(opts *runtimeOptions) {
		opts.gatewayURL = gatewayURL
	}
}

// WithAPIKey sets the governance API key.
func WithAPIKey(apiKey string) Option {
	return func(opts *runtimeOptions) {
		opts.apiKey = apiKey
	}
}
