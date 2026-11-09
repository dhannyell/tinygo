target datalayout = "e-m:e-p:32:32-i64:64-n32:64-S128"
target triple = "wasm32-unknown-unknown-wasm"

@runtime.preciseGlobals = external global { i32, ptr }
@llvm.compiler.used = appending global [6 x ptr] [ptr @keepMe, ptr @"big$gclayout", ptr @"wide$gclayout", ptr @"changed$gclayout", ptr @"constant$gclayout", ptr @"gone$gclayout"], section "llvm.metadata"

@keepMe = internal global i8 0

; Inline layout: the element { i32, ptr } has a pointer in word 1.
@big = global [4 x { i32, ptr }] zeroinitializer
@"big$gclayout" = internal constant { i32, ptr } { i32 32, ptr inttoptr (i32 133 to ptr) }

; The layout is too long for an inline value, so it is a global.
@wide = global [40 x ptr] zeroinitializer
@"runtime/gc.layout:40-ffffffffff" = linkonce_odr unnamed_addr constant { i32, [5 x i8] } { i32 40, [5 x i8] c"\FF\FF\FF\FF\FF" }
@"wide$gclayout" = internal constant { i32, ptr } { i32 160, ptr @"runtime/gc.layout:40-ffffffffff" }

; The size no longer matches: stay conservative.
@changed = global [2 x ptr] zeroinitializer
@"runtime/gc.layout:48-ffffffffffff" = linkonce_odr unnamed_addr constant { i32, [6 x i8] } { i32 48, [6 x i8] c"\FF\FF\FF\FF\FF\FF" }
@"changed$gclayout" = internal constant { i32, ptr } { i32 192, ptr @"runtime/gc.layout:48-ffffffffffff" }

; A constant global is not in a writable segment.
@constant = constant [4 x { i32, ptr }] zeroinitializer
@"constant$gclayout" = internal constant { i32, ptr } { i32 32, ptr inttoptr (i32 133 to ptr) }

; The global itself was removed.
@"gone$gclayout" = internal constant { i32, ptr } { i32 32, ptr inttoptr (i32 133 to ptr) }

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
