//go:build !scheduler.tasks && amd64 && windows

package task

// Without goroutine stacks, C calls already run on the system stack.

import "unsafe"

//export tinygo_callOnStack
func callOnStack(fn, arg unsafe.Pointer, sp uintptr, oldSP *uintptr)

func CgoCall(fn, arg unsafe.Pointer) {
	callOnStack(fn, arg, 0, nil)
}

func CgoCallbackSwitch() bool {
	return false
}

func CgoCallback(fn, arg unsafe.Pointer) {
	callOnStack(fn, arg, 0, nil)
}

// tinygo_stackLimit stays 0, so ___chkstk_ms only probes the pages and never
// calls this function.
//
//export tinygo_stackOverflow
func stackOverflow() {
	runtimeFatal("goroutine stack overflow")
}
