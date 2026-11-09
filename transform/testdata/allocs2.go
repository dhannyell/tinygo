package main

import (
	"runtime/volatile"
	"unsafe"
)

func main() {
	n1 := 5
	derefInt(&n1)

	n2 := 6
	returnIntPtr(&n2)

	s1 := make([]int, 3)
	readIntSlice(s1)

	s2 := [3]int{}
	readIntSlice(s2[:])

	s3 := make([]int, 3)
	returnIntSlice(s3)

	useSlice(make([]int, getUnknownNumber())) // OUT: size is not constant

	s4 := make([]byte, 300) // OUT: object size 300 exceeds maximum stack allocation size 256
	readByteSlice(s4)

	s5 := make([]int, 4) // OUT: escapes at line 30
	_ = append(s5, 5)

	s6 := make([]int, 3)
	s7 := []int{1, 2, 3}
	copySlice(s6, s7)

	c1 := getComplex128() // OUT: escapes at line 37
	useInterface(c1)

	n3 := 5
	func() int {
		return n3
	}()

	callVariadic(3, 5, 8) // OUT: escapes at line 44

	s8 := []int{3, 5, 8} // OUT: escapes at line 47
	callVariadic(s8...)

	n4 := 3 // the closure only reads and writes through its context
	n5 := 7
	func() {
		n4 = n5
	}()
	println(n4, n5)

	// This shouldn't escape.
	var buf [32]byte
	s := string(buf[:])
	println(len(s))

	var rbuf [5]rune
	s = string(rbuf[:])
	println(s)

	// Unsafe usage of DMA buffers: the compiler thinks this buffer won't be
	// used anymore after the volatile store.
	var dmaBuf1 [4]byte
	pseudoVolatile.Set(uint32(unsafeNoEscape(unsafe.Pointer(&dmaBuf1[0]))))

	// Safe usage of DMA buffers: keep the buffer alive until it is no longer
	// needed, but don't mark it as needing to be heap allocated. The compiler
	// will keep the buffer stack allocated if possible.
	var dmaBuf2 [4]byte
	pseudoVolatile.Set(uint32(unsafeNoEscape(unsafe.Pointer(&dmaBuf2[0]))))
	// ...use the buffer in the DMA peripheral
	keepAliveNoEscape(unsafe.Pointer(&dmaBuf2[0]))
}

type vector3 [3]float32

func scaleVector3(vec *vector3, f float32) *vector3 {
	vec[0] *= f
	vec[1] *= f
	vec[2] *= f
	return vec
}

func crossVector3(a, b *vector3) vector3 {
	return vector3{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func nonEscapingReturnedPointer() vector3 {
	a := vector3{1, 2, 3}
	b := vector3{4, 5, 6}

	c := scaleVector3(&b, 0.5)
	return crossVector3(&a, c)
}

var escapedSlice []int

func escapingReturnedSlice() {
	s := make([]int, 3) // OUT: escapes at line 108
	escapedSlice = returnIntSlice(s)
}

var escapedVector3 *vector3

func escapingReturnedPointer() {
	b := vector3{4, 5, 6} // OUT: escapes at line 117

	c := scaleVector3(&b, 0.5)
	escapedVector3 = c
}

func recursiveScaleVector3(vec *vector3, n int) *vector3 {
	if n == 0 {
		return vec
	}
	return recursiveScaleVector3(vec, n-1)
}

func recursiveReturnedPointer() vector3 {
	b := vector3{4, 5, 6} // OUT: escapes at unknown line

	c := recursiveScaleVector3(&b, 1)
	return *c
}

func derefInt(x *int) int {
	return *x
}

func returnIntPtr(x *int) *int {
	return x
}

func readIntSlice(s []int) int {
	return s[1]
}

func readByteSlice(s []byte) byte {
	return s[1]
}

func returnIntSlice(s []int) []int {
	return s
}

func getUnknownNumber() int

func copySlice(out, in []int) {
	copy(out, in)
}

func getComplex128() complex128

func useInterface(interface{})

func callVariadic(...int)

func useSlice([]int)

// See the function with the same name in the machine package.
//
//go:linkname unsafeNoEscape machine.unsafeNoEscape
func unsafeNoEscape(ptr unsafe.Pointer) uintptr

//go:linkname keepAliveNoEscape machine.keepAliveNoEscape
func keepAliveNoEscape(ptr unsafe.Pointer)

var pseudoVolatile volatile.Register32

// A pointer merged by a phi does not escape when the merge does not escape.
func phiNoEscape(flip bool) float32 {
	a := vector3{1, 2, 3}
	b := vector3{4, 5, 6}
	p := &a
	if flip {
		p = &b
	}
	return scaleVector3(p, 2)[0]
}

// A pointer stored into a local array does not escape when its loads do not.
func storedPointerNoEscape(i int) int {
	x, y := 1, 2
	ptrs := [2]*int{&x, &y}
	return *ptrs[i&1]
}

// An object allocated in a loop and carried to the next iteration must stay on
// the heap: a stack slot would be shared by all iterations.
func loopCarriedEscapes(n int) int {
	var prev *int
	for i := range n {
		v := i // OUT: escapes at line 199
		if prev != nil {
			v += *prev
		}
		prev = &v
	}
	return *prev
}

// A local container that leaves the function lets the stored pointer escape.
func containerPassedEscapes() {
	x := 1 // OUT: escapes at line 214
	ptrs := [1]*int{&x} // OUT: escapes at line 214
	usePtrs(ptrs[:])
}

func usePtrs([]*int)

type counter struct{ n int }

func (c *counter) visit(i int) bool {
	c.n += i
	return true
}

var savedCounter *counter

func (c *counter) save(int) bool {
	savedCounter = c
	return true
}

// visitAll only calls fn, so the context of fn does not leave it.
func visitAll(n int, fn func(int) bool) {
	for i := range n {
		if !fn(i) {
			return
		}
	}
}

// A method value passed to a function that only calls it does not escape.
func calledCallbackNoEscape(n int) int {
	var c counter
	visitAll(n, c.visit)
	return c.n
}

// The receiver escapes when the bound method stores it.
func storingCallbackEscapes(n int) {
	var c counter // OUT: escapes at line 229
	visitAll(n, c.save)
}

type holder struct{ p *int }

var savedInt *int

func readHolder(h *holder) int {
	return *h.p
}

func leakHeld(h *holder) {
	savedInt = h.p
}

// A container passed to a function that only reads through it does not escape.
func containerReadNoEscape() int {
	x := 1
	h := holder{&x}
	return readHolder(&h)
}

// The stored pointer escapes when the callee copies it out of the container.
func containerLeakedEscapes() {
	x := 1 // OUT: escapes at line 264
	h := holder{&x}
	leakHeld(&h)
}
