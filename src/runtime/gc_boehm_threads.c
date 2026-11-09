//go:build none

// This file is compiled as part of bdwgc on WebAssembly, despite the //go:build
// line above, because it needs the private bdwgc headers.

#include "private/gc_priv.h"

void tinygo_runtime_alloc_lock(void);
void tinygo_runtime_alloc_unlock(void);

// Allocate a list of objects with the block work outside the lock. The block
// belongs to this thread once it leaves the shared lists. The GC must be off.
void *tinygo_malloc_many_split(size_t lb) {
    void *op;
    void *p;
    void **opp;
    size_t lg = ALLOC_REQUEST_GRANS(lb);
    size_t lb_adjusted = GRANULES_TO_BYTES(lg);
    word my_bytes_allocd = 0;
    struct obj_kind *ok = &GC_obj_kinds[NORMAL];
    struct hblk **rlh;
    struct hblk *h;

    tinygo_runtime_alloc_lock();
    rlh = ok->ok_reclaim_list;
    if (rlh != NULL) {
        struct hblk *hbp;
        hdr *hhdr;

        while ((hbp = rlh[lg]) != NULL) {
            hhdr = HDR(hbp);
            rlh[lg] = hhdr->hb_next;
            hhdr->hb_last_reclaimed = (unsigned short)GC_gc_no;
            tinygo_runtime_alloc_unlock();
            my_bytes_allocd = 0;
            op = GC_reclaim_generic(hbp, hhdr, lb_adjusted, ok->ok_init, 0,
                                    &my_bytes_allocd);
            tinygo_runtime_alloc_lock();
            if (op != 0) {
                GC_bytes_found += (GC_signed_word)my_bytes_allocd;
                GC_bytes_allocd += my_bytes_allocd;
                tinygo_runtime_alloc_unlock();
                return op;
            }
            rlh = ok->ok_reclaim_list;
            if (rlh == NULL) {
                break;
            }
        }
    }
    opp = &ok->ok_freelist[lg];
    if ((op = *opp) != NULL) {
        *opp = NULL;
        my_bytes_allocd = 0;
        for (p = op; p != NULL; p = obj_link(p)) {
            my_bytes_allocd += lb_adjusted;
            if ((word)my_bytes_allocd >= HBLKSIZE) {
                *opp = obj_link(p);
                obj_link(p) = NULL;
                break;
            }
        }
        GC_bytes_allocd += my_bytes_allocd;
        tinygo_runtime_alloc_unlock();
        return op;
    }
    h = GC_allochblk(lb_adjusted, NORMAL, 0, 0);
    if (h != NULL) {
        GC_bytes_allocd += HBLKSIZE - (HBLKSIZE % lb_adjusted);
        tinygo_runtime_alloc_unlock();
        return GC_build_fl(h, NULL, lg, ok->ok_init || GC_debugging_started);
    }
    op = GC_generic_malloc_inner(lb_adjusted - EXTRA_BYTES, NORMAL, 0);
    if (op != NULL) {
        obj_link(op) = NULL;
    }
    tinygo_runtime_alloc_unlock();
    return op;
}
