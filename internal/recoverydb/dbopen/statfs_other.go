//go:build !linux && !darwin

package dbopen

import "errors"

func freeSpace(path string) (uint64, error) {
	return 0, errors.New("recoverydb: free-space check not supported on this platform")
}
