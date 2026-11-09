//go:build gc.boehm && tinygo.wasm.threads

package runtime

import "unsafe"

// parallelAlloc is set only by the main thread, and only while workers wait.
var parallelAlloc bool

const (
	allocClassCount    = 12
	allocClassMax      = 1024
	maxAllocThreads    = 16
	parallelMinCollect = 32 << 20
)

var allocClasses = [allocClassCount]uintptr{16, 32, 48, 64, 96, 128, 192, 256, 384, 512, 768, 1024}

// threadAllocCache holds one free list per class. The padding keeps the lists
// of two threads off one cache line.
type threadAllocCache struct {
	heads [allocClassCount]unsafe.Pointer
	_     [80]byte
}

// Index 0 is the main thread. The GC sees these globals as roots.
var allocCaches [maxAllocThreads + 1]threadAllocCache

type threadStack struct{ lo, hi uintptr }

var (
	threadStacks     [maxAllocThreads]threadStack
	threadStackCount int
)

// RegisterThreadStack names a worker by its stack range. Call it on the main
// thread before the worker starts, because wasm has no thread id.
func RegisterThreadStack(lo, hi uintptr) {
	threadStacks[threadStackCount] = threadStack{lo, hi}
	threadStackCount++
}

func allocThreadIndex() int {
	sp := getCurrentStackPointer()
	for i := 0; i < threadStackCount; i++ {
		if sp >= threadStacks[i].lo && sp < threadStacks[i].hi {
			return i + 1
		}
	}
	return 0
}

// allocCached pops from the list of this thread and locks only to refill it.
// The refill has no pointer-free variant, so the GC scans these objects.
func allocCached(size uintptr) unsafe.Pointer {
	class := 0
	for allocClasses[class] < size {
		class++
	}
	classSize := allocClasses[class]
	cache := &allocCaches[allocThreadIndex()]
	ptr := cache.heads[class]
	if ptr == nil {
		ptr = libgc_malloc_many_split(classSize)
		if ptr == nil {
			runtimeFatal("gc: out of memory")
			return nil
		}
	}
	cache.heads[class] = *(*unsafe.Pointer)(ptr)
	memzero(ptr, classSize)
	return ptr
}

// ParallelAllocBegin makes alloc safe on other threads until ParallelAllocEnd.
// The GC stays off, because it cannot see the stacks of those threads.
func ParallelAllocBegin() {
	libgc_disable()
	parallelAlloc = true
}

// ParallelAllocEnd must run on the main thread after all workers have joined.
// It collects if enough was allocated, because the fork ran with the GC off.
func ParallelAllocEnd() {
	parallelAlloc = false
	libgc_enable()
	if since := libgc_get_bytes_since_gc(); since >= parallelMinCollect && since >= libgc_get_heap_size()/3 {
		GC()
	}
}

//export tinygo_malloc_many_split
func libgc_malloc_many_split(uintptr) unsafe.Pointer

//export tinygo_runtime_alloc_lock
func gcAllocLock()

//export tinygo_runtime_alloc_unlock
func gcAllocUnlock()

//export GC_disable
func libgc_disable()

//export GC_enable
func libgc_enable()

//export GC_get_bytes_since_gc
func libgc_get_bytes_since_gc() uintptr

//export GC_get_heap_size
func libgc_get_heap_size() uintptr
