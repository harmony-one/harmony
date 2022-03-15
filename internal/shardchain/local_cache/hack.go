package local_cache

import (
	"reflect"
	"unsafe"
)

func String(b []byte) (s string) {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}

func StringBytes(s string) []byte {
	var b []byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Data = (*reflect.StringHeader)(unsafe.Pointer(&s)).Data
	hdr.Cap = len(s)
	hdr.Len = len(s)
	return b
}
