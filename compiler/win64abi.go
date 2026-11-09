package compiler

// This file implements the Microsoft x64 rules for aggregates at the C
// boundary (cgo calls and //export functions). LLVM splits a first-class
// aggregate argument into scalars, which does not match this ABI.
// https://learn.microsoft.com/en-us/cpp/build/x64-calling-convention

import (
	"go/types"

	"tinygo.org/x/go-llvm"
)

// win64Aggregate reports whether t is a C-boundary aggregate on windows/amd64.
// It returns the integer type that passes it, or a nil type when it is passed
// by a pointer to a copy.
func (c *compilerContext) win64Aggregate(t llvm.Type) (lowered bool, coerced llvm.Type) {
	if c.GOOS != "windows" || c.GOARCH != "amd64" {
		return false, llvm.Type{}
	}
	switch t.TypeKind() {
	case llvm.StructTypeKind, llvm.ArrayTypeKind:
	default:
		return false, llvm.Type{}
	}
	switch size := c.targetData.TypeAllocSize(t); size {
	case 0:
		return false, llvm.Type{}
	case 1, 2, 4, 8:
		return true, c.ctx.IntType(int(size) * 8)
	}
	return true, llvm.Type{}
}

// win64ParamType returns the LLVM type of a C-boundary parameter of type t.
func (c *compilerContext) win64ParamType(t llvm.Type) (llvm.Type, bool) {
	lowered, coerced := c.win64Aggregate(t)
	if !lowered {
		return t, false
	}
	if coerced.IsNil() {
		return c.dataPtrType, true
	}
	return coerced, true
}

// win64Sret reports whether a C-boundary function with this signature returns
// its result through a hidden pointer.
func (c *compilerContext) win64Sret(sig *types.Signature) bool {
	lowered, coerced := c.win64Aggregate(c.getLLVMResultType(sig))
	return lowered && coerced.IsNil()
}

func (c *compilerContext) sretAttribute(t llvm.Type) llvm.Attribute {
	return c.ctx.CreateTypeAttribute(llvm.AttributeKindID("sret"), t)
}

// win64Coerce reinterprets v as type to (of the same size) through memory.
func (b *builder) win64Coerce(v llvm.Value, to llvm.Type) llvm.Value {
	alloca, size := b.createTemporaryAlloca(v.Type(), "win64.coerce")
	b.CreateStore(v, alloca)
	result := b.CreateLoad(to, alloca, "")
	b.emitLifetimeEnd(alloca, size)
	return result
}

// createWin64Call calls a C-boundary function. The arguments and resultType
// are the Go values; this function lowers them to the Microsoft x64 form.
// A cgo C function runs on the system stack.
func (b *builder) createWin64Call(fnType llvm.Type, fn llvm.Value, args []llvm.Value, resultType llvm.Type, cgo, mayUnwind, maySuspend bool) llvm.Value {
	type temporary struct{ alloca, size llvm.Value }
	var temporaries []temporary
	var lowered []llvm.Value
	retLowered, retCoerced := b.win64Aggregate(resultType)
	var sret llvm.Value
	if retLowered && retCoerced.IsNil() {
		alloca, size := b.createTemporaryAlloca(resultType, "win64.result")
		temporaries = append(temporaries, temporary{alloca, size})
		sret = alloca
		lowered = append(lowered, sret)
	}
	for _, arg := range args {
		isAggregate, coerced := b.win64Aggregate(arg.Type())
		switch {
		case !isAggregate:
			lowered = append(lowered, arg)
		case !coerced.IsNil():
			lowered = append(lowered, b.win64Coerce(arg, coerced))
		default:
			// The callee owns this copy and may change it.
			alloca, size := b.createTemporaryAlloca(arg.Type(), "win64.arg")
			temporaries = append(temporaries, temporary{alloca, size})
			b.CreateStore(arg, alloca)
			lowered = append(lowered, alloca)
		}
	}
	var sretType llvm.Type
	if !sret.IsNil() {
		sretType = resultType
	}
	var call llvm.Value
	if cgo {
		call = b.createCgoCall(fnType, fn, lowered, sretType)
	} else {
		call = b.createInvokeWithAnalysis(fnType, fn, lowered, "", mayUnwind, maySuspend)
		if !sret.IsNil() {
			call.AddCallSiteAttribute(1, b.sretAttribute(resultType))
		}
	}
	result := call
	if !sret.IsNil() {
		result = b.CreateLoad(resultType, sret, "")
	} else if retLowered {
		result = b.win64Coerce(call, resultType)
	}
	for _, t := range temporaries {
		b.emitLifetimeEnd(t.alloca, t.size)
	}
	return result
}
