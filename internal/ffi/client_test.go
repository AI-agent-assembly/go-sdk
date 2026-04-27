package ffi

import (
	"errors"
	"testing"
	"unsafe"
)

func TestStatusToError(t *testing.T) {
	t.Parallel()

	if err := statusToError(statusOK, "op"); err != nil {
		t.Fatalf("expected nil for statusOK, got %v", err)
	}

	testCases := []struct {
		name   string
		status int32
		want   error
	}{
		{name: "null pointer", status: statusNullPointer, want: ErrNullPointer},
		{name: "invalid utf8", status: statusInvalidUTF8, want: ErrInvalidUTF8},
		{name: "not connected", status: statusNotConnected, want: ErrNotConnected},
		{name: "mutex poison", status: statusMutexPoison, want: ErrMutexPoison},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := statusToError(tc.status, "op")
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestClientWrappers(t *testing.T) {
	t.Parallel()

	binding := &mockBinding{response: `{"allow":true}`}
	client := NewClient(binding)

	if err := client.Connect("unix:///tmp/aa.sock"); err != nil {
		t.Fatalf("expected connect success, got %v", err)
	}
	if !binding.connectCalled {
		t.Fatal("expected connect to be called")
	}

	if err := client.SendEvent(`{"event":"test"}`); err != nil {
		t.Fatalf("expected send_event success, got %v", err)
	}
	if !binding.sendCalled {
		t.Fatal("expected send_event to be called")
	}

	response, err := client.QueryPolicy(`{"tool":"calculator"}`)
	if err != nil {
		t.Fatalf("expected query_policy success, got %v", err)
	}
	if response != binding.response {
		t.Fatalf("expected response %q, got %q", binding.response, response)
	}

	if err := client.Disconnect(); err != nil {
		t.Fatalf("expected disconnect success, got %v", err)
	}
	if !binding.disconnectCalled {
		t.Fatal("expected disconnect to be called")
	}
}

func TestClientBindingUnavailable(t *testing.T) {
	t.Parallel()

	client := NewClient(nil)
	if err := client.Connect("unix:///tmp/aa.sock"); !errors.Is(err, ErrBindingUnavailable) {
		t.Fatalf("expected ErrBindingUnavailable, got %v", err)
	}
}

type mockBinding struct {
	connectCalled    bool
	sendCalled       bool
	disconnectCalled bool
	response         string
}

func (m *mockBinding) connect(string) (unsafe.Pointer, int32) {
	m.connectCalled = true
	handle := new(byte)
	return unsafe.Pointer(handle), statusOK
}

func (m *mockBinding) sendEvent(unsafe.Pointer, string) int32 {
	m.sendCalled = true
	return statusOK
}

func (m *mockBinding) queryPolicy(unsafe.Pointer, string) (string, int32) {
	return m.response, statusOK
}

func (m *mockBinding) disconnect(unsafe.Pointer) int32 {
	m.disconnectCalled = true
	return statusOK
}
