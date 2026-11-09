package main

import (
	"runtime"
	"unsafe"
)

var xorshift32State uint32 = 1

func xorshift32(x uint32) uint32 {
	// Algorithm "xor" from p. 4 of Marsaglia, "Xorshift RNGs"
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	return x
}

func randuint32() uint32 {
	xorshift32State = xorshift32(xorshift32State)
	return xorshift32State
}

func main() {
	// These run first, so the freed objects are the ones reused.
	testPreciseGlobal()
	testPointerArray()
	testPinner()
	testNonPointerHeap()
	testKeepAlive()
}

// A Pinner holds the only references to small objects, like C code that keeps
// Go pointers. The XOR hides the addresses from a conservative scan.
func testPinner() {
	const hide = ^uintptr(0)
	n := 2001 // boehm reuses freed memory only at this size
	if ^uintptr(0) <= 0xffff {
		n = 21
	}
	var pinner runtime.Pinner
	addrs := make([]uintptr, n)
	for i := range addrs {
		v := uint32(i) * 2654435761
		pinner.Pin(&v)
		addrs[i] = uintptr(unsafe.Pointer(&v)) ^ hide
	}
	runtime.GC()
	reuseFreed(len(addrs))
	for i, addr := range addrs {
		if *(*uint32)(unsafe.Pointer(addr ^ hide)) != uint32(i)*2654435761 {
			panic("a pinned object was freed")
		}
	}
	pinner.Unpin()
	println("ok")
}

var scalarSlices [4][]byte
var randSeeds [4]uint32

func testNonPointerHeap() {
	maxSliceSize := uint32(1024)
	if ^uintptr(0) <= 0xffff {
		// 16-bit and lower devices, such as AVR.
		// Heap size is a real issue there, while it is still useful to run
		// these tests. Therefore, lower the max slice size.
		maxSliceSize = 64
	}
	// Allocate roughly 0.5MB of memory.
	for i := 0; i < 1000; i++ {
		// Pick a random index that the optimizer can't predict.
		index := randuint32() % 4

		// Check whether the contents of the previous allocation was correct.
		rand := randSeeds[index]
		for _, b := range scalarSlices[index] {
			rand = xorshift32(rand)
			if b != byte(rand) {
				panic("memory was overwritten!")
			}
		}

		// Allocate a randomly-sized slice, randomly sliced to be smaller.
		sliceLen := randuint32() % maxSliceSize
		slice := make([]byte, sliceLen)
		cutLen := randuint32() % maxSliceSize
		if cutLen < sliceLen {
			slice = slice[cutLen:]
		}
		scalarSlices[index] = slice

		// Fill the slice with a pattern that looks random but is easily
		// calculated and verified.
		rand = randuint32() + 1
		randSeeds[index] = rand
		for i := 0; i < len(slice); i++ {
			rand = xorshift32(rand)
			slice[i] = byte(rand)
		}
	}
	println("ok")
}

func testKeepAlive() {
	// There isn't much we can test, but at least we can test that
	// runtime.KeepAlive compiles correctly.
	var x int
	runtime.KeepAlive(&x)
}

type pointerElem struct {
	a, b, c uint64
	p       *uint32
	d       uint64
}

var reused []*uint32

// A large array holds the only references to small objects. The GC must find
// them through the pointer words of the element layout.
func testPointerArray() {
	n := 2001 // odd and above 50 elements, like most real arrays
	if ^uintptr(0) <= 0xffff {
		n = 21
	}
	elems := make([]pointerElem, n)
	for i := range elems {
		v := uint32(i) * 2654435761
		elems[i].p = &v
	}
	runtime.GC()
	reuseFreed(n)
	for i := range elems {
		if *elems[i].p != uint32(i)*2654435761 {
			panic("a pointer in an array element was not marked")
		}
	}
	println("ok")
}

// Reuse the memory of any object the GC freed by mistake. A global holds each
// new object, so the compiler cannot remove the allocations.
func reuseFreed(n int) {
	reused = make([]*uint32, 16*n)
	for i := range reused {
		reused[i] = new(uint32)
		*reused[i] = 0xdeadbeef
	}
	runtime.GC()
}

// A large global holds the only references to small objects. The GC must find
// them through the pointer words of the global layout. 16-bit targets have no
// room for it.
var preciseGlobal [128 * (unsafe.Sizeof(uintptr(0)) / 4)]pointerElem

func testPreciseGlobal() {
	for i := range preciseGlobal {
		v := uint32(i) * 2654435761
		preciseGlobal[i].p = &v
	}
	runtime.GC()
	reuseFreed(len(preciseGlobal))
	for i := range preciseGlobal {
		if *preciseGlobal[i].p != uint32(i)*2654435761 {
			panic("a pointer in a large global was not marked")
		}
	}
	println("ok")
}
