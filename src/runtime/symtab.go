package runtime

type Frames struct {
	callers []uintptr
}

type Frame struct {
	PC uintptr

	Func *Func

	Function string

	File string
	Line int

	Entry uintptr
}

// CallersFrames returns the frames of the PCs from Callers. A frame has only
// its PC: there is no symbol table for function names and lines.
func CallersFrames(callers []uintptr) *Frames {
	return &Frames{callers: callers}
}

func (ci *Frames) Next() (frame Frame, more bool) {
	if len(ci.callers) == 0 {
		return Frame{}, false
	}
	frame = Frame{PC: ci.callers[0]}
	ci.callers = ci.callers[1:]
	return frame, len(ci.callers) != 0
}
