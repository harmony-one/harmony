package utils

import (
	"fmt"
	"os"
	"testing"

	"github.com/pkg/errors"

	testutils "github.com/harmony-one/harmony/internal/utils/testing"
)

func exerciseGetPassphraseFromSource(t *testing.T, source, expected string) {
	if actual, err := GetPassphraseFromSource(source); err != nil {
		t.Fatal(errors.Wrap(err, "cannot read passphrase"))
	} else if actual != expected {
		t.Errorf("expected passphrase %#v; got %#v", expected, actual)
	}
}

func TestGetPassphraseFromSource_Pass(t *testing.T) {
	exerciseGetPassphraseFromSource(t, "pass:hello world", "hello world")
}

func TestGetPassphraseFromSource_File(t *testing.T) {
	expected := "\nhello world\n"
	t.Run("stdin", func(t *testing.T) {
		f := testutils.NewTempFileWithContents(t, []byte(expected))
		savedStdin := os.Stdin
		defer func() { os.Stdin = savedStdin }()
		os.Stdin = f
		exerciseGetPassphraseFromSource(t, "stdin", expected)
	})
	t.Run("file", func(t *testing.T) {
		f := testutils.NewTempFileWithContents(t, []byte(expected))
		defer testutils.CloseAndRemoveTempFile(t, f)
		exerciseGetPassphraseFromSource(t, "file:"+f.Name(), expected)
	})
	t.Run("fd", func(t *testing.T) {
		f := testutils.NewTempFileWithContents(t, []byte(expected))
		defer testutils.CloseAndRemoveTempFile(t, f)
		exerciseGetPassphraseFromSource(t, fmt.Sprintf("fd:%d", f.Fd()), expected)
	})
}
