//go:build (gc.conservative || gc.custom || gc.precise || scheduler.threads) && !gc.boehm

package runtime

import "unsafe"

// markRootWord marks the one root word at addr.
func markRootWord(addr uintptr) {
	markRoots(addr, addr+unsafe.Sizeof(addr))
}
