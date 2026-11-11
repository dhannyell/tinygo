package main

// Workers keep lists only on their stacks while the main thread allocates and
// collects. Each worker stops at an allocation, in ThreadWait and in Gosched.
// A missed worker stack frees a node in use, and a reused node breaks a sum.

import (
	"runtime"
	"sync/atomic"
	"syscall/js"
	"time"
	"unsafe"
)

const (
	workers   = 4
	rounds    = 12
	length    = 20000
	stackSize = 1 << 20
	want      = length * (length - 1) / 2
)

type node struct {
	next *node
	val  int
}

var (
	stacks [workers][]byte
	tls    [workers][8]uint64
	ready  atomic.Int32
	done   atomic.Int32
	bad    atomic.Int32
	tick   uint32 // the main thread changes it, and parked workers wake
	never  uint32
	sink   *node
)

//go:wasmimport threads notify
func hostNotify(addr uintptr)

func build(n int) *node {
	var head *node
	for i := range n {
		head = &node{next: head, val: i}
	}
	return head
}

// sum stops after limit nodes: a freed node can make a cycle.
func sum(head *node, limit int) int {
	total := 0
	for n := head; n != nil && limit > 0; n = n.next {
		total += n.val
		limit--
	}
	return total
}

// work holds a list and a set of large blocks across every kind of safepoint.
func work() {
	for range rounds {
		head := build(length)
		var blocks [][]byte
		for i := range 64 {
			b := make([]byte, 4096)
			b[0], b[4095] = byte(i), byte(i)
			blocks = append(blocks, b)
		}
		runtime.ThreadWait(&tick, atomic.LoadUint32(&tick))
		for range 2000 {
			runtime.Gosched()
		}
		if sum(head, length+1) != want {
			bad.Add(1)
		}
		for i, b := range blocks {
			if b[0] != byte(i) || b[4095] != byte(i) {
				bad.Add(1)
			}
		}
	}
}

//export threadsMain
func threadsMain(index int32) {
	runtime.ThreadEnter()
	ready.Add(1)
	work()
	done.Add(1)
	for {
		runtime.ThreadWait(&never, 0)
	}
}

func main() {
	for i := range workers {
		stacks[i] = make([]byte, stackSize)
		low := uintptr(unsafe.Pointer(&stacks[i][0]))
		runtime.RegisterThreadStack(low, low+stackSize)
		js.Global().Call("__threadsSpawn", i, low+stackSize, uintptr(unsafe.Pointer(&tls[i])))
	}
	// A Worker starts only while the main thread is in the event loop.
	for ready.Load() != workers {
		time.Sleep(time.Millisecond)
	}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for count := 0; done.Load() != workers; count++ {
		// The same size as a list node, so that a freed node is reused.
		sink = &node{val: -1}
		if count%4096 == 0 {
			atomic.AddUint32(&tick, 1)
			hostNotify(uintptr(unsafe.Pointer(&tick)))
		}
	}
	runtime.ReadMemStats(&after)

	println("wasm threads gc: bad walks:", bad.Load())
	println("collections while workers ran:", after.NumGC-before.NumGC >= 4)
	println("heap stable:", after.HeapSys < 256<<20)
	js.Global().Call("__done")
}
