package ffi

import (
	"errors"
	"sync"
	"unsafe"
)

var ErrBindingUnavailable = errors.New("ffi binding unavailable")

// binding encapsulates low-level transport calls.
type binding interface {
	connect(endpoint string) (unsafe.Pointer, int32)
	sendEvent(handle unsafe.Pointer, eventJSON string) int32
	queryPolicy(handle unsafe.Pointer, queryJSON string) (string, int32)
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

// Connect establishes an FFI session with the runtime endpoint.
func (c *Client) Connect(endpoint string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.binding == nil {
		return ErrBindingUnavailable
	}

	handle, status := c.binding.connect(endpoint)
	if err := statusToError(status, "connect"); err != nil {
		return err
	}

	c.handle = handle
	return nil
}

// SendEvent forwards an event payload through the active FFI session.
func (c *Client) SendEvent(eventJSON string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.binding == nil {
		return ErrBindingUnavailable
	}

	if c.handle == nil {
		return statusToError(statusNotConnected, "send_event")
	}

	status := c.binding.sendEvent(c.handle, eventJSON)
	return statusToError(status, "send_event")
}

// QueryPolicy requests a policy decision over the active FFI session.
func (c *Client) QueryPolicy(queryJSON string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.binding == nil {
		return "", ErrBindingUnavailable
	}

	if c.handle == nil {
		return "", statusToError(statusNotConnected, "query_policy")
	}

	response, status := c.binding.queryPolicy(c.handle, queryJSON)
	if err := statusToError(status, "query_policy"); err != nil {
		return "", err
	}

	return response, nil
}
