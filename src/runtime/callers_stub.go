//go:build !(windows && amd64)

package runtime

func Callers(skip int, pc []uintptr) int {
	return 0
}
