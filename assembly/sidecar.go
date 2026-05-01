package assembly

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// SidecarClient is the local gRPC sidecar contract used by the SDK.
type SidecarClient interface {
	Ping(ctx context.Context) error
}

// ErrSidecarUnavailable indicates the local sidecar cannot be reached.
var ErrSidecarUnavailable = errors.New("assembly: sidecar unavailable")

const defaultStopTimeout = 5 * time.Second

// Sidecar manages the lifecycle of a local sidecar subprocess.
type Sidecar struct {
	binaryPath  string
	address     string
	cmd         *exec.Cmd
	stopTimeout time.Duration
}

// NewSidecar creates a Sidecar for the given binary and listen address.
func NewSidecar(binaryPath, address string) *Sidecar {
	return &Sidecar{
		binaryPath:  binaryPath,
		address:     address,
		stopTimeout: defaultStopTimeout,
	}
}

// Start launches the sidecar binary as a subprocess.
func (s *Sidecar) Start(ctx context.Context) error {
	if s.cmd != nil && s.cmd.Process != nil {
		return fmt.Errorf("assembly: sidecar already started")
	}

	s.cmd = exec.CommandContext(ctx, s.binaryPath, "--listen", s.address)
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("assembly: failed to start sidecar: %w", err)
	}

	return nil
}

func connectToLocalSidecar(ctx context.Context, address string) (SidecarClient, error) {
	_, _ = ctx, address
	return nil, ErrSidecarUnavailable
}
