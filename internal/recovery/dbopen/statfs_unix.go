//go:build unix

package dbopen

import (
	"fmt"
	"syscall"
)

func statDevIno(path string) (uint64, uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, 0, fmt.Errorf("dbopen: stat %s: %w", path, err)
	}
	return uint64(st.Dev), uint64(st.Ino), nil
}

// FreeBytes returns the free space of the filesystem holding path (the
// scratch-reserve pre-run check, plan §4.6).
func FreeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("dbopen: statfs %s: %w", path, err)
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
