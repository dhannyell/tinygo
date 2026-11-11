//go:build !(gc.boehm && tinygo.wasm.threads)

package runtime

// workerYield reports whether Gosched ran on a worker thread of wasm threads.
func workerYield() bool { return false }
