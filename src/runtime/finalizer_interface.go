package runtime

import "unsafe"

// setFinalizerInterface is SetFinalizer for a finalizer whose parameter is an
// interface. The compiler calls it in place of SetFinalizer, because the GCs
// call a finalizer with only the object pointer.
func setFinalizerInterface(obj interface{}, finalizer interface{}) {
	if finalizer == nil {
		SetFinalizer(obj, nil)
		return
	}
	objType := (*_interface)(unsafe.Pointer(&obj)).typecode
	f := *(*func(interface{}))((*_interface)(unsafe.Pointer(&finalizer)).value)
	SetFinalizer(obj, func(ptr unsafe.Pointer) {
		var v interface{}
		*(*_interface)(unsafe.Pointer(&v)) = _interface{typecode: objType, value: ptr}
		f(v)
	})
}
