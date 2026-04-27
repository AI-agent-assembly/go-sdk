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

// WithFailClosed toggles gateway failure behavior.
func WithFailClosed(failClosed bool) Option {
	return func(opts *runtimeOptions) {
		opts.failClosed = failClosed
	}
}

// WithTimeout sets the gateway check timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(opts *runtimeOptions) {
		opts.timeout = timeout
	}
}

func withSidecarAddress(sidecarAddress string) Option {
	return func(opts *runtimeOptions) {
		opts.sidecarAddress = sidecarAddress
	}
}
