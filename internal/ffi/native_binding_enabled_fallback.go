//go:build !cgo || !aa_ffi_go

package ffi

// NativeBindingEnabled reports whether the Rust staticlib-backed CGo binding is active.
func NativeBindingEnabled() bool {
	return false
}
