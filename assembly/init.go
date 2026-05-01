// Package assembly provides Go SDK bootstrap and interception primitives.
package assembly

import (
	"context"
	"errors"
	"time"
)

// Config contains the user-supplied bootstrap settings.
type Config struct {
	Gateway        string
	APIKey         string
	SidecarAddress string
	FailClosed     bool
	Timeout        time.Duration
}

var (
	// ErrInvalidGateway indicates the Gateway configuration is missing.
	ErrInvalidGateway = errors.New("assembly: gateway is required")
	// ErrInvalidAPIKey indicates the API key configuration is missing.
	ErrInvalidAPIKey = errors.New("api key is required")
)

var sidecarConnector = connectToLocalSidecar

// InitAssembly initializes the SDK runtime.
func InitAssembly(cfg Config) error {
	runtime := NewAssembly(
		WithGatewayURL(cfg.Gateway),
		WithAPIKey(cfg.APIKey),
		WithFailClosed(cfg.FailClosed),
		WithTimeout(cfg.Timeout),
		withSidecarAddress(cfg.SidecarAddress),
	)

	return runtime.Init(context.Background())
}

func validateConfig(cfg Config) error {
	return validateRuntimeOptions(runtimeOptions{
		gatewayURL: cfg.Gateway,
		apiKey:     cfg.APIKey,
	})
}
