package main

// Walkers on other threads keep their lists only in locals and registers,
// while allocators make the GC run. A missed root frees a node in use.

import (
	"runtime"
	"sync"
	"sync/atomic"
)

type node struct {
	next *node
	val  int
}

var sink *node

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

func main() {
	const walkers = 4
	const length = 20000
	const want = length * (length - 1) / 2

	var mu sync.Mutex
	var bad atomic.Int32
	var done atomic.Bool
	var walking, allocating sync.WaitGroup
	for range walkers {
		walking.Add(1)
		go func() {
			for range 5 {
				head := build(length)
				// The walk does not allocate or call, so a walker that the GC
				// stops can hold the list only in a register. A defer would
				// keep it on the stack.
				for range 100 {
					if sum(head, length+1) != want {
						bad.Add(1)
					}
				}
			}
			walking.Done()
		}()
	}
	for range walkers {
		allocating.Add(1)
		go func() {
			defer allocating.Done()
			for !done.Load() {
				// The same size as a list node, so that a freed node is reused.
				b := &node{val: -1}
				mu.Lock()
				sink = b
				mu.Unlock()
			}
		}()
	}
	walking.Wait()
	done.Store(true)
	allocating.Wait()
	println("threads gc: bad walks:", bad.Load())
	println("gomaxprocs is numcpu:", runtime.GOMAXPROCS(0) == runtime.NumCPU())
}
