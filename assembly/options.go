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
	sidecarBinary  string
}

// WithGatewayURL sets the governance gateway URL. This option is required;
// [Init] returns [ErrInvalidGateway] if it is not set.
func WithGatewayURL(gatewayURL string) Option {
	return func(opts *runtimeOptions) {
		opts.gatewayURL = gatewayURL
	}
}

// WithAPIKey sets the governance API key. This option is required;
// [Init] returns [ErrInvalidAPIKey] if it is not set.
func WithAPIKey(apiKey string) Option {
	return func(opts *runtimeOptions) {
		opts.apiKey = apiKey
	}
}

// WithFailClosed toggles gateway failure behavior. When true, a governance
// check failure causes the tool call to be rejected. When false (the default),
// the tool call proceeds even if the governance check fails.
func WithFailClosed(failClosed bool) Option {
	return func(opts *runtimeOptions) {
		opts.failClosed = failClosed
	}
}

// WithTimeout sets the gateway check timeout. If not set, the default
// timeout is 500ms. The timeout is applied only when the caller's context
// does not already carry a deadline.
func WithTimeout(timeout time.Duration) Option {
	return func(opts *runtimeOptions) {
		opts.timeout = timeout
	}
}

// WithSidecarBinary sets the path to the sidecar binary for managed lifecycle.
// When set, [Init] launches the sidecar as a subprocess and waits for it to
// become healthy before returning. If not set, the SDK connects to an
// already-running sidecar.
func WithSidecarBinary(path string) Option {
	return func(opts *runtimeOptions) {
		opts.sidecarBinary = path
	}
}

func withSidecarAddress(sidecarAddress string) Option {
	return func(opts *runtimeOptions) {
		opts.sidecarAddress = sidecarAddress
	}
}
