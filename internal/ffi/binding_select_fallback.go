//go:build !cgo || !aa_ffi_go

package ffi

func defaultBinding() binding {
	return fallbackUDSBridge{}
}
