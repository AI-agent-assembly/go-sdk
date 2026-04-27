//go:build !aa_ffi_go

package ffi

import (
	"encoding/json"
	"unsafe"
)

type fallbackUDSHandle struct {
	endpoint  string
	connected bool
	events    uint64
}

type fallbackUDSBridge struct{}

func (fallbackUDSBridge) connect(endpoint string) (unsafe.Pointer, int32) {
	handle := &fallbackUDSHandle{
		endpoint:  endpoint,
		connected: true,
	}

	return unsafe.Pointer(handle), statusOK
}

func (fallbackUDSBridge) sendEvent(handle unsafe.Pointer, _ string) int32 {
	client := (*fallbackUDSHandle)(handle)
	if client == nil {
		return statusNullPointer
	}
	if !client.connected {
		return statusNotConnected
	}

	client.events++
	return statusOK
}

func (fallbackUDSBridge) queryPolicy(handle unsafe.Pointer, queryJSON string) (string, int32) {
	client := (*fallbackUDSHandle)(handle)
	if client == nil {
		return "", statusNullPointer
	}
	if !client.connected {
		return "", statusNotConnected
	}

	payload, err := json.Marshal(map[string]any{
		"allow":      true,
		"reason":     "fallback-uds",
		"endpoint":   client.endpoint,
		"events_sent": client.events,
		"query":      queryJSON,
	})
	if err != nil {
		return "", statusInvalidUTF8
	}

	return string(payload), statusOK
}

func (fallbackUDSBridge) disconnect(handle unsafe.Pointer) int32 {
	client := (*fallbackUDSHandle)(handle)
	if client == nil {
		return statusNullPointer
	}
	if !client.connected {
		return statusNotConnected
	}

	client.connected = false
	return statusOK
}
