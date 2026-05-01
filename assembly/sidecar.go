package assembly

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"syscall"
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

// Stop sends SIGTERM to the sidecar process and waits for graceful shutdown.
// If the process does not exit within the stop timeout, it sends SIGKILL.
func (s *Sidecar) Stop() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("assembly: failed to signal sidecar: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(s.stopTimeout):
		if killErr := s.cmd.Process.Kill(); killErr != nil {
			return fmt.Errorf("assembly: failed to kill sidecar: %w", killErr)
		}
		<-done
		return nil
	}
}

const healthPollInterval = 50 * time.Millisecond

// Healthy polls the sidecar address via TCP until it accepts connections
// or the context is cancelled.
func (s *Sidecar) Healthy(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("assembly: sidecar health check timed out: %w", ctx.Err())
		default:
			conn, err := net.DialTimeout("tcp", s.address, healthPollInterval)
			if err == nil {
				conn.Close()
				return nil
			}
			time.Sleep(healthPollInterval)
		}
	}
}

func connectToLocalSidecar(ctx context.Context, address string) (SidecarClient, error) {
	_, _ = ctx, address
	return nil, ErrSidecarUnavailable
}
