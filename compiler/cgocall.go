package compiler

// This file runs cgo C calls on the system stack on windows/amd64. The
// goroutine stack is small, and Windows exception dispatch accepts only
// frames inside the thread stack bounds. A Go function that C calls switches
// back to the goroutine stack. See CgoCall in internal/task.

import (
	"strings"

	"golang.org/x/tools/go/ssa"
	"tinygo.org/x/go-llvm"
)

// isCgoFunction reports whether fn is a C function declared by cgo.
func isCgoFunction(fn *ssa.Function, info functionInfo) bool {
	return info.exported && fn.Blocks == nil && strings.HasPrefix(fn.Name(), "_Cgo_")
}

// isCgoCallback reports whether the current function is a Go function that C
// code can call and that must run on the goroutine stack.
func (b *builder) isCgoCallback() bool {
	if b.GOOS != "windows" || b.GOARCH != "amd64" || !b.info.exported || b.fn.Pkg == nil {
		return false
	}
	// The runtime calls its own exported functions on the stack it needs.
	path := b.fn.Pkg.Pkg.Path()
	return path != "runtime" && !strings.HasPrefix(path, "runtime/") && !strings.HasPrefix(path, "internal/")
}

type stackThunkKey struct {
	fn       llvm.Value
	packType llvm.Type // differs between calls of a variadic function
	suffix   string
}

// stackSwitchThunk returns a function that loads the arguments of fn from a
// struct, calls fn, and stores the result after the arguments.
func (c *compilerContext) stackSwitchThunk(fnType, packType llvm.Type, fn llvm.Value, sretType llvm.Type, suffix string) llvm.Value {
	key := stackThunkKey{fn, packType, suffix}
	if thunk, ok := c.stackThunks[key]; ok {
		return thunk
	}
	thunk := llvm.AddFunction(c.mod, fn.Name()+suffix, llvm.FunctionType(c.ctx.VoidType(), []llvm.Type{c.dataPtrType}, false))
	thunk.SetLinkage(llvm.InternalLinkage)
	c.addStandardAttributes(thunk)
	thunk.SetUnnamedAddr(true)
	c.stackThunks[key] = thunk

	builder := c.ctx.NewBuilder()
	defer builder.Dispose()
	builder.SetInsertPointAtEnd(c.ctx.AddBasicBlock(thunk, "entry"))
	pack := thunk.Param(0)
	fields := packType.StructElementTypes()
	hasResult := fnType.ReturnType().TypeKind() != llvm.VoidTypeKind
	numArgs := len(fields)
	if hasResult {
		numArgs--
	}
	args := make([]llvm.Value, numArgs)
	for i := range numArgs {
		gep := builder.CreateStructGEP(packType, pack, i, "")
		args[i] = builder.CreateLoad(fields[i], gep, "")
	}
	call := builder.CreateCall(fnType, fn, args, "")
	if !sretType.IsNil() {
		call.AddCallSiteAttribute(1, c.sretAttribute(sretType))
	}
	if hasResult {
		builder.CreateStore(call, builder.CreateStructGEP(packType, pack, numArgs, ""))
	}
	builder.CreateRetVoid()
	return thunk
}

// stackSwitchPackType returns the struct type that holds args and the result.
func (c *compilerContext) stackSwitchPackType(fnType llvm.Type, args []llvm.Value) llvm.Type {
	var fields []llvm.Type
	for _, arg := range args {
		fields = append(fields, arg.Type())
	}
	if ret := fnType.ReturnType(); ret.TypeKind() != llvm.VoidTypeKind {
		fields = append(fields, ret)
	}
	return c.ctx.StructType(fields, false)
}

// createStackSwitch calls runtimeFn(thunk, pack) with args stored in pack, and
// returns the result of the thunk.
func (b *builder) createStackSwitch(runtimeFn string, fnType llvm.Type, fn llvm.Value, args []llvm.Value, sretType llvm.Type, suffix string) llvm.Value {
	packType := b.stackSwitchPackType(fnType, args)
	thunk := b.stackSwitchThunk(fnType, packType, fn, sretType, suffix)
	pack, size := b.createTemporaryAlloca(packType, "stackswitch.args")
	for i, arg := range args {
		b.CreateStore(arg, b.CreateStructGEP(packType, pack, i, ""))
	}
	result := b.createRuntimeCall(runtimeFn, []llvm.Value{thunk, pack}, "")
	if ret := fnType.ReturnType(); ret.TypeKind() != llvm.VoidTypeKind {
		result = b.CreateLoad(ret, b.CreateStructGEP(packType, pack, len(args), ""), "")
	}
	b.emitLifetimeEnd(pack, size)
	return result
}

// createCgoCall calls the C function fn on the system stack.
func (b *builder) createCgoCall(fnType llvm.Type, fn llvm.Value, args []llvm.Value, sretType llvm.Type) llvm.Value {
	return b.createStackSwitch("cgocall", fnType, fn, args, sretType, "$cgocall")
}

// createCgoCallbackSwitch adds a new entry block before entryBlock. When C
// code calls this function on the system stack, the new block calls the
// function again on the goroutine stack and returns its result.
func (b *builder) createCgoCallbackSwitch(entryBlock llvm.BasicBlock) {
	check := b.ctx.InsertBasicBlock(entryBlock, "cgocallback.check")
	switchBlock := b.ctx.InsertBasicBlock(entryBlock, "cgocallback.switch")
	b.SetInsertPointAtEnd(check)
	needSwitch := b.createRuntimeCall("cgocallbackSwitch", nil, "")
	b.CreateCondBr(needSwitch, switchBlock, entryBlock)

	b.SetInsertPointAtEnd(switchBlock)
	var sretType llvm.Type
	if b.win64Sret(b.fn.Signature) {
		sretType = b.getLLVMResultType(b.fn.Signature)
	}
	fnType := b.llvmFn.GlobalValueType()
	result := b.createStackSwitch("cgocallback", fnType, b.llvmFn, b.llvmFn.Params(), sretType, "$cgocallback")
	if fnType.ReturnType().TypeKind() == llvm.VoidTypeKind {
		b.CreateRetVoid()
	} else {
		b.CreateRet(result)
	}
}
