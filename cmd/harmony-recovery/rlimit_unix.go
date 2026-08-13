//go:build unix

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// fdHeadroom is the file-descriptor budget beyond the LevelDB handle cache
// (manifest, journal, caches, report file, stdio).
const fdHeadroom = 256

// checkFDLimit refuses to start when RLIMIT_NOFILE cannot cover the handle
// cache plus headroom (a mid-walk fd exhaustion would surface as a confusing
// read error).
func checkFDLimit(handles int) error {
	var lim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &lim); err != nil {
		return nil // best-effort; do not block on probe failure
	}
	need := uint64(handles) + fdHeadroom
	if uint64(lim.Cur) < need {
		return fmt.Errorf("RLIMIT_NOFILE is %d but --handles %d needs at least %d; raise it (ulimit -n %d) or lower --handles",
			lim.Cur, handles, need, need)
	}
	return nil
}
