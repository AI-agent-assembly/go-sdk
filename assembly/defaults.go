package assembly

import "time"

const defaultGatewayTimeout = 500 * time.Millisecond

func defaultRuntimeOptions() runtimeOptions {
	return runtimeOptions{
		failClosed: false,
		timeout:    defaultGatewayTimeout,
	}
}
