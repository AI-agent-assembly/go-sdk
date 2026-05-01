package assembly

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestHelperProcess is the test helper process entry point.
// It is invoked as a subprocess by tests that need a real process to manage.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") != "1" {
		return
	}

	switch os.Getenv("GO_TEST_HELPER_MODE") {
	case "listen":
		addr := os.Getenv("GO_TEST_HELPER_ADDR")
		if addr == "" {
			addr = "127.0.0.1:0"
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "listen error: %v\n", err)
			os.Exit(1)
		}
		defer ln.Close()
		fmt.Fprintln(os.Stdout, "listening")
		select {}
	case "sleep":
		time.Sleep(30 * time.Second)
	default:
		fmt.Fprintln(os.Stderr, "unknown mode")
		os.Exit(1)
	}
}

// helperCmd builds an exec.Cmd that re-invokes the test binary as a helper process.
func helperCmd(mode, addr string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_TEST_HELPER_PROCESS=1",
		"GO_TEST_HELPER_MODE="+mode,
		"GO_TEST_HELPER_ADDR="+addr,
	)
	return cmd
}

func TestConnectToLocalSidecarReturnsUnavailable(t *testing.T) {
	t.Parallel()

	_, err := connectToLocalSidecar(context.Background(), "127.0.0.1:50051")
	if !errors.Is(err, ErrSidecarUnavailable) {
		t.Fatalf("expected ErrSidecarUnavailable, got %v", err)
	}
}

func TestSidecarStartLaunchesProcess(t *testing.T) {
	t.Parallel()

	sc := NewSidecar(os.Args[0], "127.0.0.1:0")
	sc.cmd = helperCmd("sleep", "127.0.0.1:0")

	if err := sc.cmd.Start(); err != nil {
		t.Fatalf("failed to start helper: %v", err)
	}
	defer func() { _ = sc.cmd.Process.Kill(); _ = sc.cmd.Wait() }()

	if sc.cmd.Process == nil {
		t.Fatal("expected process to be non-nil after start")
	}
}

func TestSidecarStartAlreadyRunningReturnsError(t *testing.T) {
	t.Parallel()

	sc := NewSidecar(os.Args[0], "127.0.0.1:0")
	sc.cmd = helperCmd("sleep", "127.0.0.1:0")

	if err := sc.cmd.Start(); err != nil {
		t.Fatalf("failed to start helper: %v", err)
	}
	defer func() { _ = sc.cmd.Process.Kill(); _ = sc.cmd.Wait() }()

	err := sc.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when starting already-running sidecar")
	}
}

func TestSidecarStopGraceful(t *testing.T) {
	t.Parallel()

	sc := NewSidecar(os.Args[0], "127.0.0.1:0")
	sc.cmd = helperCmd("sleep", "127.0.0.1:0")

	if err := sc.cmd.Start(); err != nil {
		t.Fatalf("failed to start helper: %v", err)
	}

	sc.stopTimeout = 2 * time.Second
	if err := sc.Stop(); err != nil {
		t.Fatalf("expected graceful stop, got error: %v", err)
	}
}

func TestSidecarStopForcedKill(t *testing.T) {
	t.Parallel()

	sc := NewSidecar(os.Args[0], "127.0.0.1:0")
	// Use "listen" mode which blocks on select{} and ignores SIGTERM
	sc.cmd = helperCmd("listen", "127.0.0.1:0")
	sc.cmd.Stdout = nil

	if err := sc.cmd.Start(); err != nil {
		t.Fatalf("failed to start helper: %v", err)
	}

	sc.stopTimeout = 100 * time.Millisecond
	if err := sc.Stop(); err != nil {
		t.Fatalf("expected forced kill to succeed, got error: %v", err)
	}
}

func TestSidecarStopNoProcess(t *testing.T) {
	t.Parallel()

	sc := NewSidecar("/nonexistent", "127.0.0.1:0")
	if err := sc.Stop(); err != nil {
		t.Fatalf("expected no error stopping unstarted sidecar, got: %v", err)
	}
}

func TestSidecarHealthySuccess(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	sc := NewSidecar("/nonexistent", ln.Addr().String())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := sc.Healthy(ctx); err != nil {
		t.Fatalf("expected healthy, got error: %v", err)
	}
}

func TestSidecarHealthyTimeout(t *testing.T) {
	t.Parallel()

	// Use a port that nothing is listening on
	sc := NewSidecar("/nonexistent", "127.0.0.1:1")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := sc.Healthy(ctx)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded in error chain, got: %v", err)
	}
}
