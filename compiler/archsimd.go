package compiler

// This file maps simd/archsimd vector types to LLVM vectors and defines the
// bodyless archsimd functions as LLVM operations.

import (
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
	"tinygo.org/x/go-llvm"
)

const simdPackage = "simd/archsimd"

// isSimdFunction reports whether fn is declared in simd/archsimd.
func isSimdFunction(fn *ssa.Function) bool {
	return fn.Pkg != nil && fn.Pkg.Pkg.Path() == simdPackage
}

// simdElem returns the array that gives an archsimd vector its shape.
// Vector and mask types are structs with a marker field and a vals array.
func simdElem(typ types.Type) (*types.Array, bool) {
	named, ok := types.Unalias(typ).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != simdPackage {
		return nil, false
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok || st.NumFields() != 2 || st.Field(1).Name() != "vals" {
		return nil, false
	}
	array, ok := st.Field(1).Type().(*types.Array)
	return array, ok
}

// simdVectorType returns the LLVM vector type of an archsimd vector or mask.
func (c *compilerContext) simdVectorType(typ types.Type) (llvm.Type, bool) {
	array, ok := simdElem(typ)
	if !ok {
		return llvm.Type{}, false
	}
	c.usesSimdVectors = true
	return llvm.VectorType(c.getLLVMType(array.Elem()), int(array.Len())), true
}

// containsVector reports whether t is a vector or holds a vector.
func containsVector(t llvm.Type) bool {
	switch t.TypeKind() {
	case llvm.VectorTypeKind:
		return true
	case llvm.StructTypeKind:
		for _, field := range t.StructElementTypes() {
			if containsVector(field) {
				return true
			}
		}
	case llvm.ArrayTypeKind:
		return containsVector(t.ElementType())
	}
	return false
}

// relaxSimdAlignment marks vector loads and stores as unaligned.
// The heap and the *[N]T loads do not give the natural vector alignment.
func (c *compilerContext) relaxSimdAlignment() {
	if !c.usesSimdVectors {
		return
	}
	for fn := c.mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		for block := fn.FirstBasicBlock(); !block.IsNil(); block = llvm.NextBasicBlock(block) {
			for inst := block.FirstInstruction(); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
				if !inst.IsALoadInst().IsNil() && containsVector(inst.Type()) {
					inst.SetAlignment(1)
				} else if !inst.IsAStoreInst().IsNil() && containsVector(inst.Operand(0).Type()) {
					inst.SetAlignment(1)
				}
			}
		}
	}
}

// simdFeature reports a CPU feature from the compile target.
func (b *builder) simdFeature(name string) llvm.Value {
	present := uint64(0)
	if strings.Contains(","+b.Features+",", ",+"+strings.ToLower(name)+",") {
		present = 1
	}
	return llvm.ConstInt(b.ctx.Int1Type(), present, false)
}

// isSimdFeature reports whether simdFeature can answer for this X86Features method.
func isSimdFeature(name string) bool {
	return name == "AVX" || name == "AVX2" || name == "FMA"
}

// defineSimdIntrinsic gives a bodyless archsimd function an LLVM body.
// An unknown operation stays undefined, so the link names it.
func (b *builder) defineSimdIntrinsic() {
	op := b.simdOp()
	if op == nil {
		return
	}
	b.createFunctionStart(true)
	args := make([]llvm.Value, len(b.fn.Params))
	for i, param := range b.fn.Params {
		args[i] = b.getValue(param, getPos(b.fn))
	}
	if result := op(args); result.IsNil() {
		b.CreateRetVoid()
	} else {
		b.CreateRet(result)
	}
}

// simdOp returns the LLVM operation of the current function, or nil if there is none.
// The operations are only tested on amd64.
func (b *builder) simdOp() func([]llvm.Value) llvm.Value {
	if b.GOARCH != "amd64" {
		return nil
	}
	name := b.fn.Name()
	var result llvm.Type
	if results := b.fn.Signature.Results(); results.Len() == 1 {
		result = b.getLLVMType(results.At(0).Type())
	}
	isFloat, isMask := false, false
	if recv := b.fn.Signature.Recv(); recv != nil {
		array, ok := simdElem(recv.Type())
		if !ok {
			return nil
		}
		basic, _ := array.Elem().(*types.Basic)
		isFloat = basic != nil && basic.Info()&types.IsFloat != 0
		isMask = strings.HasPrefix(types.Unalias(recv.Type()).(*types.Named).Obj().Name(), "Mask")
	}
	binary := func(create func(x, y llvm.Value, name string) llvm.Value) func([]llvm.Value) llvm.Value {
		return func(a []llvm.Value) llvm.Value { return create(a[0], a[1], "") }
	}
	floatCompare := func(pred llvm.FloatPredicate) func([]llvm.Value) llvm.Value {
		return func(a []llvm.Value) llvm.Value {
			return b.CreateSExt(b.CreateFCmp(pred, a[0], a[1], ""), result, "")
		}
	}
	bitcast := func(a []llvm.Value) llvm.Value { return b.CreateBitCast(a[0], result, "") }

	switch {
	case name == "Add" && isFloat:
		return binary(b.CreateFAdd)
	case name == "Sub" && isFloat:
		return binary(b.CreateFSub)
	case name == "Mul" && isFloat:
		return binary(b.CreateFMul)
	case name == "Div" && isFloat:
		return binary(b.CreateFDiv)
	case name == "Equal" && isFloat:
		return floatCompare(llvm.FloatOEQ)
	case name == "NotEqual" && isFloat:
		return floatCompare(llvm.FloatUNE)
	case name == "Greater" && isFloat:
		return floatCompare(llvm.FloatOGT)
	case name == "Equal":
		return func(a []llvm.Value) llvm.Value {
			return b.CreateSExt(b.CreateICmp(llvm.IntEQ, a[0], a[1], ""), result, "")
		}
	case name == "And":
		return binary(b.CreateAnd)
	case name == "Or":
		return binary(b.CreateOr)
	case name == "Xor":
		return binary(b.CreateXor)
	case name == "ToBits" && isMask:
		return func(a []llvm.Value) llvm.Value {
			bits := b.ctx.IntType(a[0].Type().VectorSize())
			mask := b.CreateICmp(llvm.IntSLT, a[0], llvm.ConstNull(a[0].Type()), "")
			return b.CreateZExtOrBitCast(b.CreateBitCast(mask, bits, ""), result, "")
		}
	case name == "ToBits", name == "asMask", name == "ToInt32x8",
		strings.HasPrefix(name, "BitsTo"), strings.HasPrefix(name, "ReshapeTo"):
		return bitcast
	case strings.HasPrefix(name, "Load") && strings.HasSuffix(name, "Array"):
		return func(a []llvm.Value) llvm.Value { return b.CreateLoad(result, a[0], "") }
	case name == "StoreArray":
		return func(a []llvm.Value) llvm.Value {
			b.CreateStore(a[0], a[1])
			return llvm.Value{}
		}
	case name == "SetElem":
		return func(a []llvm.Value) llvm.Value { return b.CreateInsertElement(a[0], a[2], a[1], "") }
	case name == "broadcast1To8":
		return func(a []llvm.Value) llvm.Value {
			mask := llvm.ConstNull(llvm.VectorType(b.ctx.Int32Type(), 8))
			return b.CreateShuffleVector(a[0], llvm.Undef(a[0].Type()), mask, "")
		}
	case name == "blend":
		return func(a []llvm.Value) llvm.Value {
			return b.CreateSelect(b.CreateICmp(llvm.IntSLT, a[2], llvm.ConstNull(a[2].Type()), ""), a[1], a[0], "")
		}
	case name == "concatSelectedConstantGrouped" && result.VectorSize() == 8:
		return b.simdShufps
	case name == "ConcatPermute128Scalars":
		return b.simdPerm128
	}
	return nil
}

// simdIndex returns i as an LLVM vector index.
func (b *builder) simdIndex(i int) llvm.Value {
	return llvm.ConstInt(b.ctx.Int32Type(), uint64(i), false)
}

// simdShufps is VSHUFPS on two 128-bit groups. The selector is a parameter
// here; after inlining it is a constant and LLVM folds this into a shuffle.
func (b *builder) simdShufps(a []llvm.Value) llvm.Value {
	x, imm, y := a[0], b.CreateZExt(a[1], b.ctx.Int32Type(), ""), a[2]
	result := llvm.Undef(x.Type())
	for i := range 8 {
		src := x
		if i&3 >= 2 {
			src = y
		}
		sel := b.CreateAnd(b.CreateLShr(imm, b.simdIndex(2*(i&3)), ""), b.simdIndex(3), "")
		elem := b.CreateExtractElement(src, b.CreateAdd(sel, b.simdIndex(i&^3), ""), "")
		result = b.CreateInsertElement(result, elem, b.simdIndex(i), "")
	}
	return result
}

// simdPerm128 is VPERM2F128 without the zeroing bits: lo and hi pick 128-bit
// elements out of the concatenation of x and y.
func (b *builder) simdPerm128(a []llvm.Value) llvm.Value {
	x, lo, hi, y := a[0], b.CreateZExt(a[1], b.ctx.Int32Type(), ""), b.CreateZExt(a[2], b.ctx.Int32Type(), ""), a[3]
	n := x.Type().VectorSize()
	indices := make([]llvm.Value, 2*n)
	for i := range indices {
		indices[i] = b.simdIndex(i)
	}
	both := b.CreateShuffleVector(x, y, llvm.ConstVector(indices, false), "")
	result := llvm.Undef(x.Type())
	for i := range n {
		sel := lo
		if i >= n/2 {
			sel = hi
		}
		index := b.CreateAdd(b.CreateMul(sel, b.simdIndex(n/2), ""), b.simdIndex(i%(n/2)), "")
		result = b.CreateInsertElement(result, b.CreateExtractElement(both, index, ""), b.simdIndex(i), "")
	}
	return result
}
