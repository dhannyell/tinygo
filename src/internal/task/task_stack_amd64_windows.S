// Windows on amd64 has a slightly different ABI than other (*nix) systems.
// Therefore, assembly functions need to be tweaked slightly.
//
// The calling convention is described here:
// https://docs.microsoft.com/en-us/cpp/build/x64-calling-convention?view=msvc-170

.section .text.tinygo_startTask,"ax"
.global  tinygo_startTask
tinygo_startTask:
    // Small assembly stub for starting a goroutine. This is already run on the
    // new stack, with the callee-saved registers already loaded.
    // Most importantly, r12 contain the pc of the to-be-started function and
    // r13 contain the only argument it is given. Multiple arguments are packed
    // into one by storing them in a new allocation.

    // Set the first argument of the goroutine start wrapper, which contains all
    // the arguments.
    movq %r13, %rcx

    // Branch to the "goroutine start" function.
    callq *%r12

    // After return, exit this goroutine.
    // This has to be a call, not a jump, to keep the stack correctly aligned.
    callq tinygo_task_exit

.global tinygo_swapTask
.section .text.tinygo_swapTask,"ax"
tinygo_swapTask:
    // This function gets the following parameters:
    // %rcx = newStack uintptr
    // %rdx = oldStack *uintptr

    // Save all callee-saved registers:
    pushq %r15
    pushq %r14
    pushq %r13
    pushq %r12
    pushq %rsi
    pushq %rdi
    pushq %rbp
    sub   $160, %rsp
    movaps  %xmm6, 144(%rsp)
    movaps  %xmm7, 128(%rsp)
    movaps  %xmm8, 112(%rsp)
    movaps  %xmm9,  96(%rsp)
    movaps  %xmm10, 80(%rsp)
    movaps  %xmm11, 64(%rsp)
    movaps  %xmm12, 48(%rsp)
    movaps  %xmm13, 32(%rsp)
    movaps  %xmm14, 16(%rsp)
    movaps  %xmm15,  0(%rsp)
    pushq %rbx

    // Save the current stack pointer in oldStack.
    movq %rsp, (%rdx)

    // Switch to the new stack pointer.
    movq %rcx, %rsp

    // Load saved register from the new stack.
    popq %rbx
    movaps 0(%rsp),   %xmm15
    movaps 16(%rsp),  %xmm14
    movaps 32(%rsp),  %xmm13
    movaps 48(%rsp),  %xmm12
    movaps 64(%rsp),  %xmm11
    movaps 80(%rsp),  %xmm10
    movaps 96(%rsp),  %xmm9
    movaps 112(%rsp), %xmm8
    movaps 128(%rsp), %xmm7
    movaps 144(%rsp), %xmm6
    add   $160, %rsp
    popq %rbp
    popq %rdi
    popq %rsi
    popq %r12
    popq %r13
    popq %r14
    popq %r15

    // Return into the new task, as if tinygo_swapTask was a regular call.
    ret

.global tinygo_callOnStack
.section .text.tinygo_callOnStack,"ax"
tinygo_callOnStack:
.seh_proc tinygo_callOnStack
    // This function gets the following parameters:
    // %rcx = fn, called with one argument
    // %rdx = arg
    // %r8  = new stack pointer, or 0 to stay on the current stack
    // %r9  = oldSP *uintptr, written only when %r8 is not 0
    pushq %rbp
    .seh_pushreg %rbp
    movq  %rsp, %rbp
    .seh_setframe %rbp, 0
    .seh_endprologue

    testq %r8, %r8
    jz    1f
    movq  %rsp, (%r9)
    movq  %r8, %rsp
1:
    // Align the stack and reserve the shadow space for fn.
    andq  $-16, %rsp
    subq  $32, %rsp
    movq  %rcx, %rax
    movq  %rdx, %rcx
    callq *%rax

    leaq  0(%rbp), %rsp
    popq  %rbp
    retq
.seh_endproc

// tinygo_stackLimit is the lowest address that a large frame may use on the
// current goroutine stack, or 0 on the system stack.
.section .bss.tinygo_stackLimit,"bw"
.global  tinygo_stackLimit
.p2align 3
tinygo_stackLimit:
    .quad 0

.section .bss.tinygo_stackOverflowStack,"bw"
.p2align 4
tinygo_stackOverflowStack:
    .zero 65536

// ___chkstk_ms replaces the compiler-rt version. LLVM calls it for each frame
// of 4096 bytes or more, with the frame size in %rax. On a goroutine stack it
// checks the frame against tinygo_stackLimit; on the system stack it probes
// the pages like the original. It keeps all registers.
.global ___chkstk_ms
.section .text.___chkstk_ms,"ax"
___chkstk_ms:
    pushq %rcx
    pushq %rax
    movq  tinygo_stackLimit(%rip), %rcx
    testq %rcx, %rcx
    jz    2f

    // Goroutine stack: the pages are committed, so only check the bound.
    leaq  24(%rsp), %rcx
    subq  %rax, %rcx
    jb    1f
    cmpq  tinygo_stackLimit(%rip), %rcx
    jb    1f
    popq  %rax
    popq  %rcx
    ret
1:
    // Report the overflow on a separate stack: the frame that called this
    // function is not aligned yet, and the reserve may be too small.
    leaq  tinygo_stackOverflowStack+65536(%rip), %rsp
    subq  $32, %rsp
    callq tinygo_stackOverflow
    ud2

2:
    // System stack: touch each page in order, so the guard page grows it.
    leaq  24(%rsp), %rcx
    cmpq  $0x1000, %rax
    jb    4f
3:
    subq  $0x1000, %rcx
    orl   $0, (%rcx)
    subq  $0x1000, %rax
    cmpq  $0x1000, %rax
    ja    3b
4:
    subq  %rax, %rcx
    orl   $0, (%rcx)
    popq  %rax
    popq  %rcx
    ret
