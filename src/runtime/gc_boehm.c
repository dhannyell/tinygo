//go:build none

// This file is included in the build on systems that support the Boehm GC,
// despite the //go:build line above.

#include <stdint.h>

typedef void (* GC_push_other_roots_proc)(void);
void GC_set_push_other_roots(GC_push_other_roots_proc);

typedef void(* GC_warn_proc)(const char *msg, uintptr_t arg);
void GC_set_warn_proc(GC_warn_proc p);

void tinygo_runtime_bdwgc_callback(void);

static void callback(void) {
    tinygo_runtime_bdwgc_callback();
}

static void warn_proc(const char *msg, uintptr_t arg) {
}

void tinygo_runtime_bdwgc_init(void) {
    GC_set_push_other_roots(callback);
#if defined(__wasm__)
    // There are a lot of warnings on WebAssembly in the form:
    //
    //     GC Warning: Repeated allocation of very large block (appr. size 68 KiB):
    //         May lead to memory leak and poor performance
    //
    // The usual advice is to use something like GC_malloc_ignore_off_page but
    // unfortunately for most allocations that's not allowed: Go allocations can
    // legitimately hold pointers further than one page in the allocation. So
    // instead we just disable the warning.
    GC_set_warn_proc(warn_proc);
#endif
}

// Finalizers run on demand: the Go side drains them with no GC lock held.
typedef void (*GC_finalization_proc)(void *obj, void *client_data);
void GC_register_finalizer(void *obj, GC_finalization_proc fn, void *cd, GC_finalization_proc *ofn, void **ocd);
void GC_set_finalize_on_demand(int);

void tinygo_runtime_bdwgc_finalizer(void *obj, void *cd);

static void finalizer_proc(void *obj, void *cd) {
    tinygo_runtime_bdwgc_finalizer(obj, cd);
}

void tinygo_runtime_bdwgc_set_finalizer(void *obj, void *cd) {
    GC_register_finalizer(obj, cd ? finalizer_proc : 0, cd, 0, 0);
}

void tinygo_runtime_bdwgc_init_finalizers(void) {
    GC_set_finalize_on_demand(1);
}

static volatile int tinygo_alloc_lock_word;
static unsigned long long tinygo_alloc_lock_takes, tinygo_alloc_lock_spins;
void tinygo_runtime_alloc_lock(void) {
    unsigned long long spins = 0;
    while (__atomic_exchange_n(&tinygo_alloc_lock_word, 1, __ATOMIC_ACQUIRE)) {
        spins++;
    }
    tinygo_alloc_lock_takes++;
    tinygo_alloc_lock_spins += spins;
}
unsigned long long tinygo_runtime_alloc_lock_takes(void) { return tinygo_alloc_lock_takes; }
unsigned long long tinygo_runtime_alloc_lock_spins(void) { return tinygo_alloc_lock_spins; }
void tinygo_runtime_alloc_unlock(void) {
    __atomic_store_n(&tinygo_alloc_lock_word, 0, __ATOMIC_RELEASE);
}
