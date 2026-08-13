// Package assembly provides Go SDK bootstrap and interception primitives.
package assembly

import (
	"context"
	"errors"
	"os"
)

const (
	// envFallbackGatewayURL is the environment variable consulted when
	// WithGatewayURL is not set.
	envFallbackGatewayURL = "AA_GATEWAY_URL"
	// envFallbackControlPlaneURL is the environment variable consulted when
	// WithControlPlaneURL is not set.
	envFallbackControlPlaneURL = "AA_CONTROL_PLANE_URL"
)

var (
	// ErrInvalidGateway indicates the Gateway configuration is missing.
	ErrInvalidGateway = errors.New("assembly: gateway is required")
	// ErrInvalidControlPlane indicates the control-plane URL is missing.
	ErrInvalidControlPlane = errors.New("assembly: control plane url is required")
)

// resolveWithEnvFallback resolves a configuration value using the precedence
// explicit option > environment variable > error. The explicit value wins when
// non-empty; otherwise the named environment variable is consulted. When both
// are empty and the value is required, missingErr is returned so callers can
// surface a typed error at operation time. When the value is not required, an
// empty string is returned with a nil error.
func resolveWithEnvFallback(explicit, envVar string, required bool, missingErr error) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if env := os.Getenv(envVar); env != "" {
		return env, nil
	}
	if required {
		return "", missingErr
	}
	return "", nil
}

// resolveControlPlaneURL resolves the HTTP control-plane URL using the
// precedence explicit option (WithControlPlaneURL) > AA_CONTROL_PLANE_URL
// environment variable > ErrInvalidControlPlane when required. There is no
// HTTP control-plane caller in the Go SDK today; this helper exists so the
// first HTTP consumer can resolve the URL consistently with the Python and
// Node SDKs.
func resolveControlPlaneURL(explicit string, required bool) (string, error) {
	return resolveWithEnvFallback(explicit, envFallbackControlPlaneURL, required, ErrInvalidControlPlane)
}

// resolveGatewayURLWithEnvFallback resolves the gateway URL using the
// precedence explicit option (WithGatewayURL) > AA_GATEWAY_URL environment
// variable > ErrInvalidGateway when required.
func resolveGatewayURLWithEnvFallback(explicit string, required bool) (string, error) {
	return resolveWithEnvFallback(explicit, envFallbackGatewayURL, required, ErrInvalidGateway)
}

var sidecarConnector = connectToLocalSidecar

// Init configures and initializes the assembly runtime in a single call.
//
// When WithSidecarBinary is set, Init starts the sidecar and health-checks it
// before returning. That health check is always bounded: it honours a deadline
// on ctx when one is present, and otherwise applies an internal default timeout.
// Init therefore returns even when ctx never cancels (e.g. context.Background())
// and the sidecar address is empty or unreachable — the caller is not required
// to supply a context with a deadline to be guaranteed a return.
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
	// Auto-populate parentAgentID from context when not explicitly provided.
	// This allows child agents spawned inside WrapChain calls to inherit
	// lineage automatically without manual WithParentAgentID threading.
	if a.opts.parentAgentID == "" {
		if parentID := ParentAgentIDFromContext(ctx); parentID != "" {
			a.opts.parentAgentID = parentID
		}
	}
	if err := a.boot(ctx); err != nil {
		return nil, err
	}
	// AAASM-5731 — surface an audit sink that retains nothing on the default
	// path. Emitted here rather than inside boot because boot has two success
	// returns (the native FFI path and the sidecar fallback) and the warning must
	// fire on exactly one of them per Init, whichever was taken.
	if disposition := a.AuditSink(); disposition != AuditSinkCallerSupplied {
		warnAuditNotRecorded(disposition)
	}
	return a, nil
}
