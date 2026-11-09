package main

// runtime.Callers walks goroutine frames and frames on both sides of C.

// #include "abi.h"
import "C"

import "runtime"

var sink int

//go:noinline
func depth(k int, pc []uintptr) int {
	if k == 0 {
		return runtime.Callers(0, pc)
	}
	n := depth(k-1, pc)
	sink++ // Prevents a tail call.
	return n
}

//go:noinline
func skipCheck() bool {
	var a, b [64]uintptr
	na := runtime.Callers(1, a[:])
	nb := runtime.Callers(2, b[:])
	return na > 2 && na-nb == 1 && a[1] == b[0]
}

//go:noinline
func framesCheck() bool {
	var pc [64]uintptr
	n := runtime.Callers(0, pc[:])
	frames := runtime.CallersFrames(pc[:n])
	count := 0
	for {
		frame, more := frames.Next()
		if frame.PC != pc[count] {
			return false
		}
		count++
		if !more {
			break
		}
	}
	return n > 0 && count == n
}

var viaC int

//export goCallers
func goCallers() {
	var pc [64]uintptr
	viaC = runtime.Callers(0, pc[:])
}

func checkCallers() {
	var pc [64]uintptr
	println("callers deeper:", depth(2, pc[:])-depth(0, pc[:]))
	println("callers skip:", skipCheck())
	println("callers frames:", framesCheck())
	direct := depth(0, pc[:])
	C.abi_callers()
	println("callers through C:", viaC > direct+1)
}
