//go:build gc.boehm

package runtime

import "unsafe"

// Objects from this size on use the pointer bitmap of their layout, so the GC
// skips the words that hold no pointers. Smaller objects stay conservative.
const typedMinSize = 4096

// Elements of at least this many words make bdwgc use a leaf descriptor. With
// smaller elements it may need a complex descriptor, which GC_NO_FINALIZATION
// turns into an out-of-memory error.
const typedMinElemWords = 4

// Layout format of internal/gclayout: pppp...p_sss...s_1.
const typedSizeBits = 4 + unsafe.Sizeof(uintptr(0))/4

var typedDescrs [64]struct{ layout, descr uintptr }

// typedBitmap is the bitmap argument of GC_make_descriptor. It is a global
// because a local would escape to the C call and recurse into alloc.
var typedBitmap uintptr

// allocTyped allocates an array of layout elements. It returns nil when the
// layout cannot describe the object. The caller holds gcLock.
func allocTyped(size uintptr, layout unsafe.Pointer) unsafe.Pointer {
	l := uintptr(layout)
	if l&1 == 0 {
		return nil // nil, or a layout stored in a global
	}
	words := (l >> 1) & (1<<typedSizeBits - 1)
	if words < typedMinElemWords {
		return nil
	}
	elem := words * unsafe.Sizeof(uintptr(0))
	if size%elem != 0 {
		return nil
	}
	descr, ok := typedDescriptor(l)
	if !ok {
		return nil
	}
	return libgc_calloc_explicitly_typed(size/elem, elem, descr)
}

// typedDescriptor returns the cached bdwgc descriptor of an inline layout.
func typedDescriptor(layout uintptr) (uintptr, bool) {
	i := (layout ^ layout>>16) % uintptr(len(typedDescrs))
	for range len(typedDescrs) {
		e := &typedDescrs[i]
		if e.layout == layout {
			return e.descr, true
		}
		if e.layout == 0 {
			typedBitmap = layout >> (1 + typedSizeBits)
			nbits := uintptr(0)
			for m := typedBitmap; m != 0; m >>= 1 {
				nbits++
			}
			e.descr = libgc_make_descriptor(&typedBitmap, nbits)
			e.layout = layout
			return e.descr, true
		}
		i = (i + 1) % uintptr(len(typedDescrs))
	}
	return 0, false // table full: stay conservative
}

//export GC_make_descriptor
func libgc_make_descriptor(bitmap *uintptr, nbits uintptr) uintptr

//export GC_calloc_explicitly_typed
func libgc_calloc_explicitly_typed(n, elemSize, descr uintptr) unsafe.Pointer
