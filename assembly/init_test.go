package assembly

import (
	"context"
	"errors"
	"testing"

	"github.com/agent-assembly/go-sdk/internal/ffi"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{
			name: "missing gateway",
			cfg: Config{
				APIKey:         "test-key",
				SidecarAddress: "127.0.0.1:50051",
			},
			wantErr: ErrInvalidGateway,
		},
		{
			name: "missing api key",
			cfg: Config{
				Gateway:        "https://gateway.example.com",
				SidecarAddress: "127.0.0.1:50051",
			},
			wantErr: ErrInvalidAPIKey,
		},
		{
			name:    "valid config",
			cfg:     validTestConfig(),
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateConfig(tc.cfg)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestInitAssembly(t *testing.T) {
	t.Run("connector success", func(t *testing.T) {
		originalConnector := sidecarConnector
		t.Cleanup(func() {
			sidecarConnector = originalConnector
		})

		sidecarConnector = func(ctx context.Context, address string) (SidecarClient, error) {
			if ctx == nil {
				t.Fatal("expected context to be set")
			}
			if address != "" {
				t.Fatalf("unexpected address: %s", address)
			}
			return nil, nil
		}

		cfg := validTestConfig()
		cfg.SidecarAddress = ""
		err := InitAssembly(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("connector failure", func(t *testing.T) {
		originalConnector := sidecarConnector
		t.Cleanup(func() {
			sidecarConnector = originalConnector
		})

		wantErr := errors.New("sidecar unavailable")
		sidecarConnector = func(context.Context, string) (SidecarClient, error) {
			return nil, wantErr
		}

		cfg := validTestConfig()
		cfg.SidecarAddress = ""
		err := InitAssembly(cfg)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected error %v, got %v", wantErr, err)
		}
	})

	t.Run("fallback ffi transport is selected before sidecar connector", func(t *testing.T) {
		if ffi.NativeBindingEnabled() {
			t.Skip("native aa_ffi_go binding build does not use fallback transport")
		}

		originalConnector := sidecarConnector
		t.Cleanup(func() {
			sidecarConnector = originalConnector
		})

		sidecarConnector = func(context.Context, string) (SidecarClient, error) {
			return nil, errors.New("connector should not be reached when fallback ffi connect succeeds")
		}

		err := InitAssembly(validTestConfig())
		if err != nil {
			t.Fatalf("expected fallback ffi connect to succeed, got %v", err)
		}
	})
}
