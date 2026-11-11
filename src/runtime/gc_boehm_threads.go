//go:build gc.boehm && tinygo.wasm.threads

package runtime

import (
	"sync/atomic"
	"unsafe"
)

// parallelAlloc is set when the first thread registers. From then on every
// thread can allocate, and only the main thread collects.
var parallelAlloc bool

const (
	allocClassCount = 12
	allocClassMax   = 1024
	maxAllocThreads = 16
	// threadsMinCollect is the smallest allocation between two collections.
	threadsMinCollect = 8 << 20
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

// Thread states. Only a worker leaves running, and only the main thread
// changes parked to held and back.
const (
	threadParked = iota // in ThreadWait, or not yet in ThreadEnter
	threadRunning
	threadHeld    // parked while the main thread collects
	threadStopped // at a safepoint while the main thread collects
)

var (
	threadState [maxAllocThreads]uint32
	// threadSP is the stack pointer at the last park or stop.
	threadSP      [maxAllocThreads]uintptr
	gcStopRequest uint32
	gcEpoch       uint32
)

// RegisterThreadStack names a worker by its stack range. Call it on the main
// thread before the worker starts, because wasm has no thread id.
func RegisterThreadStack(lo, hi uintptr) {
	if threadStackCount == maxAllocThreads {
		runtimeFatal("too many threads")
	}
	if threadStackCount == 0 {
		// Boehm cannot stop a worker, so it must not collect by itself.
		libgc_disable()
		parallelAlloc = true
	}
	threadStacks[threadStackCount] = threadStack{lo, hi}
	threadStackCount++
}

// ThreadEnter must be the first call on a new worker, before it uses the heap.
func ThreadEnter() {
	threadUnpark(workerIndex())
}

// ThreadWait sleeps until a notify on addr if *addr is value. Only a worker
// can call it. The main thread can collect while the worker sleeps.
func ThreadWait(addr *uint32, value uint32) {
	i := workerIndex()
	threadSP[i] = getCurrentStackPointer()
	atomic.StoreUint32(&threadState[i], threadParked)
	wasmWait32(addr, value, -1)
	threadUnpark(i)
}

func workerIndex() int {
	index := allocThreadIndex()
	if index == 0 {
		runtimeFatal("the main thread cannot wait")
	}
	return index - 1
}

// threadUnpark waits while the main thread holds this worker.
func threadUnpark(i int) {
	for !atomic.CompareAndSwapUint32(&threadState[i], threadParked, threadRunning) {
		epoch := atomic.LoadUint32(&gcEpoch)
		if atomic.LoadUint32(&threadState[i]) == threadHeld {
			wasmWait32(&gcEpoch, epoch, -1)
		}
	}
}

// gcSafepoint stops worker i while the main thread collects. The caller must
// hold no lock and no object from its free lists.
func gcSafepoint(i int) {
	if atomic.LoadUint32(&gcStopRequest) == 0 {
		return
	}
	epoch := atomic.LoadUint32(&gcEpoch)
	threadSP[i] = getCurrentStackPointer()
	atomic.StoreUint32(&threadState[i], threadStopped)
	for atomic.LoadUint32(&gcEpoch) == epoch {
		wasmWait32(&gcEpoch, epoch, -1)
	}
}

// workerYield makes Gosched a safepoint on a worker, which has no scheduler.
func workerYield() bool {
	index := allocThreadIndex()
	if index == 0 {
		return false
	}
	gcSafepoint(index - 1)
	return true
}

// gcAllocPoint runs before an allocation that does not use the free lists.
func gcAllocPoint() {
	if index := allocThreadIndex(); index != 0 {
		gcSafepoint(index - 1)
	} else {
		gcCollectIfDue()
	}
}

// gcCollectIfDue applies the Boehm policy that the disabled GC cannot apply.
func gcCollectIfDue() {
	if since := libgc_get_bytes_since_gc(); since >= threadsMinCollect && since >= libgc_get_heap_size()/3 {
		GC()
	}
}

// gcStopWorld returns false on a worker, which stops at a safepoint instead.
// The main thread waits until every worker is parked or at a safepoint, so a
// worker that spins without a safepoint delays it here.
func gcStopWorld() bool {
	if !parallelAlloc {
		return true
	}
	if index := allocThreadIndex(); index != 0 {
		gcSafepoint(index - 1)
		return false
	}
	atomic.StoreUint32(&gcStopRequest, 1)
	for i := range threadStackCount {
		for {
			state := atomic.LoadUint32(&threadState[i])
			if state == threadHeld || state == threadStopped {
				break
			}
			if state == threadParked && atomic.CompareAndSwapUint32(&threadState[i], threadParked, threadHeld) {
				break
			}
		}
	}
	// No object on a free list is in use, so the GC can free them all.
	for i := range allocCaches {
		allocCaches[i].heads = [allocClassCount]unsafe.Pointer{}
	}
	libgc_enable()
	return true
}

// gcStartWorld lets the workers run again after gcStopWorld.
func gcStartWorld() {
	if !parallelAlloc {
		return
	}
	libgc_disable()
	atomic.StoreUint32(&gcStopRequest, 0)
	for i := range threadStackCount {
		if atomic.LoadUint32(&threadState[i]) == threadHeld {
			atomic.StoreUint32(&threadState[i], threadParked)
		} else {
			atomic.StoreUint32(&threadState[i], threadRunning)
		}
	}
	atomic.AddUint32(&gcEpoch, 1)
	wasmNotify(&gcEpoch, ^uint32(0))
}

// markThreadStacks marks the used part of each worker stack. The workers do
// not run during the mark, so their frames do not change.
func markThreadStacks() {
	for i := range threadStackCount {
		state := atomic.LoadUint32(&threadState[i])
		if (state == threadHeld || state == threadStopped) && threadSP[i] != 0 {
			markRoots(threadSP[i], threadStacks[i].hi)
		}
	}
}

func allocThreadIndex() int {
	sp := getCurrentStackPointer()
	for i := range threadStackCount {
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
	index := allocThreadIndex()
	if index != 0 {
		gcSafepoint(index - 1)
	}
	cache := &allocCaches[index]
	ptr := cache.heads[class]
	if ptr == nil {
		if index == 0 {
			gcCollectIfDue()
		}
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

//export llvm.wasm.memory.atomic.wait32
func wasmWait32(addr *uint32, value uint32, timeout int64) int32

//export llvm.wasm.memory.atomic.notify
func wasmNotify(addr *uint32, count uint32) uint32

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
