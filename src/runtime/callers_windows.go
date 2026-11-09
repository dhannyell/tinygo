//go:build windows && amd64

package runtime

// Callers walks the stack with the Win64 unwind info (.pdata). Unlike
// RtlCaptureStackBackTrace, this also works on goroutine stacks, which are
// outside the thread stack bounds.

import "unsafe"

// win64Context is the Win64 CONTEXT record: 1232 bytes, 16-byte aligned.
type win64Context [1232 / 8]uint64

const win64ContextRip = 0xF8 / 8

//export RtlCaptureContext
func _RtlCaptureContext(ctx *win64Context)

//export RtlLookupFunctionEntry
func _RtlLookupFunctionEntry(pc uintptr, imageBase *uintptr, history unsafe.Pointer) unsafe.Pointer

//export RtlVirtualUnwind
func _RtlVirtualUnwind(handlerType uint32, imageBase, pc uintptr, entry unsafe.Pointer, ctx *win64Context, handlerData *unsafe.Pointer, establisherFrame *uintptr, contextPointers unsafe.Pointer) unsafe.Pointer

//go:noinline
func Callers(skip int, pc []uintptr) int {
	var buf [len(win64Context{})*8 + 15]byte
	ctx := (*win64Context)(unsafe.Pointer((uintptr(unsafe.Pointer(&buf)) + 15) &^ 15))
	_RtlCaptureContext(ctx)
	n := 0
	for ctx[win64ContextRip] != 0 && n < len(pc) {
		rip := uintptr(ctx[win64ContextRip])
		if skip > 0 {
			skip--
		} else {
			pc[n] = rip
			n++
		}
		var imageBase uintptr
		entry := _RtlLookupFunctionEntry(rip, &imageBase, nil)
		if entry == nil {
			// No unwind info, as in tinygo_startTask: the stack ends here.
			break
		}
		var handlerData unsafe.Pointer
		var establisherFrame uintptr
		_RtlVirtualUnwind(0, imageBase, rip, entry, ctx, &handlerData, &establisherFrame, nil)
	}
	return n
}
