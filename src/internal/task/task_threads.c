//go:build none

#ifdef _WIN32

#include <windows.h>
#include <stdint.h>

#define MIN_THREAD_STACK (1024 * 1024)

// TLS slots for the current task.Task and for the handle of the thread.
static DWORD current_task_slot;
static DWORD thread_handle_slot;

struct state_pass {
    void      *(*start)(void*);
    void      *args;
    void      *task;
    uintptr_t *stackTop;
    HANDLE    thread;
    HANDLE    startlock;
};

void tinygo_task_init(void *mainTask, HANDLE *thread, int *numCPU, void *context) {
    current_task_slot = TlsAlloc();
    thread_handle_slot = TlsAlloc();
    TlsSetValue(current_task_slot, mainTask);

    // GetCurrentThread returns a pseudo handle, which other threads cannot use.
    DuplicateHandle(GetCurrentProcess(), GetCurrentThread(), GetCurrentProcess(),
                    thread, 0, FALSE, DUPLICATE_SAME_ACCESS);
    TlsSetValue(thread_handle_slot, *thread);

    int num = (int)GetActiveProcessorCount(ALL_PROCESSOR_GROUPS);
    if (num <= 0) {
        num = 1;
    }
    *numCPU = num;
}

void tinygo_task_exited(void*);
void tinygo_task_exit(void);

static DWORD WINAPI start_wrapper(LPVOID arg) {
    struct state_pass *state = arg;
    void *(*start)(void*) = state->start;
    void *args = state->args;
    TlsSetValue(current_task_slot, state->task);
    TlsSetValue(thread_handle_slot, state->thread);

    // Save the current stack pointer in the goroutine state, for the GC.
    int stackAddr;
    *(state->stackTop) = (uintptr_t)(&stackAddr);

    // The caller may free state after this.
    SetEvent(state->startlock);

    start(args);
    tinygo_task_exited(TlsGetValue(current_task_slot));
    tinygo_task_exit();
    return 0;
}

int tinygo_task_start(uintptr_t fn, void *args, void *task, HANDLE *thread, uintptr_t *stackTop, uintptr_t stackSize, void *context) {
    struct state_pass state = {
        .start     = (void*)fn,
        .args      = args,
        .task      = task,
        .stackTop  = stackTop,
    };
    state.startlock = CreateEventW(NULL, FALSE, FALSE, NULL);
    if (state.startlock == NULL) {
        return 1;
    }

    // C code such as a GPU driver also runs on this stack. The size is only a
    // reservation: pages are committed when the thread uses them.
    if (stackSize < MIN_THREAD_STACK) {
        stackSize = MIN_THREAD_STACK;
    }
    HANDLE h = CreateThread(NULL, stackSize, start_wrapper, &state,
                            CREATE_SUSPENDED | STACK_SIZE_PARAM_IS_A_RESERVATION, NULL);
    if (h == NULL) {
        CloseHandle(state.startlock);
        return 1;
    }
    state.thread = h;
    *thread = h;
    ResumeThread(h);

    WaitForSingleObject(state.startlock, INFINITE);
    CloseHandle(state.startlock);
    return 0;
}

void* tinygo_task_current(void) {
    return TlsGetValue(current_task_slot);
}

// Exit the current thread. The task is no longer in the task list, so the GC
// does not use the handle after this.
void tinygo_task_exit(void) {
    CloseHandle(TlsGetValue(thread_handle_slot));
    ExitThread(0);
}

// Stop a thread for the GC mark phase. It may run on for a short time.
void tinygo_task_suspend(HANDLE thread) {
    SuspendThread(thread);
}

// Wait until a suspended thread is stopped, and store its registers in buf.
// buf must hold a CONTEXT plus 15 bytes for the alignment. Return the stack
// pointer of the thread.
uintptr_t tinygo_task_get_context(HANDLE thread, void *buf) {
    CONTEXT *ctx = (CONTEXT*)(((uintptr_t)buf + 15) & ~(uintptr_t)15);
    ctx->ContextFlags = CONTEXT_FULL;
    if (!GetThreadContext(thread, ctx)) {
        return 0;
    }
    #if defined(_M_X64) || defined(__x86_64__)
    return ctx->Rsp;
    #else
    #error "unsupported architecture"
    #endif
}

void tinygo_task_resume(HANDLE thread) {
    ResumeThread(thread);
}

#else // _WIN32

#define _GNU_SOURCE
#include <pthread.h>
#include <signal.h>
#include <stdint.h>
#include <stdio.h>
#include <unistd.h>

#ifdef __linux__
#include <semaphore.h>

// BDWGC also uses SIGRTMIN+6 on Linux, which seems like a reasonable choice.
#define taskPauseSignal (SIGRTMIN + 6)

#elif __APPLE__
#include <dispatch/dispatch.h>
// SIGIO is for interrupt-driven I/O.
// I don't think anybody should be using this nowadays, so I think we can
// repurpose it as a signal for GC.
// BDWGC uses a special way to pause/resume other threads on MacOS, which may be
// better but needs more work. Using signal keeps the code similar between Linux
// and MacOS.
#define taskPauseSignal SIGIO

#endif // __linux__, __APPLE__

// Pointer to the current task.Task structure.
// Ideally the entire task.Task structure would be a thread-local variable but
// this also works.
static __thread void *current_task;

struct state_pass {
    void      *(*start)(void*);
    void      *args;
    void      *task;
    uintptr_t *stackTop;
    #if __APPLE__
    dispatch_semaphore_t startlock;
    #else
    sem_t     startlock;
    #endif
};

// Handle the GC pause in Go.
void tinygo_task_gc_pause(int sig);

// Initialize the main thread.
void tinygo_task_init(void *mainTask, pthread_t *thread, int *numCPU, void *context) {
    // Make sure the current task pointer is set correctly for the main
    // goroutine as well.
    current_task = mainTask;

    // Store the thread ID of the main thread.
    *thread = pthread_self();

    // Register the "GC pause" signal for the entire process.
    // Using pthread_kill, we can still send the signal to a specific thread.
    struct sigaction act = { 0 };
    act.sa_handler = tinygo_task_gc_pause;
    act.sa_flags = SA_RESTART;
    sigaction(taskPauseSignal, &act, NULL);

    // Obtain the number of CPUs available on program start (for NumCPU).
    int num = sysconf(_SC_NPROCESSORS_ONLN);
    if (num <= 0) {
        // Fallback in case there is an error.
        num = 1;
    }
    *numCPU = num;
}

void tinygo_task_exited(void*);

// Helper to start a goroutine while also storing the 'task' structure.
static void* start_wrapper(void *arg) {
    struct state_pass *state = arg;
    void *(*start)(void*) = state->start;
    void *args = state->args;
    current_task = state->task;

    // Save the current stack pointer in the goroutine state, for the GC.
    int stackAddr;
    *(state->stackTop) = (uintptr_t)(&stackAddr);

    // Notify the caller that the thread has successfully started and
    // initialized.
    #if __APPLE__
    dispatch_semaphore_signal(state->startlock);
    #else
    sem_post(&state->startlock);
    #endif

    // Run the goroutine function.
    start(args);

    // Notify the Go side this thread will exit.
    tinygo_task_exited(current_task);

    return NULL;
};

// Start a new goroutine in an OS thread.
int tinygo_task_start(uintptr_t fn, void *args, void *task, pthread_t *thread, uintptr_t *stackTop, uintptr_t stackSize, void *context) {
    // Sanity check. Should get optimized away.
    if (sizeof(pthread_t) != sizeof(void*)) {
        __builtin_trap();
    }

    struct state_pass state = {
        .start     = (void*)fn,
        .args      = args,
        .task      = task,
        .stackTop  = stackTop,
    };
    #if __APPLE__
    state.startlock = dispatch_semaphore_create(0);
    #else
    sem_init(&state.startlock, 0, 0);
    #endif
    pthread_attr_t attrs;
    pthread_attr_init(&attrs);
	pthread_attr_setdetachstate(&attrs, PTHREAD_CREATE_DETACHED);
    pthread_attr_setstacksize(&attrs, stackSize);
    int result = pthread_create(thread, &attrs, &start_wrapper, &state);
    pthread_attr_destroy(&attrs);
	if (result != 0) {
		return result;
	}

    // Wait until the thread has been created and read all state_pass variables.
    #if __APPLE__
    dispatch_semaphore_wait(state.startlock, DISPATCH_TIME_FOREVER);
    dispatch_release(state.startlock);
    #else
    sem_wait(&state.startlock);
    sem_destroy(&state.startlock);
    #endif

    return result;
}

// Return the current task (for task.Current()).
void* tinygo_task_current(void) {
    return current_task;
}

// Exit the current thread.
void tinygo_task_exit(void) {
    pthread_exit(NULL);
}

// Send a signal to cause the task to pause for the GC mark phase.
void tinygo_task_send_gc_signal(pthread_t thread) {
    pthread_kill(thread, taskPauseSignal);
}

#endif // _WIN32
