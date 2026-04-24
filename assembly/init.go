package assembly

import (
	"context"
	"errors"
	"strings"
)

// Config contains the user-supplied bootstrap settings.
type Config struct {
	Gateway        string
	APIKey         string
	SidecarAddress string
}

var (
	// ErrInvalidGateway indicates the Gateway configuration is missing.
	ErrInvalidGateway = errors.New("gateway is required")
	// ErrInvalidAPIKey indicates the API key configuration is missing.
	ErrInvalidAPIKey = errors.New("api key is required")
)

var sidecarConnector = connectToLocalSidecar

// InitAssembly initializes the SDK runtime.
func InitAssembly(cfg Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}

	_, err := sidecarConnector(context.Background(), cfg.SidecarAddress)
	return err
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Gateway) == "" {
		return ErrInvalidGateway
	}

	if strings.TrimSpace(cfg.APIKey) == "" {
		return ErrInvalidAPIKey
	}

	return nil
}
