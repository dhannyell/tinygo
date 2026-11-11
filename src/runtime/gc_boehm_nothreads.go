//go:build gc.boehm && !tinygo.wasm.threads

package runtime

import "unsafe"

const (
	parallelAlloc = false
	allocClassMax = 0
)

func allocCached(size uintptr) unsafe.Pointer { return nil }

func gcAllocLock() {}

func gcAllocUnlock() {}

func gcAllocPoint() {}

func markThreadStacks() {}

func gcStopWorld() bool { return true }

func gcStartWorld() {}
