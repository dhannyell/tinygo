package main

// Aggregates at the C boundary on windows/amd64. The _Cgo_ prefix marks C
// functions as cgo declares them; they run on the system stack.

type small struct {
	a, b int32
}

type large struct {
	a, b, c, d int64
}

type odd struct {
	a, b, c byte
}

// An 8-byte struct passes and returns as an i64.
//
//export cSmall
func _Cgo_cSmall(s small) small

// A 32-byte result uses sret; a 3-byte argument passes by a pointer to a copy.
//
//export cLarge
func _Cgo_cLarge(l large, o odd) large

func callC() int64 {
	s := _Cgo_cSmall(small{1, 2})
	l := _Cgo_cLarge(large{1, 2, 3, 4}, odd{5, 6, 7})
	return int64(s.a) + l.d
}

//export goSmall
func goSmall(s small) small {
	return small{s.b, s.a}
}

//export goLarge
func goLarge(l large, o odd) large {
	l.a += int64(o.c)
	return l
}
