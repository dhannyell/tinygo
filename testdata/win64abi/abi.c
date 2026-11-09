#include "abi.h"

// Each function changes its copy of the argument. The caller must not see it.

s3 abi_s3(s3 v) {
	s3 r = {v.c, v.b, v.a};
	v.a = 0;
	return r;
}

s4 abi_s4(s4 v) {
	v.a += 1;
	return v;
}

s8 abi_s8(s8 v) {
	s8 r = {v.b, v.a};
	v.a = 0;
	return r;
}

s12 abi_s12(s12 v) {
	s12 r = {v.c, v.b, v.a};
	v.a = 0;
	return r;
}

s16 abi_s16(s16 v) {
	v.a += v.b;
	return v;
}

s40 abi_s40(s40 v) {
	v.e = v.a + v.b + v.c + v.d;
	return v;
}

long long abi_mixed(int i, s16 a, s3 b, double d, s40 c) {
	return i + a.a * 10 + a.b * 100 + b.c * 1000 + (long long)d * 10000 + c.e * 100000;
}

long long abi_callback(void) {
	s16 x = {10, 20};
	s40 y = {1, 2, 3, 4, 5};
	s8 r = goSum(x, y);
	s40 z = goPick(7, y);
	return r.a * 1000000 + r.b * 10000 + z.a * 100 + z.e * 10 + y.a;
}

// Uses more stack than a goroutine has.
long long abi_big_stack(int seed) {
	volatile char buf[256 * 1024];
	for (int i = 0; i < (int)sizeof(buf); i += 4096) {
		buf[i] = (char)(seed + i / 4096);
	}
	long long sum = 0;
	for (int i = 0; i < (int)sizeof(buf); i += 4096) {
		sum += buf[i];
	}
	return sum;
}

// Calls Go, which calls abi_big_stack: C frames on both sides of a callback.
long long abi_nested(int seed) {
	volatile char buf[128 * 1024];
	buf[0] = 1;
	return goNested(seed) + buf[0];
}

// Calls Go, which walks its stack through this C frame.
void abi_callers(void) {
	volatile int pad = 0;
	goCallers();
	pad++;
}
