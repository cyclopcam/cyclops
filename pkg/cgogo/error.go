package cgogo

// #include "stub.h"
import "C"

import (
	"errors"
	"unsafe"
)

// Consume a C heap allocated char* and return it as a Go error.
// Before returning, free the C char*.
// If the input is NULL, then return nil.
func Error(cerr *C.char) error {
	if cerr != nil {
		err := errors.New(C.GoString(cerr))
		C.free(unsafe.Pointer(cerr))
		return err
	}
	return nil
}

//func Fumble() {
//	err := Error(C.Foo())
//	fmt.Printf("Error: %v\n", err)
//}
