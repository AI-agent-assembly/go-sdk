package ffi

import (
	"errors"
	"fmt"
)

const (
	statusOK           int32 = 0
	statusNullPointer  int32 = 1
	statusInvalidUTF8  int32 = 2
	statusNotConnected int32 = 3
	statusMutexPoison  int32 = 4
)

var (
	ErrNullPointer  = errors.New("ffi null pointer")
	ErrInvalidUTF8  = errors.New("ffi invalid utf-8")
	ErrNotConnected = errors.New("ffi client not connected")
	ErrMutexPoison  = errors.New("ffi mutex poisoned")
)

func statusToError(status int32, operation string) error {
	switch status {
	case statusOK:
		return nil
	case statusNullPointer:
		return fmt.Errorf("%s: %w", operation, ErrNullPointer)
	case statusInvalidUTF8:
		return fmt.Errorf("%s: %w", operation, ErrInvalidUTF8)
	case statusNotConnected:
		return fmt.Errorf("%s: %w", operation, ErrNotConnected)
	case statusMutexPoison:
		return fmt.Errorf("%s: %w", operation, ErrMutexPoison)
	default:
		return fmt.Errorf("%s: ffi status %d", operation, status)
	}
}
