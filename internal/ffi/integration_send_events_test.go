package ffi

import (
	"fmt"
	"sync/atomic"
	"testing"
	"unsafe"
)

func TestIntegrationSendThousandEvents(t *testing.T) {
	binding := &countingBinding{}
	client := NewClient(binding)

	if err := client.Connect("unix:///tmp/aa.sock"); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	for index := 0; index < 1000; index++ {
		event := fmt.Sprintf(`{"event":%d}`, index)
		if err := client.SendEvent(event); err != nil {
			t.Fatalf("send_event %d failed: %v", index, err)
		}
	}

	if got := atomic.LoadUint64(&binding.sent); got != 1000 {
		t.Fatalf("expected 1000 events, got %d", got)
	}
}

type countingBinding struct {
	sent uint64
}

func (c *countingBinding) connect(string) (unsafe.Pointer, int32) {
	handle := new(byte)
	return unsafe.Pointer(handle), statusOK
}

func (c *countingBinding) sendEvent(unsafe.Pointer, string) int32 {
	atomic.AddUint64(&c.sent, 1)
	return statusOK
}

func (c *countingBinding) queryPolicy(unsafe.Pointer, string) (string, int32) {
	return `{"allow":true}`, statusOK
}

func (c *countingBinding) disconnect(unsafe.Pointer) int32 {
	return statusOK
}
