//go:build gc.conservative || gc.custom || gc.precise || gc.boehm || scheduler.threads

package runtime

import (
	"math/bits"
	"unsafe"
)

// preciseGlobal is a large global that the GC scans only at its pointer words.
// The layout has the format of a heap object layout and repeats over the
// global.
type preciseGlobal struct {
	start  uintptr
	size   uintptr
	layout uintptr
}

// The compiler fills this table, see transform.PreciseGlobals.
//
//go:extern runtime.preciseGlobals
var preciseGlobals struct {
	n       uintptr
	entries *preciseGlobal
}

var preciseGlobalsSorted bool

// scanGlobals marks the globals that findGlobals finds. It marks the globals
// of the table only at their pointer words.
func scanGlobals() {
	if preciseGlobals.n == 0 {
		findGlobals(markRoots)
		return
	}
	if !preciseGlobalsSorted {
		sortPreciseGlobals()
		preciseGlobalsSorted = true
	}
	findGlobals(scanGlobalRange)
}

func scanGlobalRange(start, end uintptr) {
	for i := range preciseGlobals.n {
		g := preciseGlobalAt(i)
		if g.start < start || g.start+g.size > end {
			continue
		}
		if start < g.start {
			markRoots(start, g.start)
		}
		scanPreciseGlobal(g)
		start = g.start + g.size
	}
	if start < end {
		markRoots(start, end)
	}
}

func preciseGlobalAt(i uintptr) *preciseGlobal {
	return (*preciseGlobal)(unsafe.Add(unsafe.Pointer(preciseGlobals.entries), i*unsafe.Sizeof(preciseGlobal{})))
}

// sortPreciseGlobals sorts the table by address. It does not allocate, because
// it runs inside the GC.
func sortPreciseGlobals() {
	for i := uintptr(1); i < preciseGlobals.n; i++ {
		for j := i; j > 0 && preciseGlobalAt(j).start < preciseGlobalAt(j-1).start; j-- {
			*preciseGlobalAt(j), *preciseGlobalAt(j - 1) = *preciseGlobalAt(j - 1), *preciseGlobalAt(j)
		}
	}
}

func scanPreciseGlobal(g *preciseGlobal) {
	const wordSize = unsafe.Sizeof(uintptr(0))
	const sizeBits = 4 + wordSize/4
	var words, mask uintptr
	var bitmap unsafe.Pointer
	if g.layout&1 != 0 {
		words = g.layout >> 1 & (1<<sizeBits - 1)
		mask = g.layout >> (1 + sizeBits)
		if mask == 0 {
			return // no pointers
		}
	} else if g.layout != 0 {
		words = *(*uintptr)(unsafe.Pointer(g.layout))
		bitmap = unsafe.Add(unsafe.Pointer(g.layout), wordSize)
	}
	period := words * wordSize
	if period == 0 || g.size%period != 0 {
		markRoots(g.start, g.start+g.size) // unknown layout: stay conservative
		return
	}
	for elem := g.start; elem < g.start+g.size; elem += period {
		if bitmap == nil {
			markPointerWords(elem, mask)
			continue
		}
		for i := range (words + 7) / 8 {
			markPointerWords(elem+i*8*wordSize, uintptr(*(*byte)(unsafe.Add(bitmap, i))))
		}
	}
}

// markPointerWords marks the non-nil words at base whose bits are set in mask.
func markPointerWords(base, mask uintptr) {
	const wordSize = unsafe.Sizeof(uintptr(0))
	for mask != 0 {
		addr := base + uintptr(bits.TrailingZeros(uint(mask)))*wordSize
		if *(*uintptr)(unsafe.Pointer(addr)) != 0 {
			markRootWord(addr)
		}
		mask &= mask - 1
	}
}
