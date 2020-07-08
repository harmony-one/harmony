package blsloader

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// passProviderConfig is the data structure of passProviders config
type passProviderConfig struct {
	passSrcType       PassSrcType
	passFile          *string
	persistPassphrase bool
}

func (config passProviderConfig) validate() error {
	if !config.passSrcType.isValid() {
		return errors.New("unknown PassSrcType")
	}
	return nil
}

// passProvider is the interface to provide the passphrase of a bls keys.
// Implemented by
// 	  promptPassProvider - provide passphrase through user-interactive prompt
//    filePassProvider - provide passphrase from a .pass file
//    dirPassProvider - provide passphrase from .pass files in a directory
type passProvider interface {
	toStr() string
	getPassphrase(keyFile string) (string, error)
}

// promptPassProvider provides the bls password through console prompt.
type promptPassProvider struct {
	// if enablePersist is true, after user enter the passphrase, the
	// passphrase is also persisted into the persistDir
	enablePersist bool
	persistDir    string
}

const pwdPromptStr = "Enter passphrase for the BLS key file %s:"

func newPromptPassProvider() *promptPassProvider {
	return &promptPassProvider{}
}

func (provider *promptPassProvider) toStr() string {
	return "prompt"
}

func (provider *promptPassProvider) setPersist(dirPath string) *promptPassProvider {
	provider.enablePersist = true
	os.MkdirAll(dirPath, defWritePassDirMode)
	provider.persistDir = dirPath

	return provider
}

func (provider *promptPassProvider) getPassphrase(keyFile string) (string, error) {
	prompt := fmt.Sprintf(pwdPromptStr, keyFile)
	pass, err := promptGetPassword(prompt)
	if err != nil {
		return "", err
	}
	if provider.enablePersist {
		if err := provider.persistPassphrase(keyFile, pass); err != nil {
			return "", err
		}
	}
	return pass, nil
}

func (provider *promptPassProvider) persistPassphrase(keyFile string, passPhrase string) error {
	passFile := filepath.Join(provider.persistDir, filepath.Base(keyFile))
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

// filePassProvider provide the bls password from the single bls pass file
type filePassProvider struct {
	fileName string

	pass string
}

func newFilePassProvider(fileName string) *filePassProvider {
	return &filePassProvider{fileName: fileName}
}

func (provider *filePassProvider) toStr() string {
	return "passphrase file " + provider.fileName
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

func (provider *dirPassProvider) toStr() string {
	return "directory " + provider.dirPath
}

func newDirPassProvider(dirPath string) *dirPassProvider {
	return &dirPassProvider{dirPath: dirPath}
}

func (provider *dirPassProvider) getPassphrase(keyFile string) (string, error) {
	baseName := filepath.Base(keyFile)
	passKeyBase := keyFileToPassFileBase(baseName)
	passFile := filepath.Join(provider.dirPath, passKeyBase)
	return readPassFromFile(passFile)
}
