package assembly

import "time"

// Option mutates runtime options during Assembly construction.
type Option func(*runtimeOptions)

type runtimeOptions struct {
	gatewayURL    string
	apiKey        string
	failClosed    bool
	timeout       time.Duration
	sidecarAddress string
}
