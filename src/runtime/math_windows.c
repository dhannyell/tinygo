//go:build none

// This file is included on Windows (despite the //go:build line above).

#include <math.h>

// LLVM joins sin and cos of one value into a sincos call, but the ucrt has no
// sincos. no_builtin stops LLVM from joining the two calls here again.
__attribute__((no_builtin("sin", "cos")))
void sincos(double x, double *s, double *c) {
	*s = sin(x);
	*c = cos(x);
}
