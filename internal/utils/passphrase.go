package utils

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
)

// AskForPassphrase return passphrase using password input
func AskForPassphrase(prompt string) string {
	fmt.Println(prompt)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		panic("read password error")
	}
	password := string(bytePassword)
	fmt.Println()

	return password
}

// readAllAsString reads the entire file contents as a string.
func readAllAsString(r io.Reader) (data string, err error) {
	bytes, err := io.ReadAll(r)
	return string(bytes), err
}

// GetPassphraseFromSource reads a passphrase such as a key-encrypting one
// non-interactively from the given source.
//
// The source can be "pass:password", "env:var", "file:pathname", "fd:number",
// or "stdin".  See “PASS PHRASE ARGUMENTS” section of openssl(1) for details.
//
// When "stdin" or "fd:" is used,
// the standard input or the given file descriptor is exhausted.
// Therefore, this function should be called at most once per program
// invocation; the second call, if any, may return an empty string if "stdin"
// or "fd" is used.
func GetPassphraseFromSource(src string) (pass string, err error) {
	switch src {
	case "stdin":
		return readAllAsString(os.Stdin)
	}
	methodArg := strings.SplitN(src, ":", 2)
	if len(methodArg) < 2 {
		return "", errors.Errorf("invalid passphrase reading method %#v", src)
	}
	method := methodArg[0]
	arg := methodArg[1]
	switch method {
	case "pass":
		return arg, nil
	case "env":
		pass, ok := os.LookupEnv(arg)
		if !ok {
			return "", errors.Errorf("environment variable %#v undefined", arg)
		}
		return pass, nil
	case "file":
		f, err := os.Open(arg)
		if err != nil {
			return "", errors.Wrapf(err, "cannot open file %#v", arg)
		}
		defer func() { _ = f.Close() }()
		return readAllAsString(f)
	case "fd":
		fd, err := strconv.ParseUint(arg, 10, 0)
		if err != nil {
			return "", errors.Wrapf(err, "invalid fd literal %#v", arg)
		}
		f := os.NewFile(uintptr(fd), "(passphrase-source)")
		if f == nil {
			return "", errors.Errorf("cannot open fd %#v", fd)
		}
		defer func() { _ = f.Close() }()
		return readAllAsString(f)
	}
	return "", errors.Errorf("invalid passphrase reading method %#v", method)
}
