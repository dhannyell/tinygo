package transform

import (
	"strings"

	"tinygo.org/x/go-llvm"
)

// PreciseGlobals builds runtime.preciseGlobals, the table of large globals
// that the GC scans only at their pointer words. The compiler marks each
// candidate G with a companion G$gclayout that holds its size and layout. A
// global that changed since then stays out of the table, so the GC scans it
// conservatively. The companions are removed.
func PreciseGlobals(mod llvm.Module) {
	ctx := mod.Context()
	targetData := llvm.NewTargetData(mod.DataLayout())
	defer targetData.Dispose()
	uintptrType := ctx.IntType(targetData.PointerSize() * 8)
	ptrType := llvm.PointerType(ctx.Int8Type(), 0)

	companions := map[llvm.Value]struct{}{}
	var entries []llvm.Value
	for g := mod.FirstGlobal(); !g.IsNil(); g = llvm.NextGlobal(g) {
		name, ok := strings.CutSuffix(g.Name(), "$gclayout")
		if !ok || g.IsDeclaration() {
			continue
		}
		companions[g] = struct{}{}
		init := g.Initializer()
		size, layout := init.Operand(0), init.Operand(1)
		target := mod.NamedGlobal(name)
		if target.IsNil() || target.IsDeclaration() || target.IsGlobalConstant() ||
			target.IsThreadLocal() || target.Section() != "" ||
			targetData.TypeAllocSize(target.GlobalValueType()) != size.ZExtValue() {
			continue
		}
		entries = append(entries, ctx.ConstStruct([]llvm.Value{target, size, layout}, false))
	}
	// A later globaldce removes the layouts that no entry uses.
	removeFromUsed(mod, "llvm.compiler.used", companions)
	for g := range companions {
		g.EraseFromParentAsGlobal()
	}

	decl := mod.NamedGlobal("runtime.preciseGlobals")
	if decl.IsNil() || !decl.IsDeclaration() {
		return
	}
	entryType := ctx.StructType([]llvm.Type{ptrType, uintptrType, ptrType}, false)
	entriesInit := llvm.ConstArray(entryType, entries)
	// The runtime sorts the entries in place.
	entriesGlobal := llvm.AddGlobal(mod, entriesInit.Type(), "runtime.preciseGlobals.entries")
	entriesGlobal.SetInitializer(entriesInit)
	entriesGlobal.SetLinkage(llvm.InternalLinkage)
	tableInit := ctx.ConstStruct([]llvm.Value{
		llvm.ConstInt(uintptrType, uint64(len(entries)), false),
		entriesGlobal,
	}, false)
	table := llvm.AddGlobal(mod, tableInit.Type(), "")
	table.SetInitializer(tableInit)
	table.SetLinkage(llvm.InternalLinkage)
	table.SetAlignment(targetData.ABITypeAlignment(uintptrType))
	decl.ReplaceAllUsesWith(table)
	decl.EraseFromParentAsGlobal()
	table.SetName("runtime.preciseGlobals")
}

// removeFromUsed removes values from an array like llvm.compiler.used.
func removeFromUsed(mod llvm.Module, name string, values map[llvm.Value]struct{}) {
	used := mod.NamedGlobal(name)
	if used.IsNil() || len(values) == 0 {
		return
	}
	init := used.Initializer()
	var keep []llvm.Value
	for i := range init.OperandsCount() {
		if _, ok := values[init.Operand(i)]; !ok {
			keep = append(keep, init.Operand(i))
		}
	}
	elemType := init.Type().ElementType()
	section := used.Section()
	used.EraseFromParentAsGlobal()
	if len(keep) == 0 {
		return
	}
	keepInit := llvm.ConstArray(elemType, keep)
	used = llvm.AddGlobal(mod, keepInit.Type(), name)
	used.SetInitializer(keepInit)
	used.SetLinkage(llvm.AppendingLinkage)
	used.SetSection(section)
}
