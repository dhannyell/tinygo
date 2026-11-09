package main

// Structs by value at the C boundary, in both directions.

// #include "abi.h"
import "C"

import "runtime"

func main() {
	v3 := C.s3{a: 1, b: 2, c: 3}
	r3 := C.abi_s3(v3)
	println("s3:", r3.a, r3.b, r3.c, v3.a)

	v4 := C.s4{a: 4}
	r4 := C.abi_s4(v4)
	println("s4:", r4.a, v4.a)

	v8 := C.s8{a: 1, b: 2}
	r8 := C.abi_s8(v8)
	println("s8:", r8.a, r8.b, v8.a)

	v12 := C.s12{a: 1, b: 2, c: 3}
	r12 := C.abi_s12(v12)
	println("s12:", r12.a, r12.b, r12.c, v12.a)

	v16 := C.s16{a: 1, b: 2}
	r16 := C.abi_s16(v16)
	println("s16:", r16.a, r16.b, v16.a)

	v40 := C.s40{a: 1, b: 2, c: 3, d: 4}
	r40 := C.abi_s40(v40)
	println("s40:", r40.e, v40.e)

	println("mixed:", C.abi_mixed(1, C.s16{a: 2, b: 3}, C.s3{c: 4}, 5.5, C.s40{e: 6}))
	println("callback:", C.abi_callback())
	println("big stack:", C.abi_big_stack(1))
	println("nested:", C.abi_nested(1))
	checkCallers()
}

//export goSum
func goSum(v C.s16, w C.s40) C.s8 {
	return C.s8{a: C.int(v.a + w.e), b: C.int(v.b)}
}

//export goPick
func goPick(i C.int, v C.s40) C.s40 {
	v.a = C.longlong(i)
	return v
}

//export goNested
func goNested(seed C.int) C.longlong {
	// The GC scans the goroutine stack, so this must run on it.
	kept := []int{2}
	runtime.GC()
	return C.abi_big_stack(seed) * C.longlong(kept[0])
}
