package compile

/*
#include <stdio.h>
#include <stdlib.h>

void c_printf_str(const char* str) {
    printf("%s", str);
}
void* stellar_alloc(size_t size);
void stellar_init();
*/
import "C"
import "unsafe"

func PrintFExernal(format string) {
	cstr := C.CString(format)
	defer C.free(unsafe.Pointer(cstr))

	C.c_printf_str(cstr)
}
