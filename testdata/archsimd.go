//go:build goexperiment.simd && amd64

package main

// The expected output comes from the gc compiler with real AVX2 instructions.

import (
	"math"
	"simd/archsimd"
	"unsafe"
)

var (
	valuesA = [8]float32{1.5, -2, 3, 4, 5, 6, 7, 8}
	valuesB = [8]float32{10, 20, -30, 3, 50, 7, 70, 1}
	valuesC = [8]float32{0, float32(math.NaN()), 3, 0, 5, 0, 7, 0}
	groups  = [8]int32{-1, 0, -1, 0, 0, -1, 0, -1}
)

// layout has fields after a vector: Go and LLVM offsets must agree.
type layout struct {
	before int32
	vector archsimd.Float32x8
	after  int32
}

func show(name string, v archsimd.Float32x8) {
	var out [8]float32
	v.StoreArray(&out)
	print(name)
	for _, value := range out {
		print(" ", math.Float32bits(value))
	}
	println()
}

func main() {
	if !archsimd.X86.AVX2() {
		println("no AVX2")
		return
	}

	a := archsimd.LoadFloat32x8Array(&valuesA)
	b := archsimd.LoadFloat32x8Array(&valuesB)
	c := archsimd.LoadFloat32x8Array(&valuesC)
	mask := archsimd.LoadInt32x8Array(&groups).ToMask()

	show("add", a.Add(b))
	show("sub", a.Sub(b))
	show("mul", a.Mul(b))
	show("div", a.Div(b))
	show("broadcast", archsimd.BroadcastFloat32x8(2.5))
	show("ifelse", a.IfElse(mask, b))

	println("equal", a.Equal(c).ToBits())
	println("notequal", a.NotEqual(c).ToBits())
	println("greater", a.Greater(b).ToBits())
	println("nan", c.NotEqual(c).ToBits())
	println("or", a.Greater(b).Or(mask).ToBits())

	show("shuffle-low", a.ConcatPermuteScalarsGrouped(0, 1, 4, 5, b))
	show("shuffle-high", a.ConcatPermuteScalarsGrouped(2, 3, 6, 7, b))
	show("shuffle-even", a.ConcatPermuteScalarsGrouped(0, 2, 4, 6, b))
	show("shuffle-odd", a.ConcatPermuteScalarsGrouped(1, 3, 5, 7, b))
	show("shuffle-mixed", a.ConcatPermuteScalarsGrouped(3, 1, 6, 4, b))

	show("perm-low", a.ConcatPermute128Scalars(0, 2, b))
	show("perm-high", a.ConcatPermute128Scalars(1, 3, b))
	show("perm-cross", a.ConcatPermute128Scalars(3, 0, b))

	// The vector in a heap value and at a misaligned address must not fault.
	value := new(layout)
	value.before = 11
	value.vector = a.Add(b)
	value.after = 22
	show("field", value.vector)
	println("around", value.before, value.after)

	raw := make([]float32, 16)
	misaligned := (*[8]float32)(unsafe.Pointer(&raw[1]))
	storeAt(misaligned, a)
	show("misaligned", loadAt(misaligned))
}

// storeAt and loadAt stop LLVM from folding the store into the load.
//
//go:noinline
func storeAt(p *[8]float32, v archsimd.Float32x8) { v.StoreArray(p) }

//go:noinline
func loadAt(p *[8]float32) archsimd.Float32x8 { return archsimd.LoadFloat32x8Array(p) }
