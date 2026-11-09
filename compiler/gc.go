package compiler

// This file provides IR transformations necessary for precise and portable
// garbage collectors.

import (
	"go/token"
	"slices"

	"github.com/tinygo-org/tinygo/compiler/llvmutil"
	"golang.org/x/tools/go/ssa"
	"tinygo.org/x/go-llvm"
)

// Globals from this size on get a pointer layout, so the GC can scan only
// their pointer words.
const globalLayoutMinSize = 4096

// addGlobalLayout stores the size and pointer layout of a large global in the
// companion global "<name>$gclayout". transform.PreciseGlobals reads it.
func (c *compilerContext) addGlobalLayout(g *ssa.Global, global llvm.Value) {
	typ := global.GlobalValueType()
	size := c.targetData.TypeAllocSize(typ)
	if size < globalLayoutMinSize || global.IsThreadLocal() {
		return
	}
	init := c.ctx.ConstStruct([]llvm.Value{
		llvm.ConstInt(c.uintptrType, size, false),
		c.createObjectLayout(typ, g.Pos()),
	}, false)
	companion := llvm.AddGlobal(c.mod, init.Type(), global.Name()+"$gclayout")
	companion.SetInitializer(init)
	companion.SetGlobalConstant(true)
	companion.SetLinkage(llvm.InternalLinkage)
	// Keep the companion until the transform, even if nothing refers to it.
	llvmutil.AppendToGlobal(c.mod, "llvm.compiler.used", companion)
}

// Heap-allocate a buffer of the given size. This will typically call
// runtime.alloc.
func (b *builder) createAlloc(sizeValue, layoutValue llvm.Value, align int, comment string) llvm.Value {
	// Normally allocate using "runtime.alloc", but use "runtime.alloc_noheap"
	// if the //go:noheap pragma is used.
	allocFunc := "alloc"
	if b.info.noheap {
		allocFunc = "alloc_noheap"
	}

	// Allocs that don't allocate anything can return an architecture-specific
	// sentinel value.
	if !sizeValue.IsAConstantInt().IsNil() && sizeValue.ZExtValue() == 0 {
		allocFunc = "alloc_zero"
	}

	// Make the runtime call.
	call := b.createRuntimeCall(allocFunc, []llvm.Value{sizeValue, layoutValue}, comment)
	if align == 0 || align&(align-1) != 0 {
		panic("invalid alignment")
	}
	call.AddCallSiteAttribute(0, b.ctx.CreateEnumAttribute(llvm.AttributeKindID("align"), uint64(align)))

	return call
}

// trackExpr inserts pointer tracking intrinsics for the GC if the expression is
// one of the expressions that need this.
func (b *builder) trackExpr(expr ssa.Value, value llvm.Value) {
	// There are uses of this expression, Make sure the pointers
	// are tracked during GC.
	switch expr := expr.(type) {
	case *ssa.Alloc, *ssa.MakeChan, *ssa.MakeMap:
		// These values are always of pointer type in IR.
		b.trackPointer(value)
	case *ssa.Call, *ssa.Convert, *ssa.MakeClosure, *ssa.MakeInterface, *ssa.MakeSlice, *ssa.Next:
		if !value.IsNil() {
			b.trackValue(value)
		}
	case *ssa.Select:
		if alloca, ok := b.selectRecvBuf[expr]; ok {
			if alloca.IsAUndefValue().IsNil() {
				b.trackPointer(alloca)
			}
		}
	case *ssa.UnOp:
		switch expr.Op {
		case token.MUL:
			// Pointer dereference.
			b.trackValue(value)
		case token.ARROW:
			// Channel receive operator.
			// It's not necessary to look at commaOk here, because in that
			// case it's just an aggregate and trackValue will extract the
			// pointer in there (if there is one).
			b.trackValue(value)
		}
	case *ssa.BinOp:
		switch expr.Op {
		case token.ADD:
			// String concatenation.
			b.trackValue(value)
		}
	}
}

// trackValue locates pointers in a value (possibly an aggregate) and tracks the
// individual pointers
func (b *builder) trackValue(value llvm.Value) {
	typ := value.Type()
	switch typ.TypeKind() {
	case llvm.PointerTypeKind:
		b.trackPointer(value)
	case llvm.StructTypeKind:
		if !typeHasPointers(typ) {
			return
		}
		numElements := typ.StructElementTypesCount()
		for i := range numElements {
			subValue := b.CreateExtractValue(value, i, "")
			b.trackValue(subValue)
		}
	case llvm.ArrayTypeKind:
		if !typeHasPointers(typ) {
			return
		}
		numElements := typ.ArrayLength()
		for i := range numElements {
			subValue := b.CreateExtractValue(value, i, "")
			b.trackValue(subValue)
		}
	}
}

// trackPointer creates a call to runtime.trackPointer, bitcasting the pointer
// first if needed. The input value must be of LLVM pointer type.
func (b *builder) trackPointer(value llvm.Value) {
	b.createRuntimeCall("trackPointer", []llvm.Value{value, b.stackChainAlloca}, "")
}

// typeHasPointers returns whether this type is a pointer or contains pointers.
// If the type is an aggregate type, it will check whether there is a pointer
// inside.
func typeHasPointers(t llvm.Type) bool {
	switch t.TypeKind() {
	case llvm.PointerTypeKind:
		return true
	case llvm.StructTypeKind:
		return slices.ContainsFunc(t.StructElementTypes(), typeHasPointers)
	case llvm.ArrayTypeKind:
		if t.ArrayLength() == 0 {
			return false
		}
		if typeHasPointers(t.ElementType()) {
			return true
		}
		return false
	default:
		return false
	}
}
