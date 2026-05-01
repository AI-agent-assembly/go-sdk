package assembly

import (
	"context"
	"errors"
	"testing"
)

func TestAssemblyLifecycle(t *testing.T) {
	t.Parallel()

	assembly := newAssembly(
		WithGatewayURL("https://gateway.example.com"),
		WithAPIKey("test-key"),
	)

	connectorCalled := false
	assembly.sidecarConnector = func(ctx context.Context, address string) (SidecarClient, error) {
		connectorCalled = true
		if ctx == nil {
			t.Fatal("expected context to be set")
		}
		if address != "" {
			t.Fatalf("expected default sidecar address to be empty, got %q", address)
		}
		return stubSidecarClient{}, nil
	}

	if err := assembly.boot(context.Background()); err != nil {
		t.Fatalf("expected no init error, got %v", err)
	}
	if !connectorCalled {
		t.Fatal("expected sidecar connector to be called")
	}
	if assembly.sidecar == nil {
		t.Fatal("expected sidecar to be set after init")
	}

	if err := assembly.Close(); err != nil {
		t.Fatalf("expected no close error, got %v", err)
	}
	if assembly.sidecar != nil {
		t.Fatal("expected sidecar to be released after close")
	}
}

func TestAssemblyInitValidation(t *testing.T) {
	t.Parallel()

	assembly := newAssembly(WithAPIKey("test-key"))
	assembly.sidecarConnector = func(context.Context, string) (SidecarClient, error) {
		t.Fatal("connector should not run when config is invalid")
		return nil, nil
	}

	err := assembly.boot(context.Background())
	if !errors.Is(err, ErrInvalidGateway) {
		t.Fatalf("expected ErrInvalidGateway, got %v", err)
	}
}

type stubSidecarClient struct{}

func (stubSidecarClient) Ping(context.Context) error {
	return nil
}
