package rodb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/syndtr/goleveldb/leveldb/storage"
)

// roStorage implements goleveldb's storage.Storage over an existing DB
// directory using plain file reads only.
//
// Why not storage.OpenFile(dir, readOnly=true): its constructor acquires an
// OS flock on the LOCK file (LOCK_SH for read-only openers) and O_CREATEs
// LOCK if missing. A running validator holds LOCK_EX, so a second process
// cannot open the DB that way at all - and creating LOCK would be a write
// into a live DB directory. The goleveldb session only needs the Storage
// interface's Lock() method, which is an in-process lock; the OS flock lives
// exclusively in storage.OpenFile's constructor. So this implementation:
//
//   - never opens, creates, stats or flocks the LOCK file
//   - never writes the LOG file (Log is a no-op)
//   - returns ErrWriteRefused from Create/Remove/Rename/SetMeta
type roStorage struct {
	dir string

	mu     sync.Mutex
	locked bool
	closed bool
}

var _ storage.Storage = (*roStorage)(nil)

func newROStorage(dir string) *roStorage {
	return &roStorage{dir: dir}
}

type roLock struct{ s *roStorage }

func (l *roLock) Unlock() {
	l.s.mu.Lock()
	defer l.s.mu.Unlock()
	l.s.locked = false
}

// Lock takes the in-process lock the goleveldb session requires. No OS-level
// lock is involved.
func (s *roStorage) Lock() (storage.Locker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, storage.ErrClosed
	}
	if s.locked {
		return nil, storage.ErrLocked
	}
	s.locked = true
	return &roLock{s: s}, nil
}

// Log is a no-op; the file storage implementation would append to LOG.
func (s *roStorage) Log(str string) {}

// SetMeta would rewrite CURRENT; refused.
func (s *roStorage) SetMeta(fd storage.FileDesc) error { return ErrWriteRefused }

// GetMeta returns the manifest file descriptor named by CURRENT.
func (s *roStorage) GetMeta() (storage.FileDesc, error) {
	if err := s.ok(); err != nil {
		return storage.FileDesc{}, err
	}
	raw, err := os.ReadFile(filepath.Join(s.dir, "CURRENT"))
	if err != nil {
		return storage.FileDesc{}, err
	}
	content := strings.TrimRight(string(raw), "\r\n ")
	var num int64
	if _, err := fmt.Sscanf(content, "MANIFEST-%d", &num); err != nil || num < 0 {
		return storage.FileDesc{}, fmt.Errorf("rodb: malformed CURRENT content %q", content)
	}
	fd := storage.FileDesc{Type: storage.TypeManifest, Num: num}
	if _, err := os.Stat(filepath.Join(s.dir, fsGenName(fd))); err != nil {
		return storage.FileDesc{}, err
	}
	return fd, nil
}

// List returns the file descriptors in the DB directory matching the type
// mask, following the file storage naming rules (LOCK, LOG, CURRENT* do not
// parse as descriptors and are naturally skipped).
func (s *roStorage) List(ft storage.FileType) ([]storage.FileDesc, error) {
	if err := s.ok(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	seen := make(map[storage.FileDesc]bool)
	var fds []storage.FileDesc
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if fd, ok := fsParseName(ent.Name()); ok && fd.Type&ft != 0 && !seen[fd] {
			seen[fd] = true
			fds = append(fds, fd)
		}
	}
	return fds, nil
}

// Open opens the named file read-only with plain os.Open.
func (s *roStorage) Open(fd storage.FileDesc) (storage.Reader, error) {
	if err := s.ok(); err != nil {
		return nil, err
	}
	if !storage.FileDescOk(fd) {
		return nil, storage.ErrInvalidFile
	}
	f, err := os.Open(filepath.Join(s.dir, fsGenName(fd)))
	if os.IsNotExist(err) && fd.Type == storage.TypeTable {
		// Tables written by older goleveldb use the .sst suffix.
		f, err = os.Open(filepath.Join(s.dir, fsGenOldName(fd)))
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Create/Remove/Rename are write paths; refused.
func (s *roStorage) Create(fd storage.FileDesc) (storage.Writer, error) {
	return nil, ErrWriteRefused
}

func (s *roStorage) Remove(fd storage.FileDesc) error { return ErrWriteRefused }

func (s *roStorage) Rename(oldfd, newfd storage.FileDesc) error { return ErrWriteRefused }

func (s *roStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *roStorage) ok() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return storage.ErrClosed
	}
	return nil
}

// fsGenName/fsGenOldName/fsParseName mirror goleveldb's file storage naming
// rules (leveldb/storage/file_storage.go).
func fsGenName(fd storage.FileDesc) string {
	switch fd.Type {
	case storage.TypeManifest:
		return fmt.Sprintf("MANIFEST-%06d", fd.Num)
	case storage.TypeJournal:
		return fmt.Sprintf("%06d.log", fd.Num)
	case storage.TypeTable:
		return fmt.Sprintf("%06d.ldb", fd.Num)
	case storage.TypeTemp:
		return fmt.Sprintf("%06d.tmp", fd.Num)
	default:
		panic("rodb: invalid file type")
	}
}

func fsGenOldName(fd storage.FileDesc) string {
	if fd.Type == storage.TypeTable {
		return fmt.Sprintf("%06d.sst", fd.Num)
	}
	return fsGenName(fd)
}

func fsParseName(name string) (fd storage.FileDesc, ok bool) {
	var tail string
	_, err := fmt.Sscanf(name, "%d.%s", &fd.Num, &tail)
	if err == nil {
		switch tail {
		case "log":
			fd.Type = storage.TypeJournal
		case "ldb", "sst":
			fd.Type = storage.TypeTable
		case "tmp":
			fd.Type = storage.TypeTemp
		default:
			return storage.FileDesc{}, false
		}
		return fd, true
	}
	n, _ := fmt.Sscanf(name, "MANIFEST-%d%s", &fd.Num, &tail)
	if n == 1 {
		fd.Type = storage.TypeManifest
		return fd, true
	}
	return storage.FileDesc{}, false
}
