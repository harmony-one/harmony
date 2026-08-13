//go:build unix

package dbopen

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

var errWouldBlock = errors.New("dbopen: lock held")

// openNoFollowReadOnly opens path with O_RDONLY|O_NOFOLLOW in a single
// atomic open(2): a missing file errors, a symlinked LOCK is refused, and
// nothing is ever created.
func openNoFollowReadOnly(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}

// flockShared takes a non-blocking shared flock: it excludes concurrent
// writers (a running node holds LOCK_EX) while allowing other readers.
func flockShared(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB)
	if err == unix.EWOULDBLOCK || err == unix.EAGAIN {
		return errWouldBlock
	}
	return err
}

func funlock(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
