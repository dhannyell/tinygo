//go:build windows && amd64

package runtime

import (
	"internal/task"
	"unsafe"
)

// The compiler calls these functions around cgo C calls and around Go
// functions that C code calls. See compiler/cgocall.go.

func cgocall(fn, arg unsafe.Pointer) {
	task.CgoCall(fn, arg)
}

func cgocallbackSwitch() bool {
	return task.CgoCallbackSwitch()
}

func cgocallback(fn, arg unsafe.Pointer) {
	task.CgoCallback(fn, arg)
}
