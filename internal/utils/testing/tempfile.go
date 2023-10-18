package testutils

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

// NewTempFile creates a new, empty temp file for testing.  Errors are fatal.
func NewTempFile(t *testing.T) *os.File {
	pattern := strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_")
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatal(errors.Wrap(err, "cannot create temp file"))
	}
	return f
}

// NewTempFileWithContents creates a new, empty temp file for testing,
// with the given contents.  The read/write offset is set to the beginning.
// Errors are fatal.
func NewTempFileWithContents(t *testing.T, contents []byte) *os.File {
	f := NewTempFile(t)
	if _, err := f.Write(contents); err != nil {
		t.Fatal(errors.Wrapf(err, "cannot write contents into %s", f.Name()))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(errors.Wrapf(err, "cannot rewind test file %s", f.Name()))
	}
	return f
}

// CloseAndRemoveTempFile closes/removes the temp file.  Errors are logged.
func CloseAndRemoveTempFile(t *testing.T, f *os.File) {
	fn := f.Name()
	if err := f.Close(); err != nil {
		t.Log(errors.Wrapf(err, "cannot close test file %s", fn))
	}
	if err := os.Remove(fn); err != nil {
		t.Log(errors.Wrapf(err, "cannot remove test file %s", fn))
	}
}
