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
