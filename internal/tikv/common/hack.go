package common

import (
	"unsafe"
)

// String converts byte slice to string.
func String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringBytes converts string to byte slice.
func StringBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}
