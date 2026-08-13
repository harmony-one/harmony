//go:build !unix

package dbopen

import "errors"

func statDevIno(path string) (uint64, uint64, error) {
	return 0, 0, nil
}

// FreeBytes is unsupported off unix; callers treat an error as "unknown".
func FreeBytes(path string) (uint64, error) {
	return 0, errors.New("dbopen: statfs unsupported on this platform")
}
