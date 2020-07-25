package blsloader

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/harmony-one/harmony/crypto/bls"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
)

// PassSrcType is the type of passphrase provider source.
// Three options available:
//  PassSrcFile   - Read the passphrase from files
//  PassSrcPrompt - Read the passphrase from prompt
//  PassSrcAuto   - First try to unlock with passphrase from file, then read passphrase from prompt
type PassSrcType uint8

const (
	PassSrcNil    PassSrcType = iota // place holder for nil src
	PassSrcFile                      // provide the passphrase through pass files
	PassSrcPrompt                    // provide the passphrase through prompt
	PassSrcAuto                      // first try to unlock with pass from file, then look for prompt
)

func (srcType PassSrcType) isValid() bool {
	switch srcType {
	case PassSrcAuto, PassSrcFile, PassSrcPrompt:
		return true
	default:
		return false
	}
}

type passDecrypter struct {
	pps []passProvider
}

func newPassDecrypter(cfg passDecrypterConfig) *passDecrypter {
	pps := cfg.makePassProviders()
	return &passDecrypter{pps}
}

func (pd *passDecrypter) decrypt(keyFile string) (*bls_core.SecretKey, error) {
	for _, pp := range pd.pps {

	}
}

func (pd *passDecrypter) checkDecryptResult(keyFile string, got *bls_core.SecretKey) error {
	expPubKey, err := getPubKeyFromFilePath(keyFile, passExt)
	if err != nil {
		if err == errUnableGetPubkey {
			// file name not bls pub key + .pass
			return nil
		}
		return err
	}
	gotPubKey := *bls.FromLibBLSPublicKeyUnsafe(got.GetPublicKey())
	if expPubKey != gotPubKey {
		return errors.New("public key unexpected")
	}
	return nil
}

// passDecrypterConfig is the data structure of passProviders config
type passDecrypterConfig struct {
	passSrcType       PassSrcType
	passFile          *string
	passDir           *string
	persistPassphrase bool
}

func (config passDecrypterConfig) validate() error {
	if !config.passSrcType.isValid() {
		return errors.New("unknown PassSrcType")
	}
	return nil
}

func (config passDecrypterConfig) makePassProviders() []passProvider {
	switch config.passSrcType {
	case PassSrcFile:
		return []passProvider{config.getFilePassProvider()}
	case PassSrcPrompt:
		return []passProvider{config.getPromptPassProvider()}
	case PassSrcAuto:
		return []passProvider{
			config.getFilePassProvider(),
			config.getPromptPassProvider(),
		}
	}
}

func (config passDecrypterConfig) getFilePassProvider() passProvider {
	switch {
	case stringIsSet(config.passFile):
		return newStaticPassProvider(*config.passFile)
	case stringIsSet(config.passDir):
		return newDirPassProvider(*config.passDir)
	default:
		return newDynamicPassProvider()
	}
}

func (config passDecrypterConfig) getPromptPassProvider() passProvider {
	return newPromptPassProvider(config.persistPassphrase)
}

// passProvider is the interface to provide the passphrase of a bls keys.
// Implemented by
// 	  promptPassProvider  - provide passphrase through user-interactive prompt
//    staticPassProvider  - provide passphrase from a static .pass file
//    dynamicPassProvider - provide the passphrase based on the given key file path
//    dirPassProvider     - provide passphrase from .pass files in a directory
type passProvider interface {
	getPassphrase(keyFile string) (string, error)
}

// promptPassProvider provides the bls password through console prompt.
type promptPassProvider struct {
	// if enablePersist is true, after user enter the passphrase, the
	// passphrase is also persisted into .pass file under the same directory
	// of the key file
	enablePersist bool
}

const pwdPromptStr = "Enter passphrase for the BLS key file %s:"

func newPromptPassProvider(enablePersist bool) *promptPassProvider {
	return &promptPassProvider{enablePersist: enablePersist}
}

func (provider *promptPassProvider) getPassphrase(keyFile string) (string, error) {
	prompt := fmt.Sprintf(pwdPromptStr, keyFile)
	pass, err := promptGetPassword(prompt)
	if err != nil {
		return "", fmt.Errorf("unable to read from prompt: %v", err)
	}
	// If user set to persist the pass file, persist to .pass file
	if provider.enablePersist {
		if err := provider.persistPassphrase(keyFile, pass); err != nil {
			return "", fmt.Errorf("unable to save passphrase: %v", err)
		}
	}
	return pass, nil
}

func (provider *promptPassProvider) persistPassphrase(keyFile string, passPhrase string) error {
	passFile := keyFileToPassFileFull(keyFile)
	if _, err := os.Stat(passFile); err == nil {
		// File exist. Prompt user to overwrite pass file
		overwrite, err := promptYesNo(fmt.Sprintf("pass file [%v] already exist. Overwrite? ", passFile))
		if err != nil {
			return err
		}
		if !overwrite {
			return nil
		}
	} else if !os.IsNotExist(err) {
		// Unknown error. Directly return
		return err
	}
	return ioutil.WriteFile(passFile, []byte(passPhrase), defWritePassFileMode)
}

// staticPassProvider provide the bls password from a static .pass file
type staticPassProvider struct {
	fileName string

	// cached field
	pass string
	err  error
	once sync.Once
}

func newStaticPassProvider(fileName string) *staticPassProvider {
	return &staticPassProvider{fileName: fileName}
}

func (provider *staticPassProvider) getPassphrase(keyFile string) (string, error) {
	provider.once.Do(func() {
		provider.pass, provider.err = readPassFromFile(provider.fileName)
	})
	return provider.pass, provider.err
}

// dynamicPassProvider provide the passphrase based on .pass file with the given
// key file path. For example, looking for key file xxx.key will provide the
// passphrase from xxx.pass
type dynamicPassProvider struct{}

func newDynamicPassProvider() passProvider {
	return &dynamicPassProvider{}
}

func (provider *dynamicPassProvider) getPassphrase(keyFile string) (string, error) {
	passFile := keyFileToPassFileFull(keyFile)
	if !isPassFile(passFile) {
		return "", fmt.Errorf("pass file %v not exist", passFile)
	}
	return readPassFromFile(passFile)
}

// dirPassProvider provide the all bls password available in the directory.
type dirPassProvider struct {
	dirPath string
}

func (provider *dirPassProvider) toStr() string {
	return "directory " + provider.dirPath
}

func newDirPassProvider(dirPath string) *dirPassProvider {
	return &dirPassProvider{dirPath: dirPath}
}

func (provider *dirPassProvider) getPassphrase(keyFile string) (string, error) {
	passFile := keyFileToPassFileFull(keyFile)
	if !isPassFile(passFile) {
		return "", fmt.Errorf("pass file %v not exist", passFile)
	}
	return readPassFromFile(passFile)
}

func readPassFromFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("cannot open passphrase file: %v", err)
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
