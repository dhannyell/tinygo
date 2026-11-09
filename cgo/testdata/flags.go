package main

/*
// this name doesn't exist
#cgo  NOFLAGS: -foo

// unknown flag
#cgo CFLAGS: -fdoes-not-exist -DNOTDEFINED

#cgo CFLAGS: -DFOO

#cgo CFLAGS: -Iinclude
#include "foo.h"

#if defined(FOO)
#define BAR 3
#else
#define BAR 5
#endif

#if defined(NOTDEFINED)
#warning flag must not be defined
#endif

// Check Compiler flags
#cgo LDFLAGS: -lc

// This flag is not valid ldflags
#cgo LDFLAGS: -does-not-exists

// Build constraints: the test runs for linux/arm.
#cgo linux CFLAGS: -DLINUX
#cgo windows CFLAGS: -DWINDOWS
#cgo linux,!arm CFLAGS: -DNOT_ARM
#cgo darwin linux,arm CFLAGS: -DLINUX_ARM
#cgo unix,cgo CFLAGS: -DUNIX_CGO
#if defined(LINUX) && defined(LINUX_ARM) && defined(UNIX_CGO) && !defined(WINDOWS) && !defined(NOT_ARM)
#define CONSTRAINTS_OK 1
#endif

#cgo CFLAGS: -I${SRCDIR}/include/srcdir
#include "srcdir.h"

*/
import "C"

var (
	_ = C.BAR
	_ = C.FOO_H
	_ = C.CONSTRAINTS_OK
	_ = C.SRCDIR_H
)
