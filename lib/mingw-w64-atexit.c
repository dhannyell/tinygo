// atexit for UCRT programs. TinyGo does not link the mingw-w64 startup code
// (crtexe.c) that normally defines it.
#include <stdlib.h>
#include <corecrt_startup.h>

int __cdecl atexit(_PVFV func) {
    return _crt_atexit(func);
}
