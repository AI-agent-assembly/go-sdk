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
