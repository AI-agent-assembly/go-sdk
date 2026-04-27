package ffi

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"
)

func TestConcurrentSendEventSafety(t *testing.T) {
	binding := &raceSafeBinding{}
	client := NewClient(binding)

	if err := client.Connect("unix:///tmp/aa.sock"); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	const workers = 20
	const sendsPerWorker = 100

	var group sync.WaitGroup
	group.Add(workers)

	for worker := 0; worker < workers; worker++ {
		worker := worker
		go func() {
			defer group.Done()
			for sendIndex := 0; sendIndex < sendsPerWorker; sendIndex++ {
				payload := fmt.Sprintf(`{"worker":%d,"send":%d}`, worker, sendIndex)
				if err := client.SendEvent(payload); err != nil {
					t.Errorf("send_event failed: %v", err)
					return
				}
			}
		}()
	}

	group.Wait()

	if got := atomic.LoadUint64(&binding.sent); got != workers*sendsPerWorker {
		t.Fatalf("expected %d sends, got %d", workers*sendsPerWorker, got)
	}
}

type raceSafeBinding struct {
	sent uint64
}

func (r *raceSafeBinding) connect(string) (unsafe.Pointer, int32) {
	handle := new(byte)
	return unsafe.Pointer(handle), statusOK
}

func (r *raceSafeBinding) sendEvent(unsafe.Pointer, string) int32 {
	atomic.AddUint64(&r.sent, 1)
	return statusOK
}

func (r *raceSafeBinding) queryPolicy(unsafe.Pointer, string) (string, int32) {
	return `{"allow":true}`, statusOK
}

func (r *raceSafeBinding) disconnect(unsafe.Pointer) int32 {
	return statusOK
}
