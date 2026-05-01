// Package assembly provides Go SDK bootstrap and interception primitives.
package assembly

import (
	"context"
	"errors"
)

var (
	// ErrInvalidGateway indicates the Gateway configuration is missing.
	ErrInvalidGateway = errors.New("assembly: gateway is required")
	// ErrInvalidAPIKey indicates the API key configuration is missing.
	ErrInvalidAPIKey = errors.New("assembly: api key is required")
)

var sidecarConnector = connectToLocalSidecar

// Init configures and initializes the assembly runtime in a single call.
//
// Example:
//
//	a, err := assembly.Init(ctx,
//	    assembly.WithGatewayURL("https://gateway.example.com"),
//	    assembly.WithAPIKey("my-key"),
//	    assembly.WithFailClosed(true),
//	)
func Init(ctx context.Context, options ...Option) (*Assembly, error) {
	a := newAssembly(options...)
	if err := a.boot(ctx); err != nil {
		return nil, err
	}
	return a, nil
}
