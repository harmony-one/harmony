package blsloader

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

// passProvider is the interface to provide the passphrase of a bls keys.
// Implemented by
//    emptyPassProvider - provide empty passphrase
// 	  promptPassProvider - provide passphrase through user-interactive prompt
//    filePassProvider - provide passphrase from a .pass file
//    dirPassProvider - provide passphrase from .pass files in a directory
//    multiPassProvider - multiple passProviders that will provide the passphrase.
type passProvider interface {
	getPassphrase(blsPubStr string) (string, error)
}

// promptPassProvider provides the bls password through console prompt.
type promptPassProvider struct{}

const pwdPromptStr = "Enter passphrase for the BLS key file %s:"

func (provider *promptPassProvider) getPassphrase(blsPubStr string) (string, error) {
	fmt.Printf(pwdPromptStr, blsPubStr)
	hexBytes, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		return "", err
	}
	return string(hexBytes), nil
}

// filePassProvider provide the bls password from the single bls pass file
type filePassProvider struct {
	file string
}

func (provider *filePassProvider) getPassphrase(blsPubStr string) (string, error) {
	return readPassFromFile(provider.file)
}

func readPassFromFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("cannot open passphrase file: [%s]", file)
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// dirPassProvider provide the all bls password available in the directory.
type dirPassProvider struct {
	dirPath string
}

func (provider *dirPassProvider) getPassphrase(blsPubStr string) (string, error) {
	file := filepath.Join(provider.dirPath, blsPubStr+passExt)
	return readPassFromFile(file)
}

// multiPassProvider is a slice of passProviders to provide the passphrase
// of the given bls key. If a passProvider fails, will continue to the next provider.
type multiPassProvider []passProvider

func (provider multiPassProvider) getPassphrase(blsPubStr string) (string, error) {
	for _, pp := range provider {
		pass, err := pp.getPassphrase(blsPubStr)
		if err != nil {
			continue
		}
		return pass, err
	}
	return "", fmt.Errorf("cannot get passphrase for %s", blsPubStr)
}
