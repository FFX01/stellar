package compile

/*
#include <stdio.h>
#include <stdlib.h>

void stellar_print(const char *s);
void* stellar_alloc(size_t size);
void stellar_init();
*/
import "C"
import "unsafe"

func PrintFExernal(format string) {
	cstr := C.CString(format)
	defer C.free(unsafe.Pointer(cstr))

	C.stellar_print(cstr)
}
