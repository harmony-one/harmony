//go:build !unix

package dbopen

import (
	"errors"
	"os"
)

var errWouldBlock = errors.New("dbopen: lock held")

// The strict opener's writer-exclusion guard requires flock semantics;
// the recovery run targets Linux (and tests run on macOS), both unix.
func openNoFollowReadOnly(path string) (*os.File, error) {
	return nil, errors.New("dbopen: strict read-only open is only supported on unix platforms")
}

func flockShared(f *os.File) error {
	return errors.New("dbopen: flock unsupported on this platform")
}

func funlock(f *os.File) error { return nil }
