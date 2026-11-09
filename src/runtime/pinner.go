package runtime

import "internal/reflectlite"

// Pinner pins Go objects, so C code can keep pointers to them. The GC does
// not move objects, so a pinned object only has to stay reachable.
type Pinner struct {
	refs []any
}

// Pin pins the object that pointer points to until Unpin is called.
func (p *Pinner) Pin(pointer any) {
	t := reflectlite.TypeOf(pointer)
	if t == nil {
		panic("runtime.Pinner: argument is nil")
	}
	if k := t.Kind(); k != reflectlite.Pointer && k != reflectlite.UnsafePointer {
		panic("runtime.Pinner: argument is not a pointer: " + t.String())
	}
	p.refs = append(p.refs, pointer)
}

// Unpin unpins all objects that the Pinner pinned.
func (p *Pinner) Unpin() {
	clear(p.refs)
	p.refs = p.refs[:0]
}
