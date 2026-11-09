package transform

// This file implements an escape analysis pass. It looks for calls to
// runtime.alloc and replaces these calls with a stack allocation if the
// allocated value does not escape. It uses the LLVM nocapture flag for
// interprocedural escape analysis.

import (
	"bufio"
	"fmt"
	"go/token"
	"os"
	"regexp"

	"github.com/tinygo-org/tinygo/compiler/llvmutil"
	"tinygo.org/x/go-llvm"
)

// OptimizeAllocs tries to replace heap allocations with stack allocations
// whenever possible. It relies on the LLVM 'nocapture' flag for interprocedural
// escape analysis, and within a function looks whether an allocation can escape
// to the heap.
// If printAllocs is non-nil, it indicates the regexp of functions for which a
// heap allocation explanation should be printed (why the object can't be stack
// allocated).
func OptimizeAllocs(mod llvm.Module, printAllocs *regexp.Regexp, maxStackAlloc uint64, logger func(token.Position, string)) {
	// Find allocator functions.
	var allocators []llvm.Value
	for _, name := range []string{"runtime.alloc", "runtime.alloc_noheap"} {
		allocator := mod.NamedFunction(name)
		if !allocator.IsNil() {
			allocators = append(allocators, allocator)
		}
	}
	if len(allocators) == 0 {
		// nothing to optimize
		return
	}

	targetData := llvm.NewTargetData(mod.DataLayout())
	defer targetData.Dispose()
	ctx := mod.Context()
	builder := ctx.NewBuilder()
	defer builder.Dispose()

	// Determine the maximum alignment on this platform.
	complex128Type := ctx.StructType([]llvm.Type{ctx.DoubleType(), ctx.DoubleType()}, false)
	maxAlign := int64(targetData.ABITypeAlignment(complex128Type))

	// Find all allocator calls.
	var heapallocs []llvm.Value
	for _, allocator := range allocators {
		heapallocs = append(heapallocs, getUses(allocator)...)
	}

	for _, heapalloc := range heapallocs {
		logAllocs := printAllocs != nil && printAllocs.MatchString(heapalloc.InstructionParent().Parent().Name())
		if heapalloc.Operand(0).IsAConstantInt().IsNil() {
			// Do not allocate variable length arrays on the stack.
			if logAllocs {
				logger(getPosition(heapalloc), "size is not constant")
			}
			continue
		}

		size := heapalloc.Operand(0).ZExtValue()
		if size > maxStackAlloc {
			// The maximum size for a stack allocation.
			if logAllocs {
				logger(getPosition(heapalloc),
					fmt.Sprintf("object size %d exceeds maximum stack allocation size %d", size, maxStackAlloc))
			}
			continue
		}

		if size == 0 {
			// If the size is 0, the pointer is allowed to alias other
			// zero-sized pointers. Use the pointer to the global that would
			// also be returned by runtime.alloc.
			zeroSizedAlloc := mod.NamedGlobal("runtime.zeroSizedAlloc")
			if !zeroSizedAlloc.IsNil() {
				heapalloc.ReplaceAllUsesWith(zeroSizedAlloc)
				heapalloc.EraseFromParentAsInstruction()
			}
			continue
		}

		// In general the pattern is:
		//     %0 = call i8* @runtime.alloc(i32 %size, i8* null)
		//     %1 = bitcast i8* %0 to type*
		//     (use %1 only)
		// But the bitcast might sometimes be dropped when allocating an *i8.
		// The 'bitcast' variable below is thus usually a bitcast of the
		// heapalloc but not always.
		bitcast := heapalloc // instruction that creates the value
		if uses := getUses(heapalloc); len(uses) == 1 && !uses[0].IsABitCastInst().IsNil() {
			// getting only bitcast use
			bitcast = uses[0]
		}

		if at := valueEscapesAt(bitcast); !at.IsNil() {
			if logAllocs {
				atPos := getPosition(at)
				msg := "escapes at unknown line"
				if atPos.Line != 0 {
					msg = fmt.Sprintf("escapes at line %d", atPos.Line)
				}
				logger(getPosition(heapalloc), msg)
			}
			continue
		}
		// The pointer value does not escape.

		// Determine the appropriate alignment of the alloca.
		attr := heapalloc.GetCallSiteEnumAttribute(0, llvm.AttributeKindID("align"))
		alignment := int(maxAlign)
		if !attr.IsNil() {
			// 'align' return value attribute is set, so use it.
			// This is basically always the case, but to be sure we'll default
			// to maxAlign if it isn't.
			alignment = int(attr.GetEnumValue())
		}

		// Insert alloca in the entry block. Do it here so that mem2reg can
		// promote it to a SSA value.
		fn := bitcast.InstructionParent().Parent()
		builder.SetInsertPointBefore(fn.EntryBasicBlock().FirstInstruction())
		allocaType := llvm.ArrayType(mod.Context().Int8Type(), int(size))
		alloca := builder.CreateAlloca(allocaType, "stackalloc")
		alloca.SetAlignment(alignment)

		// Zero the allocation inside the block where the value was originally allocated.
		zero := llvm.ConstNull(alloca.AllocatedType())
		builder.SetInsertPointBefore(bitcast)
		store := builder.CreateStore(zero, alloca)
		store.SetAlignment(alignment)

		// Replace heap alloc bitcast with stack alloc bitcast.
		bitcast.ReplaceAllUsesWith(alloca)
		if heapalloc != bitcast {
			bitcast.EraseFromParentAsInstruction()
		}
		heapalloc.EraseFromParentAsInstruction()
	}
}

// FormatAllocReason renders the heap allocation in a human-readable format.
func FormatAllocReason(pos token.Position, reason string) string {
	return fmt.Sprintf("%s: object allocated on the heap: %s", pos.String(), reason)
}

// FormatAllocCover renders the heap allocation in the go coverage tool format.
func FormatAllocCover(pos token.Position) string {
	if pos.Filename == "" || pos.Line <= 0 {
		return "" // no position info; a blank line would corrupt the profile
	}
	// Highlight the whole line: column 1 to one past the last byte (the end
	// column is exclusive, so add 1 to the line length).
	endCol := max(lineLengthAt(pos.Filename, pos.Line), 1) + 1
	return fmt.Sprintf("%s:%d.1,%d.%d 1 0", pos.Filename, pos.Line, pos.Line, endCol)
}

// valueEscapesAt returns the instruction where the given value may escape and a
// nil llvm.Value if it definitely doesn't. The value must be an instruction.
func valueEscapesAt(value llvm.Value) llvm.Value {
	// A phi or a local container may hold the object only if it is allocated
	// at most once per call: the alloca is shared by all loop iterations.
	aliasOK := !inCycle(value.InstructionParent())
	return valueEscapesAtImpl(value, false, aliasOK, nil, nil).escapeAt
}

// inCycle reports whether bb can run more than once per call.
func inCycle(bb llvm.BasicBlock) bool {
	seen := map[llvm.BasicBlock]bool{}
	stack := []llvm.BasicBlock{bb}
	for len(stack) > 0 {
		b := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		term := b.LastInstruction()
		for i := range term.SuccessorsCount() {
			s := term.Successor(i)
			if s == bb {
				return true
			}
			if !seen[s] {
				seen[s] = true
				stack = append(stack, s)
			}
		}
	}
	return false
}

// localObject returns the alloca or runtime.alloc call that ptr points into,
// or a nil value if ptr may point elsewhere.
func localObject(ptr llvm.Value) llvm.Value {
	for !ptr.IsAInstruction().IsNil() {
		switch ptr.InstructionOpcode() {
		case llvm.GetElementPtr, llvm.BitCast:
			ptr = ptr.Operand(0)
		case llvm.Alloca:
			return ptr
		case llvm.Call:
			switch ptr.CalledValue().Name() {
			case "runtime.alloc", "runtime.alloc_noheap":
				return ptr
			}
			return llvm.Value{}
		default:
			return llvm.Value{}
		}
	}
	return llvm.Value{}
}

// containerEscapesAt checks a local object that holds the analyzed pointer: the
// pointer escapes if a pointer loaded from the container escapes, or if the
// container itself leaves the function.
func containerEscapesAt(container llvm.Value, allowReturn, aliasOK bool, visiting map[llvm.Value]struct{}, env map[llvm.Value]llvm.Value) escapeResult {
	var result escapeResult
	for _, use := range getUses(container) {
		switch use.InstructionOpcode() {
		case llvm.GetElementPtr, llvm.BitCast:
			if !result.merge(containerEscapesAt(use, allowReturn, aliasOK, visiting, env)) {
				return result
			}
		case llvm.Load:
			switch use.Type().TypeKind() {
			case llvm.PointerTypeKind, llvm.StructTypeKind, llvm.ArrayTypeKind:
				// An aggregate load may carry the pointer: follow it as a value.
				if !result.merge(valueEscapesAtImpl(use, allowReturn, aliasOK, visiting, env)) {
					return result
				}
			case llvm.IntegerTypeKind, llvm.FloatTypeKind, llvm.DoubleTypeKind:
				// Not a pointer: cannot be the stored pointer.
			default:
				return escapeResult{escapeAt: use}
			}
		case llvm.Store:
			if use.Operand(0) == container {
				return escapeResult{escapeAt: use}
			}
		case llvm.ICmp:
		case llvm.Call:
			// trackPointer only makes the container a GC root.
			if use.CalledValue().Name() == "runtime.trackPointer" {
				continue
			}
			if !result.merge(containerCallEscapesAt(use, container, aliasOK, visiting, env)) {
				return result
			}
		default:
			return escapeResult{escapeAt: use}
		}
	}
	return result
}

// containerCallEscapesAt checks a call that receives the container. A defined
// callee may load the stored pointer; those loads must not escape there.
// Declarations, memcpy included, may copy the pointer out.
func containerCallEscapesAt(call, container llvm.Value, aliasOK bool, visiting map[llvm.Value]struct{}, env map[llvm.Value]llvm.Value) escapeResult {
	called := calledFunction(call, env)
	if called.IsNil() || called.IsDeclaration() {
		return escapeResult{escapeAt: call}
	}
	for i := called.ParamsCount(); i < call.OperandsCount()-1; i++ {
		if call.Operand(i) == container {
			return escapeResult{escapeAt: call}
		}
	}
	var result escapeResult
	childEnv := calleeEnv(call, called, env)
	for i := range called.ParamsCount() {
		if call.Operand(i) != container {
			continue
		}
		param := called.Param(i)
		if _, ok := visiting[param]; ok {
			return escapeResult{escapeAt: call} // recursive callee
		}
		visiting[param] = struct{}{}
		// The container belongs to the caller: returning it is an escape.
		ok := result.merge(containerEscapesAt(param, false, aliasOK, visiting, childEnv))
		delete(visiting, param)
		if !ok {
			return result
		}
	}
	return result
}

type escapeResult struct {
	escapeAt llvm.Value

	// returned is separate from escapeAt because values can flow to a return
	// through aggregate operations. LLVM can mark a scalar returned parameter,
	// but a slice data pointer returned inside {ptr, len, cap} is only visible
	// after walking insertvalue/ret uses in the callee.
	returned bool
}

func (r *escapeResult) merge(other escapeResult) bool {
	if !other.escapeAt.IsNil() {
		r.escapeAt = other.escapeAt
		return false
	}
	r.returned = r.returned || other.returned
	return true
}

func valueEscapesAtImpl(value llvm.Value, allowReturn, aliasOK bool, visiting map[llvm.Value]struct{}, env map[llvm.Value]llvm.Value) escapeResult {
	if visiting == nil {
		visiting = make(map[llvm.Value]struct{})
	}
	if _, ok := visiting[value]; ok {
		// Recursive call graph while following returned parameters. Treat this
		// as escaping to keep the analysis conservative and bounded.
		return escapeResult{escapeAt: value}
	}
	visiting[value] = struct{}{}
	defer delete(visiting, value)

	var result escapeResult
	uses := getUses(value)
	for _, use := range uses {
		if use.IsAInstruction().IsNil() {
			panic("expected instruction use")
		}
		switch use.InstructionOpcode() {
		case llvm.GetElementPtr:
			if !result.merge(valueEscapesAtImpl(use, allowReturn, aliasOK, visiting, env)) {
				return result
			}
		case llvm.BitCast:
			// A bitcast escapes if the casted-to value escapes.
			if !result.merge(valueEscapesAtImpl(use, allowReturn, aliasOK, visiting, env)) {
				return result
			}
		case llvm.InsertValue:
			if !result.merge(valueEscapesAtImpl(use, allowReturn, aliasOK, visiting, env)) {
				return result
			}
		case llvm.ExtractValue:
			if typeHasPointers(use.Type()) {
				if !result.merge(valueEscapesAtImpl(use, allowReturn, aliasOK, visiting, env)) {
					return result
				}
			}
		case llvm.Select:
			// A select escapes if the selected value escapes.
			if !result.merge(valueEscapesAtImpl(use, allowReturn, aliasOK, visiting, env)) {
				return result
			}
		case llvm.PHI:
			// A phi escapes if the merged value escapes.
			if !aliasOK {
				return escapeResult{escapeAt: use}
			}
			if _, ok := visiting[use]; ok {
				continue // phi cycle: the frame that visits it checks its other uses
			}
			if !result.merge(valueEscapesAtImpl(use, allowReturn, aliasOK, visiting, env)) {
				return result
			}
		case llvm.Load:
			// Load does not escape.
		case llvm.Store:
			// Store only escapes when the value is stored to, not when the
			// value is stored into another value, or into a local container
			// whose loads do not escape.
			if use.Operand(0) != value {
				break
			}
			container := localObject(use.Operand(1))
			if !aliasOK || container.IsNil() {
				return escapeResult{escapeAt: use}
			}
			if !result.merge(containerEscapesAt(container, allowReturn, aliasOK, visiting, env)) {
				return result
			}
		case llvm.Call:
			if !result.merge(callValueEscapesAt(use, value, allowReturn, aliasOK, visiting, env)) {
				return result
			}
		case llvm.ICmp:
			// Comparing pointers don't let the pointer escape.
			// This is often a compiler-inserted nil check.
		case llvm.Ret:
			if !allowReturn || use.Operand(0) != value {
				return escapeResult{escapeAt: use}
			}
			result.returned = true
		default:
			// Unknown instruction, might escape.
			return escapeResult{escapeAt: use}
		}
	}

	// Checked all uses, and none let the pointer value escape.
	return result
}

// callValueEscapesAt returns whether value escapes through this call. It also
// handles calls that return value unchanged, as long as the called function does
// not otherwise capture the parameter and the returned alias does not escape.
func callValueEscapesAt(call, value llvm.Value, allowReturn, aliasOK bool, visiting map[llvm.Value]struct{}, env map[llvm.Value]llvm.Value) escapeResult {
	called := calledFunction(call, env)
	if called.IsNil() {
		return escapeResult{escapeAt: call}
	}
	var childEnv map[llvm.Value]llvm.Value
	childEnvDone := false
	kindNoCapture := llvm.AttributeKindID(llvmutil.NoCaptureAttrName())
	kindReturned := llvm.AttributeKindID("returned")
	matched := false
	var result escapeResult
	for i := 0; i < called.ParamsCount(); i++ {
		if call.Operand(i) != value {
			continue
		}
		matched = true
		index := i + 1 // param attributes start at 1
		nocapture := llvmutil.IsNoCapture(called.GetEnumAttributeAtIndex(index, kindNoCapture))
		returnedParam := !called.GetEnumAttributeAtIndex(index, kindReturned).IsNil()
		if returnedParam {
			result.returned = true
		}
		if nocapture {
			continue
		}
		if called.IsDeclaration() {
			return escapeResult{escapeAt: call}
		}
		if !childEnvDone {
			childEnv, childEnvDone = calleeEnv(call, called, env), true
		}
		if !result.merge(valueEscapesAtImpl(called.Param(i), true, aliasOK, visiting, childEnv)) {
			return result
		}
	}
	for i := called.ParamsCount(); i < call.OperandsCount(); i++ {
		if call.Operand(i) == value {
			return escapeResult{escapeAt: call}
		}
	}
	if !matched {
		return escapeResult{}
	}
	if result.returned {
		return valueEscapesAtImpl(call, allowReturn, aliasOK, visiting, env)
	}
	return escapeResult{}
}

// calledFunction returns the function that call runs: the callee, or the
// constant function that a caller bound to the function-pointer parameter used
// as the callee. It returns a nil value if the target is unknown.
func calledFunction(call llvm.Value, env map[llvm.Value]llvm.Value) llvm.Value {
	called := call.CalledValue()
	if called.IsAFunction().IsNil() {
		called = env[called]
		if called.IsNil() {
			return llvm.Value{}
		}
	}
	if called.GlobalValueType() != call.CalledFunctionType() {
		return llvm.Value{}
	}
	return called
}

// calleeEnv binds each parameter of called that receives a constant function at
// this call, directly or through a binding of the caller.
func calleeEnv(call, called llvm.Value, env map[llvm.Value]llvm.Value) map[llvm.Value]llvm.Value {
	var child map[llvm.Value]llvm.Value
	for i := range called.ParamsCount() {
		fn := call.Operand(i)
		if fn.IsAFunction().IsNil() {
			fn = env[fn]
		}
		if fn.IsNil() {
			continue
		}
		if child == nil {
			child = make(map[llvm.Value]llvm.Value)
		}
		child[called.Param(i)] = fn
	}
	return child
}

func lineLengthAt(filename string, lineNumber int) int {
	f, err := os.Open(filename)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	line := 1
	for scanner.Scan() {
		if line == lineNumber {
			return len(scanner.Text())
		}
		line++
	}
	return 0
}
