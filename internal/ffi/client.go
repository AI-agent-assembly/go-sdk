package ffi

import (
	"errors"
	"sync"
	"unsafe"
)

// ErrBindingUnavailable reports that no FFI transport binding is compiled in.
var ErrBindingUnavailable = errors.New("ffi binding unavailable")

// binding encapsulates low-level transport calls.
//
// Policy is decided server-side by the authoritative runtime, so there is no
// query_policy surface here — the SDK only reports events (AAASM-2552).
type binding interface {
	// connect opens a session. agentID is signed into the runtime handshake
	// (AAASM-3587) and sdkVersion (the Go-module SDK version) is signed into the
	// handshake proof so downgrade detection reflects the installed release
	// (AAASM-3683); an empty sdkVersion falls back to the crate version.
	connect(endpoint, agentID, sdkVersion string) (unsafe.Pointer, int32)
	sendEvent(handle unsafe.Pointer, eventType, details string) int32
	disconnect(handle unsafe.Pointer) int32
}

// Client wraps FFI transport operations.
type Client struct {
	mu      sync.Mutex
	binding binding
	handle  unsafe.Pointer
}

// NewClient constructs a wrapper around a low-level FFI binding.
func NewClient(lowLevelBinding binding) *Client {
	return &Client{binding: lowLevelBinding}
}

// NewDefaultClient constructs a client with build-selected transport binding.
func NewDefaultClient() *Client {
	return NewClient(defaultBinding())
}

// Connect establishes an FFI session with the runtime endpoint.
//
// agentID is signed into the runtime session handshake (AAASM-3587); sdkVersion
// is the user-facing Go-module SDK version forwarded so it — not the shared
// aa-sdk-client crate version — is signed into the handshake proof for accurate
// downgrade detection (AAASM-3683). An empty sdkVersion falls back to the crate
// version (no regression vs AAASM-3666).
func (c *Client) Connect(endpoint, agentID, sdkVersion string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.binding == nil {
		return ErrBindingUnavailable
	}

	handle, status := c.binding.connect(endpoint, agentID, sdkVersion)
	if err := statusToError(status, "connect"); err != nil {
		return err
	}

	c.handle = handle
	return nil
}

// SendEvent reports an audit event (eventType, details) through the active FFI
// session. The runtime re-scans every event authoritatively.
func (c *Client) SendEvent(eventType, details string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.binding == nil {
		return ErrBindingUnavailable
	}

	if c.handle == nil {
		return statusToError(statusNotConnected, "send_event")
	}

	status := c.binding.sendEvent(c.handle, eventType, details)
	return statusToError(status, "send_event")
}

// Disconnect closes the active FFI session.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.binding == nil {
		return ErrBindingUnavailable
	}

	if c.handle == nil {
		return statusToError(statusNotConnected, "disconnect")
	}

	// The native aa_disconnect unconditionally reclaims the handle
	// (Box::from_raw frees it) before computing its return status, so a
	// non-OK status does NOT mean the free was skipped. Clear c.handle the
	// moment the native free has been invoked — regardless of status — or a
	// later call would use freed memory and a repeat Disconnect would
	// double-free (AAASM-5052).
	status := c.binding.disconnect(c.handle)
	c.handle = nil
	return statusToError(status, "disconnect")
}
