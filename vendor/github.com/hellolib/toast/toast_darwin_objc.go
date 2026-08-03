//go:build darwin

package toast

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#import <Foundation/Foundation.h>
#import <Cocoa/Cocoa.h>

void Push(const char *strJson);
*/
import "C"
import (
	"encoding/json"
	"unsafe"
)

func WithObjectiveC(b ...bool) NotificationOption {
	if len(b) == 0 {
		b = []bool{true}
	}
	return func(n *notification) {
		n._useObjC = b[0]
	}
}

// func WithFakeBundleID(bundleID string) NotificationOption {
// 	return func(n *notification) {
// 		n._useObjC = true
// 		n.BundleID = bundleID
// 	}
// }

func (n *notification) pushWithObjC() error {
	bsData, err := json.Marshal(n)
	if err != nil {
		return err
	}
	cs := C.CString(string(bsData))
	defer C.free(unsafe.Pointer(cs))

	C.Push(cs)
	return nil
}
