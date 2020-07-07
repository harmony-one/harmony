package blsloader

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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
	getPassphrase(keyFile string) (string, error)
}

// promptPassProvider provides the bls password through console prompt.
type promptPassProvider struct{}

const pwdPromptStr = "Enter passphrase for the BLS key file %s:"

func newPromptPassProvider() *promptPassProvider {
	return &promptPassProvider{}
}

func (provider *promptPassProvider) getPassphrase(keyFile string) (string, error) {
	fmt.Printf(pwdPromptStr, keyFile)
	hexBytes, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		return "", err
	}
	return string(hexBytes), nil
}

// filePassProvider provide the bls password from the single bls pass file
type filePassProvider struct {
	fileName string

	pass string
}

func newFilePassProvider(fileName string) *filePassProvider {
	return &filePassProvider{fileName: fileName}
}

func (provider *filePassProvider) getPassphrase(keyFile string) (string, error) {
	return readPassFromFile(provider.fileName)
}

func readPassFromFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("cannot open passphrase file")
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

func (provider *dirPassProvider) getPassphrase(keyFile string) (string, error) {
	baseName := filepath.Base(keyFile)
	pubKey := strings.TrimSuffix(baseName, basicKeyExt)
	passFile := filepath.Join(provider.dirPath, pubKey+passExt)
	return readPassFromFile(passFile)
}

// multiPassProvider is a slice of passProviders to provide the passphrase
// of the given bls key. If a passProvider fails, will continue to the next provider.
type multiPassProvider []passProvider

func (provider multiPassProvider) getPassphrase(keyStr string) (string, error) {
	for _, pp := range provider {
		pass, err := pp.getPassphrase(keyStr)
		if err != nil {
			continue
		}
		return pass, err
	}
	return "", fmt.Errorf("cannot get passphrase for %s", keyStr)
}
