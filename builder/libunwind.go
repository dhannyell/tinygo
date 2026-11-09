package builder

import (
	"os"
	"path/filepath"

	"github.com/tinygo-org/tinygo/goenv"
)

// libUnwind is the LLVM unwinder. It provides the _Unwind_* functions and
// _GCC_specific_handler that C++ and Rust objects for windows-gnu link to.
var libUnwind = Library{
	name: "libunwind",
	cflags: func(target, headerPath string) []string {
		return []string{
			"-I" + filepath.Join(libUnwindDir(), "include"),
			"-nostdinc++",
			"-fno-exceptions",
			"-fno-rtti",
			"-funwind-tables",
			"-D_LIBUNWIND_IS_NATIVE_ONLY",
			"-D_LIBUNWIND_DISABLE_VISIBILITY_ANNOTATIONS",
		}
	},
	needsLibc: true,
	sourceDir: func() string { return filepath.Join(libUnwindDir(), "src") },
	librarySources: func(target string, _ bool) ([]string, error) {
		return []string{
			"libunwind.cpp",
			"Unwind-seh.cpp",
			"UnwindLevel1.c",
			"UnwindLevel1-gcc-ext.c",
			"UnwindRegistersRestore.S",
			"UnwindRegistersSave.S",
		}, nil
	},
}

// libUnwindDir returns lib/libunwind in a release, or the llvm-project
// checkout in a development build.
func libUnwindDir() string {
	dir := filepath.Join(goenv.Get("TINYGOROOT"), "lib/libunwind")
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	return filepath.Join(goenv.Get("TINYGOROOT"), "llvm-project/libunwind")
}
