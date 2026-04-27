package assembly

import "strings"

func validateRuntimeOptions(opts runtimeOptions) error {
	if strings.TrimSpace(opts.gatewayURL) == "" {
		return ErrInvalidGateway
	}

	if strings.TrimSpace(opts.apiKey) == "" {
		return ErrInvalidAPIKey
	}

	return nil
}
