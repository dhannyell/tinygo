package main

/*
// Function signatures.
int foo(int a, int b);
void variadic0();
void variadic2(int x, int y, ...);
void variadicNoescape(int *a, ...);
static void staticfunc(int x);

// Global variable signatures.
extern int someValue;

void notEscapingFunction(int *a);

#cgo noescape notEscapingFunction
#cgo noescape variadicNoescape

// A macro that names a function, like the A/W names in windows.h.
int pickedA(int x);
#define PICK_A(name) name##A
#define picked PICK_A(picked)

struct point { int x, y; };
*/
import "C"

import "unsafe"

// Test function signatures.
func accessFunctions() {
	C.foo(3, 4)
	C.variadic0()
	C.variadic2(3, 5)
	C.variadicNoescape(nil)
	C.staticfunc(3)
	C.notEscapingFunction(nil)
	C.picked(1)
}

func accessGlobals() {
	_ = C.someValue
}

// Exported functions go into _cgo_export.h.
//
//export exportedC
func exportedC(a C.int, s *C.char, u C.uint, p *C.struct_point) C.longlong {
	return 0
}

//export exportedGo
func exportedGo(n int, f float64, ok bool, p *[4]byte, raw unsafe.Pointer) {
}

//export exportedPair
func exportedPair() (C.int, C.int) {
	return 0, 0
}
