package assembly

import (
	"context"
	"errors"
	"testing"

	"github.com/agent-assembly/go-sdk/internal/ffi"
)

func TestInit(t *testing.T) {
	t.Run("connector success", func(t *testing.T) {
		originalConnector := sidecarConnector
		t.Cleanup(func() {
			sidecarConnector = originalConnector
		})

		sidecarConnector = func(ctx context.Context, address string) (SidecarClient, error) {
			if ctx == nil {
				t.Fatal("expected context to be set")
			}
			return nil, nil
		}

		a, err := Init(context.Background(),
			WithGatewayURL("https://gateway.example.com"),
			WithAPIKey("test-key"),
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if a == nil {
			t.Fatal("expected non-nil Assembly")
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

		a, err := Init(context.Background(),
			WithGatewayURL("https://gateway.example.com"),
			WithAPIKey("test-key"),
		)
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected error %v, got %v", wantErr, err)
		}
		if a != nil {
			t.Fatal("expected nil Assembly on error")
		}
	})

	t.Run("validation failure returns error", func(t *testing.T) {
		a, err := Init(context.Background(), WithAPIKey("test-key"))
		if !errors.Is(err, ErrInvalidGateway) {
			t.Fatalf("expected ErrInvalidGateway, got %v", err)
		}
		if a != nil {
			t.Fatal("expected nil Assembly on validation error")
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

		a, err := Init(context.Background(), validTestOptions()...)
		if err != nil {
			t.Fatalf("expected fallback ffi connect to succeed, got %v", err)
		}
		if a == nil {
			t.Fatal("expected non-nil Assembly")
		}
	})
}
