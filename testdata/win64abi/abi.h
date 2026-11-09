typedef struct { char a, b, c; } s3;
typedef struct { int a; } s4;
typedef struct { int a, b; } s8;
typedef struct { int a, b, c; } s12;
typedef struct { long long a, b; } s16;
typedef struct { long long a, b, c, d, e; } s40;

s3 abi_s3(s3 v);
s4 abi_s4(s4 v);
s8 abi_s8(s8 v);
s12 abi_s12(s12 v);
s16 abi_s16(s16 v);
s40 abi_s40(s40 v);
long long abi_mixed(int i, s16 a, s3 b, double d, s40 c);
long long abi_callback(void);
long long abi_big_stack(int seed);
long long abi_nested(int seed);
void abi_callers(void);

// Implemented in Go.
s8 goSum(s16 v, s40 w);
s40 goPick(int i, s40 v);
long long goNested(int seed);
void goCallers(void);
