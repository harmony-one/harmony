//go:build unix

package rodb

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// ProbeLiveWriter best-effort detects a running node: it opens LOCK
// read-only ONLY if it already exists (never O_CREATE) and tries a
// non-blocking shared flock, which a running node's LOCK_EX blocks. The
// shared lock is released immediately and never held across reads. The
// result is informational only; probe failure is ignored.
func ProbeLiveWriter(dir string) (running bool, known bool) {
	path := filepath.Join(dir, "LOCK")
	if _, err := os.Lstat(path); err != nil {
		// No LOCK file: nothing to probe (and nothing must be created).
		return false, false
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false, false
	}
	defer f.Close()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		if err == unix.EWOULDBLOCK || err == unix.EAGAIN {
			return true, true
		}
		return false, false
	}
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return false, true
}
