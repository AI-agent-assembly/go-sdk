//go:build cgo && aa_ffi_go

package ffi

/*
#cgo LDFLAGS: -laa_ffi_go
#include <stdint.h>
#include <stdlib.h>

typedef struct aa_client_handle aa_client_handle;

typedef int32_t aa_status;

aa_status aa_connect(const char* endpoint, aa_client_handle** out_client);
aa_status aa_send_event(aa_client_handle* client, const char* event_json);
aa_status aa_query_policy(aa_client_handle* client, const char* query_json, char** out_response);
aa_status aa_disconnect(aa_client_handle* client);
void aa_free_string(char* value);
void aa_free_bytes(uint8_t* bytes, size_t len);
*/
import "C"

import "unsafe"

type cgoBridge struct{}

func (cgoBridge) connect(endpoint string) (unsafe.Pointer, int32) {
	cEndpoint := C.CString(endpoint)
	defer C.free(unsafe.Pointer(cEndpoint))

	var handle *C.aa_client_handle
	status := C.aa_connect(cEndpoint, &handle)
	return unsafe.Pointer(handle), int32(status)
}

func (cgoBridge) sendEvent(handle unsafe.Pointer, eventJSON string) int32 {
	cEventJSON := C.CString(eventJSON)
	defer C.free(unsafe.Pointer(cEventJSON))

	status := C.aa_send_event((*C.aa_client_handle)(handle), cEventJSON)
	return int32(status)
}

func (cgoBridge) queryPolicy(handle unsafe.Pointer, queryJSON string) (*C.char, int32) {
	cQueryJSON := C.CString(queryJSON)
	defer C.free(unsafe.Pointer(cQueryJSON))

	var out *C.char
	status := C.aa_query_policy((*C.aa_client_handle)(handle), cQueryJSON, &out)
	return out, int32(status)
}

func (cgoBridge) disconnect(handle unsafe.Pointer) int32 {
	status := C.aa_disconnect((*C.aa_client_handle)(handle))
	return int32(status)
}

func (cgoBridge) freeString(value *C.char) {
	C.aa_free_string(value)
}

func (cgoBridge) freeBytes(value *C.uint8_t, length C.size_t) {
	C.aa_free_bytes(value, length)
}
