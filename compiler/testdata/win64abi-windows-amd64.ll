; ModuleID = 'win64abi.go'
source_filename = "win64abi.go"
target datalayout = "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128"
target triple = "x86_64-unknown-windows-gnu"

%main.large = type { i64, i64, i64, i64 }
%main.odd = type { i8, i8, i8 }
%main.small = type { i32, i32 }

; Function Attrs: nounwind uwtable(sync)
define hidden void @main.init(ptr %context) unnamed_addr #0 {
entry:
  ret void
}

declare i64 @cSmall(i64) #1

declare void @cLarge(ptr sret(%main.large) nocapture, ptr nocapture, ptr nocapture) #1

; Function Attrs: nounwind uwtable(sync)
define hidden i64 @main.callC(ptr %context) unnamed_addr #0 {
entry:
  %stackswitch.args13 = alloca { ptr, ptr, ptr }, align 8
  %win64.arg12 = alloca %main.odd, align 8
  %win64.arg = alloca %main.large, align 8
  %win64.result = alloca %main.large, align 8
  %win64.coerce2 = alloca i64, align 8
  %stackswitch.args = alloca { i64, i64 }, align 8
  %win64.coerce = alloca %main.small, align 8
  %s = call align 4 dereferenceable(8) ptr @runtime.alloc(i64 8, ptr nonnull inttoptr (i64 3 to ptr), ptr undef) #4
  %complit = call align 4 dereferenceable(8) ptr @runtime.alloc(i64 8, ptr nonnull inttoptr (i64 3 to ptr), ptr undef) #4
  %0 = getelementptr inbounds nuw i8, ptr %complit, i64 4
  br i1 false, label %store.throw, label %store.next

store.next:                                       ; preds = %entry
  store i32 1, ptr %complit, align 4
  br i1 false, label %store.throw, label %store.next1

store.next1:                                      ; preds = %store.next
  store i32 2, ptr %0, align 4
  %.unpack = load i32, ptr %complit, align 4
  call void @llvm.lifetime.start.p0(ptr nonnull %win64.coerce)
  store i32 %.unpack, ptr %win64.coerce, align 4
  %win64.coerce.repack17 = getelementptr inbounds nuw i8, ptr %win64.coerce, i64 4
  store i32 2, ptr %win64.coerce.repack17, align 4
  %1 = load i64, ptr %win64.coerce, align 8
  call void @llvm.lifetime.end.p0(ptr nonnull %win64.coerce)
  call void @llvm.lifetime.start.p0(ptr nonnull %stackswitch.args)
  store i64 %1, ptr %stackswitch.args, align 8
  call void @runtime.cgocall(ptr nonnull @"cSmall$cgocall", ptr nonnull %stackswitch.args, ptr undef) #4
  %2 = getelementptr inbounds nuw i8, ptr %stackswitch.args, i64 8
  %3 = load i64, ptr %2, align 8
  call void @llvm.lifetime.end.p0(ptr nonnull %stackswitch.args)
  call void @llvm.lifetime.start.p0(ptr nonnull %win64.coerce2)
  store i64 %3, ptr %win64.coerce2, align 8
  %.unpack19 = load i32, ptr %win64.coerce2, align 4
  %.elt20 = getelementptr inbounds nuw i8, ptr %win64.coerce2, i64 4
  %.unpack21 = load i32, ptr %.elt20, align 4
  call void @llvm.lifetime.end.p0(ptr nonnull %win64.coerce2)
  store i32 %.unpack19, ptr %s, align 4
  %s.repack22 = getelementptr inbounds nuw i8, ptr %s, i64 4
  store i32 %.unpack21, ptr %s.repack22, align 4
  %l = call align 8 dereferenceable(32) ptr @runtime.alloc(i64 32, ptr nonnull inttoptr (i64 3 to ptr), ptr undef) #4
  %complit3 = call align 8 dereferenceable(32) ptr @runtime.alloc(i64 32, ptr nonnull inttoptr (i64 3 to ptr), ptr undef) #4
  %4 = getelementptr inbounds nuw i8, ptr %complit3, i64 8
  %5 = getelementptr inbounds nuw i8, ptr %complit3, i64 16
  %6 = getelementptr inbounds nuw i8, ptr %complit3, i64 24
  br i1 false, label %store.throw, label %store.next4

store.next4:                                      ; preds = %store.next1
  store i64 1, ptr %complit3, align 8
  br i1 false, label %store.throw, label %store.next5

store.next5:                                      ; preds = %store.next4
  store i64 2, ptr %4, align 8
  br i1 false, label %store.throw, label %store.next6

store.next6:                                      ; preds = %store.next5
  store i64 3, ptr %5, align 8
  br i1 false, label %store.throw, label %store.next7

store.next7:                                      ; preds = %store.next6
  store i64 4, ptr %6, align 8
  %.unpack24 = load i64, ptr %complit3, align 8
  %.elt25 = getelementptr inbounds nuw i8, ptr %complit3, i64 8
  %.unpack26 = load i64, ptr %.elt25, align 8
  %.elt27 = getelementptr inbounds nuw i8, ptr %complit3, i64 16
  %.unpack28 = load i64, ptr %.elt27, align 8
  %.elt29 = getelementptr inbounds nuw i8, ptr %complit3, i64 24
  %.unpack30 = load i64, ptr %.elt29, align 8
  %complit8 = call align 1 dereferenceable(3) ptr @runtime.alloc(i64 3, ptr nonnull inttoptr (i64 3 to ptr), ptr undef) #4
  %7 = getelementptr inbounds nuw i8, ptr %complit8, i64 1
  %8 = getelementptr inbounds nuw i8, ptr %complit8, i64 2
  br i1 false, label %store.throw, label %store.next9

store.next9:                                      ; preds = %store.next7
  store i8 5, ptr %complit8, align 1
  br i1 false, label %store.throw, label %store.next10

store.next10:                                     ; preds = %store.next9
  store i8 6, ptr %7, align 1
  br i1 false, label %store.throw, label %store.next11

store.next11:                                     ; preds = %store.next10
  store i8 7, ptr %8, align 1
  %.unpack31 = load i8, ptr %complit8, align 1
  %.elt32 = getelementptr inbounds nuw i8, ptr %complit8, i64 1
  %.unpack33 = load i8, ptr %.elt32, align 1
  call void @llvm.lifetime.start.p0(ptr nonnull %win64.result)
  call void @llvm.lifetime.start.p0(ptr nonnull %win64.arg)
  store i64 %.unpack24, ptr %win64.arg, align 8
  %win64.arg.repack36 = getelementptr inbounds nuw i8, ptr %win64.arg, i64 8
  store i64 %.unpack26, ptr %win64.arg.repack36, align 8
  %win64.arg.repack38 = getelementptr inbounds nuw i8, ptr %win64.arg, i64 16
  store i64 %.unpack28, ptr %win64.arg.repack38, align 8
  %win64.arg.repack40 = getelementptr inbounds nuw i8, ptr %win64.arg, i64 24
  store i64 %.unpack30, ptr %win64.arg.repack40, align 8
  call void @llvm.lifetime.start.p0(ptr nonnull %win64.arg12)
  store i8 %.unpack31, ptr %win64.arg12, align 1
  %win64.arg12.repack42 = getelementptr inbounds nuw i8, ptr %win64.arg12, i64 1
  store i8 %.unpack33, ptr %win64.arg12.repack42, align 1
  %win64.arg12.repack44 = getelementptr inbounds nuw i8, ptr %win64.arg12, i64 2
  store i8 7, ptr %win64.arg12.repack44, align 1
  call void @llvm.lifetime.start.p0(ptr nonnull %stackswitch.args13)
  store ptr %win64.result, ptr %stackswitch.args13, align 8
  %9 = getelementptr inbounds nuw i8, ptr %stackswitch.args13, i64 8
  store ptr %win64.arg, ptr %9, align 8
  %10 = getelementptr inbounds nuw i8, ptr %stackswitch.args13, i64 16
  store ptr %win64.arg12, ptr %10, align 8
  call void @runtime.cgocall(ptr nonnull @"cLarge$cgocall", ptr nonnull %stackswitch.args13, ptr undef) #4
  call void @llvm.lifetime.end.p0(ptr nonnull %stackswitch.args13)
  %.unpack46 = load i64, ptr %win64.result, align 8
  %.elt47 = getelementptr inbounds nuw i8, ptr %win64.result, i64 8
  %.unpack48 = load i64, ptr %.elt47, align 8
  %.elt49 = getelementptr inbounds nuw i8, ptr %win64.result, i64 16
  %.unpack50 = load i64, ptr %.elt49, align 8
  %.elt51 = getelementptr inbounds nuw i8, ptr %win64.result, i64 24
  %.unpack52 = load i64, ptr %.elt51, align 8
  call void @llvm.lifetime.end.p0(ptr nonnull %win64.result)
  call void @llvm.lifetime.end.p0(ptr nonnull %win64.arg)
  call void @llvm.lifetime.end.p0(ptr nonnull %win64.arg12)
  store i64 %.unpack46, ptr %l, align 8
  %l.repack53 = getelementptr inbounds nuw i8, ptr %l, i64 8
  store i64 %.unpack48, ptr %l.repack53, align 8
  %l.repack55 = getelementptr inbounds nuw i8, ptr %l, i64 16
  store i64 %.unpack50, ptr %l.repack55, align 8
  %l.repack57 = getelementptr inbounds nuw i8, ptr %l, i64 24
  store i64 %.unpack52, ptr %l.repack57, align 8
  br i1 false, label %store.throw, label %deref.next

deref.next:                                       ; preds = %store.next11
  br i1 false, label %store.throw, label %deref.next14

deref.next14:                                     ; preds = %deref.next
  %11 = getelementptr inbounds nuw i8, ptr %l, i64 24
  %12 = load i32, ptr %s, align 4
  %13 = sext i32 %12 to i64
  %14 = load i64, ptr %11, align 8
  %15 = add i64 %14, %13
  ret i64 %15

store.throw:                                      ; preds = %deref.next, %store.next11, %store.next10, %store.next9, %store.next7, %store.next6, %store.next5, %store.next4, %store.next1, %store.next, %entry
  unreachable
}

; Function Attrs: allockind("alloc,zeroed") allocsize(0)
declare noalias nonnull ptr @runtime.alloc(i64, ptr, ptr) #2

declare void @runtime.nilPanic(ptr) #1

; Function Attrs: nocallback nofree nosync nounwind willreturn memory(argmem: readwrite)
declare void @llvm.lifetime.start.p0(ptr nocapture) #3

; Function Attrs: nocallback nofree nosync nounwind willreturn memory(argmem: readwrite)
declare void @llvm.lifetime.end.p0(ptr nocapture) #3

; Function Attrs: nounwind uwtable(sync)
define internal void @"cSmall$cgocall"(ptr %0) unnamed_addr #0 {
entry:
  %1 = load i64, ptr %0, align 8
  %2 = call i64 @cSmall(i64 %1) #4
  %3 = getelementptr inbounds nuw i8, ptr %0, i64 8
  store i64 %2, ptr %3, align 8
  ret void
}

declare void @runtime.cgocall(ptr, ptr, ptr) #1

; Function Attrs: nounwind uwtable(sync)
define internal void @"cLarge$cgocall"(ptr %0) unnamed_addr #0 {
entry:
  %1 = load ptr, ptr %0, align 8
  %2 = getelementptr inbounds nuw i8, ptr %0, i64 8
  %3 = load ptr, ptr %2, align 8
  %4 = getelementptr inbounds nuw i8, ptr %0, i64 16
  %5 = load ptr, ptr %4, align 8
  call void @cLarge(ptr sret(%main.large) %1, ptr %3, ptr %5) #4
  ret void
}

; Function Attrs: nounwind uwtable(sync)
define i64 @goSmall(i64 %s) #0 {
cgocallback.check:
  %win64.coerce4 = alloca %main.small, align 8
  %win64.coerce = alloca i64, align 8
  %stackswitch.args = alloca { i64, i64 }, align 8
  %0 = call i1 @runtime.cgocallbackSwitch(ptr undef) #4
  br i1 %0, label %cgocallback.switch, label %entry

cgocallback.switch:                               ; preds = %cgocallback.check
  call void @llvm.lifetime.start.p0(ptr nonnull %stackswitch.args)
  store i64 %s, ptr %stackswitch.args, align 8
  call void @runtime.cgocallback(ptr nonnull @"goSmall$cgocallback", ptr nonnull %stackswitch.args, ptr undef) #4
  %1 = getelementptr inbounds nuw i8, ptr %stackswitch.args, i64 8
  %2 = load i64, ptr %1, align 8
  call void @llvm.lifetime.end.p0(ptr nonnull %stackswitch.args)
  ret i64 %2

entry:                                            ; preds = %cgocallback.check
  call void @llvm.lifetime.start.p0(ptr nonnull %win64.coerce)
  store i64 %s, ptr %win64.coerce, align 8
  %.unpack = load i32, ptr %win64.coerce, align 4
  %.elt5 = getelementptr inbounds nuw i8, ptr %win64.coerce, i64 4
  %.unpack6 = load i32, ptr %.elt5, align 4
  call void @llvm.lifetime.end.p0(ptr nonnull %win64.coerce)
  %s1 = call align 4 dereferenceable(8) ptr @runtime.alloc(i64 8, ptr nonnull inttoptr (i64 3 to ptr), ptr undef) #4
  store i32 %.unpack, ptr %s1, align 4
  %s1.repack7 = getelementptr inbounds nuw i8, ptr %s1, i64 4
  store i32 %.unpack6, ptr %s1.repack7, align 4
  %complit = call align 4 dereferenceable(8) ptr @runtime.alloc(i64 8, ptr nonnull inttoptr (i64 3 to ptr), ptr undef) #4
  br i1 false, label %deref.throw, label %deref.next

deref.next:                                       ; preds = %entry
  %3 = getelementptr inbounds nuw i8, ptr %s1, i64 4
  %4 = load i32, ptr %3, align 4
  br i1 false, label %deref.throw, label %deref.next2

deref.next2:                                      ; preds = %deref.next
  %5 = load i32, ptr %s1, align 4
  br i1 false, label %deref.throw, label %store.next

store.next:                                       ; preds = %deref.next2
  store i32 %4, ptr %complit, align 4
  br i1 false, label %deref.throw, label %store.next3

store.next3:                                      ; preds = %store.next
  %6 = getelementptr inbounds nuw i8, ptr %complit, i64 4
  store i32 %5, ptr %6, align 4
  %.unpack9 = load i32, ptr %complit, align 4
  call void @llvm.lifetime.start.p0(ptr nonnull %win64.coerce4)
  store i32 %.unpack9, ptr %win64.coerce4, align 4
  %win64.coerce4.repack12 = getelementptr inbounds nuw i8, ptr %win64.coerce4, i64 4
  store i32 %5, ptr %win64.coerce4.repack12, align 4
  %7 = load i64, ptr %win64.coerce4, align 8
  call void @llvm.lifetime.end.p0(ptr nonnull %win64.coerce4)
  ret i64 %7

deref.throw:                                      ; preds = %store.next, %deref.next2, %deref.next, %entry
  unreachable
}

declare i1 @runtime.cgocallbackSwitch(ptr) #1

; Function Attrs: nounwind uwtable(sync)
define internal void @"goSmall$cgocallback"(ptr %0) unnamed_addr #0 {
entry:
  %1 = load i64, ptr %0, align 8
  %2 = call i64 @goSmall(i64 %1)
  %3 = getelementptr inbounds nuw i8, ptr %0, i64 8
  store i64 %2, ptr %3, align 8
  ret void
}

declare void @runtime.cgocallback(ptr, ptr, ptr) #1

; Function Attrs: nounwind uwtable(sync)
define void @goLarge(ptr sret(%main.large) nocapture %return, ptr nocapture %l, ptr nocapture %o) #0 {
cgocallback.check:
  %stackswitch.args = alloca { ptr, ptr, ptr }, align 8
  %0 = call i1 @runtime.cgocallbackSwitch(ptr undef) #4
  br i1 %0, label %cgocallback.switch, label %entry

cgocallback.switch:                               ; preds = %cgocallback.check
  call void @llvm.lifetime.start.p0(ptr nonnull %stackswitch.args)
  store ptr %return, ptr %stackswitch.args, align 8
  %1 = getelementptr inbounds nuw i8, ptr %stackswitch.args, i64 8
  store ptr %l, ptr %1, align 8
  %2 = getelementptr inbounds nuw i8, ptr %stackswitch.args, i64 16
  store ptr %o, ptr %2, align 8
  call void @runtime.cgocallback(ptr nonnull @"goLarge$cgocallback", ptr nonnull %stackswitch.args, ptr undef) #4
  call void @llvm.lifetime.end.p0(ptr nonnull %stackswitch.args)
  ret void

entry:                                            ; preds = %cgocallback.check
  %.unpack = load i64, ptr %l, align 8
  %.elt4 = getelementptr inbounds nuw i8, ptr %l, i64 8
  %.unpack5 = load i64, ptr %.elt4, align 8
  %.elt6 = getelementptr inbounds nuw i8, ptr %l, i64 16
  %.unpack7 = load i64, ptr %.elt6, align 8
  %.elt8 = getelementptr inbounds nuw i8, ptr %l, i64 24
  %.unpack9 = load i64, ptr %.elt8, align 8
  %.unpack10 = load i8, ptr %o, align 1
  %.elt11 = getelementptr inbounds nuw i8, ptr %o, i64 1
  %.unpack12 = load i8, ptr %.elt11, align 1
  %.elt13 = getelementptr inbounds nuw i8, ptr %o, i64 2
  %.unpack14 = load i8, ptr %.elt13, align 1
  %l1 = call align 8 dereferenceable(32) ptr @runtime.alloc(i64 32, ptr nonnull inttoptr (i64 3 to ptr), ptr undef) #4
  store i64 %.unpack, ptr %l1, align 8
  %l1.repack15 = getelementptr inbounds nuw i8, ptr %l1, i64 8
  store i64 %.unpack5, ptr %l1.repack15, align 8
  %l1.repack17 = getelementptr inbounds nuw i8, ptr %l1, i64 16
  store i64 %.unpack7, ptr %l1.repack17, align 8
  %l1.repack19 = getelementptr inbounds nuw i8, ptr %l1, i64 24
  store i64 %.unpack9, ptr %l1.repack19, align 8
  %o2 = call align 1 dereferenceable(3) ptr @runtime.alloc(i64 3, ptr nonnull inttoptr (i64 3 to ptr), ptr undef) #4
  store i8 %.unpack10, ptr %o2, align 1
  %o2.repack21 = getelementptr inbounds nuw i8, ptr %o2, i64 1
  store i8 %.unpack12, ptr %o2.repack21, align 1
  %o2.repack23 = getelementptr inbounds nuw i8, ptr %o2, i64 2
  store i8 %.unpack14, ptr %o2.repack23, align 1
  br i1 false, label %deref.throw, label %deref.next

deref.next:                                       ; preds = %entry
  br i1 false, label %deref.throw, label %deref.next3

deref.next3:                                      ; preds = %deref.next
  br i1 false, label %deref.throw, label %store.next

store.next:                                       ; preds = %deref.next3
  %3 = load i64, ptr %l1, align 8
  %4 = getelementptr inbounds nuw i8, ptr %o2, i64 2
  %5 = load i8, ptr %4, align 1
  %6 = zext i8 %5 to i64
  %7 = add i64 %3, %6
  store i64 %7, ptr %l1, align 8
  %.elt26 = getelementptr inbounds nuw i8, ptr %l1, i64 8
  %.unpack27 = load i64, ptr %.elt26, align 8
  %.elt28 = getelementptr inbounds nuw i8, ptr %l1, i64 16
  %.unpack29 = load i64, ptr %.elt28, align 8
  %.elt30 = getelementptr inbounds nuw i8, ptr %l1, i64 24
  %.unpack31 = load i64, ptr %.elt30, align 8
  store i64 %7, ptr %return, align 8
  %return.repack32 = getelementptr inbounds nuw i8, ptr %return, i64 8
  store i64 %.unpack27, ptr %return.repack32, align 8
  %return.repack34 = getelementptr inbounds nuw i8, ptr %return, i64 16
  store i64 %.unpack29, ptr %return.repack34, align 8
  %return.repack36 = getelementptr inbounds nuw i8, ptr %return, i64 24
  store i64 %.unpack31, ptr %return.repack36, align 8
  ret void

deref.throw:                                      ; preds = %deref.next3, %deref.next, %entry
  unreachable
}

; Function Attrs: nounwind uwtable(sync)
define internal void @"goLarge$cgocallback"(ptr %0) unnamed_addr #0 {
entry:
  %1 = load ptr, ptr %0, align 8
  %2 = getelementptr inbounds nuw i8, ptr %0, i64 8
  %3 = load ptr, ptr %2, align 8
  %4 = getelementptr inbounds nuw i8, ptr %0, i64 16
  %5 = load ptr, ptr %4, align 8
  call void @goLarge(ptr sret(%main.large) %1, ptr %3, ptr %5)
  ret void
}

attributes #0 = { nounwind uwtable(sync) "target-features"="+cmov,+cx8,+fxsr,+mmx,+sse,+sse2,+x87" }
attributes #1 = { "target-features"="+cmov,+cx8,+fxsr,+mmx,+sse,+sse2,+x87" }
attributes #2 = { allockind("alloc,zeroed") allocsize(0) "alloc-family"="runtime.alloc" "target-features"="+cmov,+cx8,+fxsr,+mmx,+sse,+sse2,+x87" }
attributes #3 = { nocallback nofree nosync nounwind willreturn memory(argmem: readwrite) }
attributes #4 = { nounwind }
