target datalayout = "e-m:e-p:32:32-i64:64-n32:64-S128"
target triple = "wasm32-unknown-unknown-wasm"

@keepMe = internal global i8 0
@big = global [4 x { i32, ptr }] zeroinitializer
@wide = global [40 x ptr] zeroinitializer
@"runtime/gc.layout:40-ffffffffff" = linkonce_odr unnamed_addr constant { i32, [5 x i8] } { i32 40, [5 x i8] c"\FF\FF\FF\FF\FF" }
@changed = global [2 x ptr] zeroinitializer
@"runtime/gc.layout:48-ffffffffffff" = linkonce_odr unnamed_addr constant { i32, [6 x i8] } { i32 48, [6 x i8] c"\FF\FF\FF\FF\FF\FF" }
@constant = constant [4 x { i32, ptr }] zeroinitializer
@llvm.compiler.used = appending global [1 x ptr] [ptr @keepMe], section "llvm.metadata"
@runtime.preciseGlobals.entries = internal global [2 x { ptr, i32, ptr }] [{ ptr, i32, ptr } { ptr @big, i32 32, ptr inttoptr (i32 133 to ptr) }, { ptr, i32, ptr } { ptr @wide, i32 160, ptr @"runtime/gc.layout:40-ffffffffff" }]
@runtime.preciseGlobals = internal global { i32, ptr } { i32 2, ptr @runtime.preciseGlobals.entries }, align 4

define i32 @countPreciseGlobals() {
  %n = load i32, ptr @runtime.preciseGlobals, align 4
  ret i32 %n
}

define ptr @useGlobals() {
  store ptr null, ptr @big, align 4
  store ptr null, ptr @wide, align 4
  store ptr null, ptr @changed, align 4
  ret ptr @constant
}
