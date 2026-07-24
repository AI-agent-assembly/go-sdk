package ffi

import (
	"errors"
	"testing"
	"unsafe"
)

// statusBinding is a configurable binding that returns preset status codes so
// the Client error-mapping branches can be driven without a native transport.
type statusBinding struct {
	connectStatus    int32
	sendStatus       int32
	disconnectStatus int32
}

func (b statusBinding) connect(string, string, string) (unsafe.Pointer, int32) {
	if b.connectStatus != statusOK {
		return nil, b.connectStatus
	}
	return unsafe.Pointer(new(byte)), statusOK
}

func (b statusBinding) sendEvent(unsafe.Pointer, string, string) int32 { return b.sendStatus }

func (b statusBinding) disconnect(unsafe.Pointer) int32 { return b.disconnectStatus }

func TestClient_ConnectSurfacesBindingError(t *testing.T) {
	t.Parallel()

	client := NewClient(statusBinding{connectStatus: statusIPCError})
	if err := client.Connect("unix:///tmp/aa.sock", "", ""); !errors.Is(err, ErrIPC) {
		t.Fatalf("expected ErrIPC from connect, got %v", err)
	}
	// A failed connect must not leave a handle, so a later send reports
	// not-connected rather than dispatching on a bogus handle.
	if err := client.SendEvent("t", "d"); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected after a failed connect, got %v", err)
	}
}

func TestClient_SendEventSurfacesBindingError(t *testing.T) {
	t.Parallel()

	client := NewClient(statusBinding{sendStatus: statusChannelClosed})
	if err := client.Connect("unix:///tmp/aa.sock", "", ""); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := client.SendEvent("tool_call", "{}"); !errors.Is(err, ErrChannelClosed) {
		t.Fatalf("expected ErrChannelClosed from send, got %v", err)
	}
}

func TestClient_DisconnectSurfacesBindingError(t *testing.T) {
	t.Parallel()

	client := NewClient(statusBinding{disconnectStatus: statusPanic})
	if err := client.Connect("unix:///tmp/aa.sock", "", ""); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := client.Disconnect(); !errors.Is(err, ErrPanic) {
		t.Fatalf("expected ErrPanic from disconnect, got %v", err)
	}
}

// TestClient_DisconnectClearsHandleOnNonOKStatus guards the AAASM-5052
// invariant: the native aa_disconnect frees the handle unconditionally
// (Box::from_raw) before returning its status, so even on a non-OK status the
// Client must drop its handle. If it kept the dangling pointer, a later call
// would be use-after-free and a second Disconnect a double-free.
func TestClient_DisconnectClearsHandleOnNonOKStatus(t *testing.T) {
	t.Parallel()

	client := NewClient(statusBinding{disconnectStatus: statusPanic})
	if err := client.Connect("unix:///tmp/aa.sock", "", ""); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := client.Disconnect(); !errors.Is(err, ErrPanic) {
		t.Fatalf("expected ErrPanic from disconnect, got %v", err)
	}

	// Handle must be cleared despite the non-OK status so freed memory is
	// never reused.
	if client.handle != nil {
		t.Fatal("expected handle cleared after non-OK disconnect (use-after-free risk)")
	}

	// A second Disconnect must not re-invoke the native free (double-free);
	// with the handle cleared it short-circuits to not-connected.
	if err := client.Disconnect(); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected on repeat disconnect, got %v", err)
	}
}
