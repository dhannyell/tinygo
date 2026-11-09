//go:build windows

package task

import "unsafe"

// threadID is a thread HANDLE.
type threadID unsafe.Pointer
