package ffi

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"unsafe"
)

func TestMemoryRegressionHarness(t *testing.T) {
	if os.Getenv("AAASM_MEMORY_HARNESS") != "1" {
		t.Skip("set AAASM_MEMORY_HARNESS=1 to run 1M send_event memory harness")
	}

	binding := &memoryHarnessBinding{}
	client := NewClient(binding)
	if err := client.Connect("unix:///tmp/aa.sock"); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for index := 0; index < 1_000_000; index++ {
		payload := fmt.Sprintf(`{"seq":%d}`, index)
		if err := client.SendEvent(payload); err != nil {
			t.Fatalf("send_event %d failed: %v", index, err)
		}
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	const maxGrowthBytes uint64 = 16 * 1024 * 1024
	if after.Alloc > before.Alloc+maxGrowthBytes {
		t.Fatalf("memory grew beyond threshold: before=%d after=%d threshold=%d", before.Alloc, after.Alloc, maxGrowthBytes)
	}
}

type memoryHarnessBinding struct{}

func (m *memoryHarnessBinding) connect(string) (unsafe.Pointer, int32) {
	handle := new(byte)
	return unsafe.Pointer(handle), statusOK
}

func (m *memoryHarnessBinding) sendEvent(unsafe.Pointer, string) int32 {
	return statusOK
}

func (m *memoryHarnessBinding) queryPolicy(unsafe.Pointer, string) (string, int32) {
	return `{"allow":true}`, statusOK
}

func (m *memoryHarnessBinding) disconnect(unsafe.Pointer) int32 {
	return statusOK
}
