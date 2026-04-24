package assembly

import (
	"context"
	"errors"
	"testing"
)

func TestConnectToLocalSidecarReturnsUnavailable(t *testing.T) {
	t.Parallel()

	_, err := connectToLocalSidecar(context.Background(), "127.0.0.1:50051")
	if !errors.Is(err, ErrSidecarUnavailable) {
		t.Fatalf("expected ErrSidecarUnavailable, got %v", err)
	}
}
