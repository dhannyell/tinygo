package main

// A goroutine recurses with large frames past the end of its stack.

var sink byte

//go:noinline
func deep(n int) byte {
	var buf [6000]byte
	buf[n%len(buf)] = byte(n)
	if n == 0 {
		return buf[0]
	}
	r := deep(n - 1)
	sink += buf[(n*7)%len(buf)] // Prevents a tail call.
	return r + buf[n%len(buf)]
}

func main() {
	done := make(chan byte)
	go func() { done <- deep(100) }()
	println("not reached:", <-done)
}
