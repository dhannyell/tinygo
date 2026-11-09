//go:build gc.boehm

// This is the Boehm-Demers-Weiser conservative garbage collector, integrated
// into TinyGo.
//
// Note that we use a special way of dealing with threads:
//   * All calls to the bdwgc library are serialized using locks.
//   * When the bdwgc library wants to push GC roots, all other threads that are
//     running are stopped.
//   * After returning from a bdwgc library call, the caller checks whether
//     other threads were stopped (meaning a GC cycle happened) and resumes the
//     world.
// This is not exactly the most efficient way to do this. We can likely speed
// things up by using bdwgc-native wrappers for starting/stopping threads (and
// also to resume the world while sweeping). Also, thread local allocation might
// help. But we don't do any of these right now, it is left as a possible future
// improvement.

package runtime

import (
	"internal/gclayout"
	"internal/task"
	"unsafe"
)

const needsStaticHeap = false

var gcLock task.PMutex

func initHeap() {
	libgc_init()

	// Call GC_set_push_other_roots(gcCallback) in C because of function
	// signature differences that do matter in WebAssembly.
	gcInit()
	gcInitFinalizers()
}

//export tinygo_runtime_bdwgc_init
func gcInit()

//export tinygo_runtime_bdwgc_init_finalizers
func gcInitFinalizers()

//export tinygo_runtime_bdwgc_set_finalizer
// The arguments are uintptr: an unsafe.Pointer passed to an external function
// does not count as an escape, so box would stay on the stack.
func gcSetFinalizer(obj, box uintptr)

//export GC_should_invoke_finalizers
func libgc_should_invoke_finalizers() int32

//export GC_invoke_finalizers
func libgc_invoke_finalizers() int32

//export tinygo_runtime_bdwgc_callback
func gcCallback() {
	// Mark globals and all stacks, and stop the world if we're using threading.
	gcMarkReachable()
}

func markRoots(start, end uintptr) {
	libgc_push_all(start, end)
}

// markRootWord marks the one root word at addr. It scans the word now, because
// one GC_push_all entry per word can overflow the mark stack.
func markRootWord(addr uintptr) {
	libgc_push_all_eager(addr, addr+unsafe.Sizeof(addr))
}

func markCurrentGoroutineStack(sp uintptr) {
	// Only mark the area of the stack that is currently in use.
	// (This doesn't work for other goroutines, but at least it doesn't keep
	// more pointers alive than needed on the current stack).
	base := libgc_base(sp)
	if base == 0 { // && asserts
		runtimeFatal("goroutine stack not in a heap allocation?")
	}
	stackBottom := base + libgc_size(base)
	libgc_push_all_stack(sp, stackBottom)
}

//go:noinline
func alloc(size uintptr, layout unsafe.Pointer) unsafe.Pointer {
	if size == 0 {
		return alloc_zero(size, layout)
	}

	parallel := parallelAlloc
	if parallel {
		if ParallelAllocCache && size <= allocClassMax {
			return allocCached(size)
		}
		gcAllocLock()
		bigAllocs++
		bigAllocBytes += uint64(size)
	}
	gcLock.Lock()
	var ptr unsafe.Pointer
	var needsZero bool
	if layout == gclayout.NoPtrs.AsPtr() {
		// This object is entirely pointer free, for example make([]int, ...).
		// Make sure the GC knows this so it doesn't scan the object
		// unnecessarily to improve performance.
		ptr = libgc_malloc_atomic(size)
		// Memory returned from libgc_malloc_atomic has not been zeroed so we
		// have to do that manually.
		needsZero = true
	} else {
		// Large objects are scanned only at their pointer words.
		if size >= typedMinSize {
			ptr = allocTyped(size, layout)
		}
		if ptr == nil {
			ptr = libgc_malloc(size)
		}
		// Memory returned from bdwgc has already been zeroed, so nothing to do
		// here.
	}
	gcResumeWorld()
	gcLock.Unlock()
	if parallel {
		gcAllocUnlock()
	}
	if ptr == nil {
		runtimeFatal("gc: out of memory")
		return nil
	}
	if needsZero {
		memzero(ptr, size)
	}
	if !parallel {
		drainFinalizers()
	}

	return ptr
}

var (
	bigAllocs     uint64
	bigAllocBytes uint64
)

//export tinygo_runtime_alloc_lock_takes
func gcAllocLockTakes() uint64

//export tinygo_runtime_alloc_lock_spins
func gcAllocLockSpins() uint64

// ParallelAllocStats reads the counters. Call it only while no worker runs.
func ParallelAllocStats() (lockTakes, lockSpins, bigCount, bigBytes uint64) {
	return gcAllocLockTakes(), gcAllocLockSpins(), bigAllocs, bigAllocBytes
}

// parallelAlloc is set only by the main thread, and only while workers wait.
var parallelAlloc bool

// ParallelAllocCache selects the per-thread caches. Off, every alloc takes the lock.
var ParallelAllocCache = true

const (
	allocClassCount = 12
	allocClassMax   = 1024
	maxAllocThreads = 16
)

var allocClasses = [allocClassCount]uintptr{16, 32, 48, 64, 96, 128, 192, 256, 384, 512, 768, 1024}

// threadAllocCache holds one free list per class. The padding keeps the lists
// of two threads off one cache line.
type threadAllocCache struct {
	heads [allocClassCount]unsafe.Pointer
	_     [80]byte
}

// Index 0 is the main thread. The caches stay filled between forks: they are
// globals, so the GC sees them as roots and keeps at most one batch per class.
var allocCaches [maxAllocThreads + 1]threadAllocCache

type threadStack struct{ lo, hi uintptr }

var (
	threadStacks     [maxAllocThreads]threadStack
	threadStackCount int
)

// RegisterThreadStack names a worker by its stack range. wasm has no thread id,
// and a worker's stack pointer always stays in its own range.
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

//export GC_malloc_many
func libgc_malloc_many(uintptr) unsafe.Pointer

//export tinygo_malloc_many_split
func libgc_malloc_many_split(uintptr) unsafe.Pointer

// ParallelAllocSplit holds the lock only for shared state during a refill.
var ParallelAllocSplit = true

// allocCached pops from this thread's list and takes the lock only to refill it.
// GC_malloc_many has no pointer-free variant, so these objects are always scanned.
func allocCached(size uintptr) unsafe.Pointer {
	class := 0
	for allocClasses[class] < size {
		class++
	}
	classSize := allocClasses[class]
	cache := &allocCaches[allocThreadIndex()]
	ptr := cache.heads[class]
	if ptr == nil {
		if ParallelAllocSplit {
			ptr = libgc_malloc_many_split(classSize)
		} else {
			gcAllocLock()
			ptr = libgc_malloc_many(classSize)
			gcAllocUnlock()
		}
		if ptr == nil {
			runtimeFatal("gc: out of memory")
			return nil
		}
	}
	cache.heads[class] = *(*unsafe.Pointer)(ptr)
	memzero(ptr, classSize)
	return ptr
}

//export tinygo_runtime_alloc_lock
func gcAllocLock()

//export tinygo_runtime_alloc_unlock
func gcAllocUnlock()

//export GC_disable
func libgc_disable()

//export GC_enable
func libgc_enable()

// ParallelAllocBegin makes alloc safe on other threads until ParallelAllocEnd.
// The GC stays off, because it cannot see the stacks of those threads.
func ParallelAllocBegin() {
	libgc_disable()
	parallelAlloc = true
}

// ParallelAllocEnd must run on the main thread after all workers have joined.
// It also collects when enough was allocated: the fork allocated with the GC off,
// so no allocation would trigger the collection later.
func ParallelAllocEnd() {
	parallelAlloc = false
	libgc_enable()
	if since := libgc_get_bytes_since_gc(); since >= parallelMinCollect && since >= libgc_get_heap_size()/3 {
		GC()
	}
}

const parallelMinCollect = 32 << 20

//export GC_get_bytes_since_gc
func libgc_get_bytes_since_gc() uintptr

//export GC_get_heap_size
func libgc_get_heap_size() uintptr

// AllocLockStress adds n to the counter under the alloc lock, to test that lock.
func AllocLockStress(counter *int32, n int32) {
	for i := int32(0); i < n; i++ {
		gcAllocLock()
		*counter = *counter + 1
		gcAllocUnlock()
	}
}

func free(ptr unsafe.Pointer) {
	libgc_free(ptr)
}

func GC() {
	gcLock.Lock()
	libgc_gcollect()
	gcResumeWorld()
	gcLock.Unlock()
	drainFinalizers()
}

// This should be stack-allocated, but we don't currently have a good way of
// ensuring that happens.
var gcMemStats libgc_prof_stats

func ReadMemStats(m *MemStats) {
	gcLock.Lock()

	libgc_get_prof_stats(&gcMemStats, unsafe.Sizeof(gcMemStats))

	// Fill in MemStats as well as we can, given the information that bdwgc
	// provides to us.
	m.HeapIdle = uint64(gcMemStats.free_bytes_full - gcMemStats.unmapped_bytes)
	m.HeapInuse = uint64(gcMemStats.heapsize_full - gcMemStats.unmapped_bytes)
	m.HeapReleased = uint64(gcMemStats.unmapped_bytes)
	m.HeapSys = uint64(m.HeapInuse + m.HeapIdle)
	m.GCSys = 0 // not provided by bdwgc
	m.TotalAlloc = uint64(gcMemStats.allocd_bytes_before_gc + gcMemStats.bytes_allocd_since_gc)
	m.Mallocs = 0 // not provided by bdwgc
	m.Frees = 0   // not provided by bdwgc
	m.Sys = uint64(gcMemStats.obtained_from_os_bytes)
	m.NumGC = uint32(gcMemStats.gc_no)

	gcLock.Unlock()
}

func setHeapEnd(newHeapEnd uintptr) {
	runtimeFatal("gc: did not expect setHeapEnd call")
}

func SetFinalizer(obj interface{}, finalizer interface{}) {
	objPtr := (*_interface)(unsafe.Pointer(&obj)).value
	if objPtr == nil {
		return
	}
	// The bdwgc finalizer table scans the client data, so the box keeps the
	// closure alive.
	var box unsafe.Pointer
	if finalizer != nil {
		fn := new(interface{})
		*fn = finalizer
		box = unsafe.Pointer(fn)
	}
	gcLock.Lock()
	gcSetFinalizer(uintptr(objPtr), uintptr(box))
	gcResumeWorld()
	gcLock.Unlock()
}

//export tinygo_runtime_bdwgc_finalizer
func gcFinalizerCallback(obj, box unsafe.Pointer) {
	fn := *(*interface{})(box)
	fnBox := (*_interface)(unsafe.Pointer(&fn)).value
	f := *(*func(unsafe.Pointer))(fnBox)
	f(obj)
}

var finalizerDraining bool

// drainFinalizers runs the ready finalizers on the allocating goroutine.
// A finalizer that blocks on a lock its caller holds would deadlock.
func drainFinalizers() {
	if finalizerDraining || libgc_should_invoke_finalizers() == 0 {
		return
	}
	finalizerDraining = true
	libgc_invoke_finalizers()
	finalizerDraining = false
}

//export GC_init
func libgc_init()

//export GC_malloc
func libgc_malloc(uintptr) unsafe.Pointer

//export GC_malloc_atomic
func libgc_malloc_atomic(uintptr) unsafe.Pointer

//export GC_free
func libgc_free(unsafe.Pointer)

//export GC_base
func libgc_base(ptr uintptr) uintptr

//export GC_size
func libgc_size(ptr uintptr) uintptr

//export GC_push_all
func libgc_push_all(bottom, top uintptr)

//export GC_push_all_stack
func libgc_push_all_stack(bottom, top uintptr)

//export GC_push_all_eager
func libgc_push_all_eager(bottom, top uintptr)

//export GC_gcollect
func libgc_gcollect()

//export GC_get_prof_stats
func libgc_get_prof_stats(*libgc_prof_stats, uintptr) uintptr

//export GC_set_push_other_roots
func libgc_set_push_other_roots(unsafe.Pointer)

type libgc_prof_stats struct {
	heapsize_full             uintptr
	free_bytes_full           uintptr
	unmapped_bytes            uintptr
	bytes_allocd_since_gc     uintptr
	allocd_bytes_before_gc    uintptr
	non_gc_bytes              uintptr
	gc_no                     uintptr
	markers_m1                uintptr
	bytes_reclaimed_since_gc  uintptr
	reclaimed_bytes_before_gc uintptr
	expl_freed_bytes_since_gc uintptr
	obtained_from_os_bytes    uintptr
}

// AllocRefillBench makes n batches and returns the object count, to time one refill.
func AllocRefillBench(n int32, size uintptr) int32 {
	count := int32(0)
	for i := int32(0); i < n; i++ {
		for p := libgc_malloc_many(size); p != nil; p = *(*unsafe.Pointer)(p) {
			count++
		}
	}
	return count
}
