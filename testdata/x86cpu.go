package main

// golang.org/x/sys/cpu reads the CPU with cpuid and xgetbv, which are Go
// assembly functions. The compiler lowers them to inline assembly.

import (
	"os"
	"strings"

	"golang.org/x/sys/cpu"
)

func main() {
	// Every x86 CPU that Go supports has SSE2.
	println("sse2:", cpu.X86.HasSSE2)

	// Linux reports the same features, after it checks OS support for AVX.
	flags := map[string]bool{}
	data, err := os.ReadFile("/proc/cpuinfo")
	for _, line := range strings.Split(string(data), "\n") {
		if name, list, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(name) == "flags" {
			for _, flag := range strings.Fields(list) {
				flags[flag] = true
			}
			break
		}
	}
	same := err != nil ||
		flags["sse4_1"] == cpu.X86.HasSSE41 &&
			flags["popcnt"] == cpu.X86.HasPOPCNT &&
			flags["avx"] == cpu.X86.HasAVX &&
			flags["avx2"] == cpu.X86.HasAVX2 &&
			flags["bmi2"] == cpu.X86.HasBMI2
	println("same as /proc/cpuinfo:", same)
}
