//go:build scheduler.tasks && amd64 && windows

package task

// cgo C calls run on the system stack: C code such as a GPU driver needs a
// large stack, and Windows exception dispatch accepts only frames inside the
// thread stack bounds. A Go function that C calls goes back to the goroutine
// stack, below the frame that left it.

import "unsafe"

var (
	cgoOnSystemStack bool    // the current goroutine runs C code on the system stack
	cgoGoSP          uintptr // goroutine stack pointer while cgoOnSystemStack
	cgoThread        uint32  // the thread that runs goroutines
)

// callOnStack calls fn(arg) with the stack pointer set to sp, or on the
// current stack if sp is 0. It stores the old stack pointer in *oldSP.
//
//export tinygo_callOnStack
func callOnStack(fn, arg unsafe.Pointer, sp uintptr, oldSP *uintptr)

//export GetCurrentThreadId
func getCurrentThreadId() uint32

// CgoCall calls the C call wrapper fn(arg) on the system stack.
func CgoCall(fn, arg unsafe.Pointer) {
	if cgoOnSystemStack || Current() == nil {
		callOnStack(fn, arg, 0, nil)
		return
	}
	if cgoThread == 0 {
		cgoThread = getCurrentThreadId()
	}
	sp := cgoSysSP
	if sp == 0 {
		sp = systemStack
	}
	savedGoSP := cgoGoSP
	cgoOnSystemStack = true
	stackLimit = 0
	callOnStack(fn, arg, sp, &cgoGoSP)
	stackLimit = currentTask.state.limit()
	cgoOnSystemStack = false
	cgoGoSP = savedGoSP
}

// CgoCallbackSwitch reports whether a Go function that C code called must go
// back to the goroutine stack. Other threads keep their own stack.
func CgoCallbackSwitch() bool {
	return cgoOnSystemStack && getCurrentThreadId() == cgoThread
}

// CgoCallback calls the Go function wrapper fn(arg) on the goroutine stack.
func CgoCallback(fn, arg unsafe.Pointer) {
	savedSysSP := cgoSysSP
	cgoOnSystemStack = false
	stackLimit = currentTask.state.limit()
	callOnStack(fn, arg, cgoGoSP, &cgoSysSP)
	stackLimit = 0
	cgoOnSystemStack = true
	cgoSysSP = savedSysSP
}
