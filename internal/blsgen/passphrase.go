package blsgen

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
)

// PassSrcType is the type of passphrase provider source.
// Four options available:
//
//	PassSrcNil    - Do not use passphrase decryption
//	PassSrcFile   - Read the passphrase from files
//	PassSrcPrompt - Read the passphrase from prompt
//	PassSrcAuto   - First try to unlock with passphrase from file, then read passphrase from prompt
type PassSrcType uint8

const (
	// PassSrcNil is place holder for nil src
	PassSrcNil PassSrcType = iota
	// PassSrcFile provide the passphrase through pass files
	PassSrcFile
	// PassSrcPrompt provide the passphrase through prompt
	PassSrcPrompt
	// PassSrcAuto first try to unlock with pass from file, then look for prompt
	PassSrcAuto
)

func (srcType PassSrcType) isValid() bool {
	switch srcType {
	case PassSrcAuto, PassSrcFile, PassSrcPrompt:
		return true
	default:
		return false
	}
}

// passDecrypterConfig is the data structure of passProviders config
type passDecrypterConfig struct {
	passSrcType       PassSrcType
	passFile          *string
	persistPassphrase bool
}

// passDecrypter decrypt the .key bls files with passphrase from a series
// of passProvider as passphrase source
type passDecrypter struct {
	config passDecrypterConfig

	pps []passProvider
}

func newPassDecrypter(cfg passDecrypterConfig) (*passDecrypter, error) {
	pd := &passDecrypter{config: cfg}
	if err := pd.validateConfig(); err != nil {
		return nil, err
	}
	pd.makePassProviders()
	return pd, nil
}

func (pd *passDecrypter) extension() string {
	return basicKeyExt
}

func (pd *passDecrypter) decryptFile(keyFile string) (*bls_core.SecretKey, error) {
	for _, pp := range pd.pps {
		secretKey, err := loadBasicKeyWithProvider(keyFile, pp)
		if err != nil {
			console.println(err)
			continue
		}
		return secretKey, nil
	}
	return nil, fmt.Errorf("failed to load bls key %v", keyFile)
}

func (pd *passDecrypter) validateConfig() error {
	config := pd.config
	if !config.passSrcType.isValid() {
		return errors.New("unknown PassSrcType")
	}
	if stringIsSet(config.passFile) {
		if err := checkIsFile(*config.passFile); err != nil {
			return err
		}
	}
	return nil
}

func (pd *passDecrypter) makePassProviders() {
	switch pd.config.passSrcType {
	case PassSrcFile:
		pd.pps = []passProvider{pd.getFilePassProvider()}
	case PassSrcPrompt:
		pd.pps = []passProvider{pd.getPromptPassProvider()}
	case PassSrcAuto:
		pd.pps = []passProvider{
			pd.getFilePassProvider(),
			pd.getPromptPassProvider(),
		}
	}
}

func (pd *passDecrypter) getPromptPassProvider() passProvider {
	return newPromptPassProvider(pd.config.persistPassphrase)
}

func (pd *passDecrypter) getFilePassProvider() passProvider {
	switch {
	case stringIsSet(pd.config.passFile):
		return newStaticPassProvider(*pd.config.passFile)
	default:
		return newDynamicPassProvider()
	}
}

// passProvider is the interface to provide the passphrase of a bls keys.
// Implemented by
//
//		  promptPassProvider  - provide passphrase through user-interactive prompt
//	   staticPassProvider  - provide passphrase from a static .pass file
//	   dynamicPassProvider - provide the passphrase based on the given key file keyFile
//	   dirPassProvider     - provide passphrase from .pass files in a directory
type passProvider interface {
	getPassphrase(keyFile string) (string, error)
}

// promptPassProvider provides the bls passphrase through console prompt.
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
	pass = strings.TrimSpace(pass)
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

	return os.WriteFile(passFile, []byte(passPhrase), defWritePassFileMode)
}

// staticPassProvider provide the bls passphrase from a static .pass file
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
// key file keyFile. For example, looking for key file xxx.key will provide the
// passphrase from xxx.pass
type dynamicPassProvider struct{}

func newDynamicPassProvider() passProvider {
	return &dynamicPassProvider{}
}

func (provider *dynamicPassProvider) getPassphrase(keyFile string) (string, error) {
	passFile := keyFileToPassFileFull(keyFile)
	return readPassFromFile(passFile)
}

func readPassFromFile(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
