#include <stdio.h>
#include <gc.h>


void* stellar_alloc(size_t size) {
    return GC_malloc(size);
}


void stellar_init() {
    GC_init();
}
