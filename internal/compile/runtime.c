#include <stddef.h>
#include <inttypes.h>
#include <stdio.h>
#include <gc.h>


void* stellar_alloc(size_t size) {
    return GC_malloc(size);
}


void stellar_init() {
    GC_init();
}

void stellar_print(const char *s) {
    fputs(s, stdout);
    fputc('\n', stdout);
}

char* stellar_int_string(int64_t v) {
    size_t bufsize = 21;
    char *buffer = (char *)GC_malloc_atomic(bufsize);

    if (buffer == NULL) {
        return buffer;
    }

    snprintf(buffer, bufsize, "%" PRId64, v);
    return buffer;
}

char* stellar_float_string(double v) {
    size_t bufsize = 500;
    char *buffer = (char *)GC_malloc_atomic(bufsize);

    snprintf(buffer, bufsize, "%f", v);

    return buffer;
}

char* stellar_bool_string(int v) {
    if (v == 0) {
        return "false";
    } else {
        return "true";
    }
}
