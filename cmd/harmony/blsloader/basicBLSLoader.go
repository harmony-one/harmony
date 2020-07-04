package blsloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/blsgen"
	"github.com/harmony-one/harmony/multibls"
)

// basicBLSLoader is the structure to load bls private keys through a key file
// and passphrase combination.
type basicBLSLoader struct {
	// input fields
	BLSKeyFile  *string // If both BLSKeyFile and blsFolder exists, use BLSKeyFile only
	BLSPassFile *string // If both BLSPassFile and BLSDir exists, use BLSPassFile only
	BLSDir      *string

	// variables in process
	loadKeyFiles []string
	passProvider passProvider

	// results
	privateKeys multibls.PrivateKey
}

func (loader *basicBLSLoader) loadKeys() (multibls.PrivateKey, error) {
	if err := loader.validateInput(); err != nil {
		return multibls.PrivateKey{}, err
	}

	loader.loadPassProvider()
	keyFiles, err := loader.getTargetKeyFiles()
	if err != nil {
		return multibls.PrivateKey{}, err
	}

	res := multibls.PrivateKey{PrivateKey: make([]*ffi_bls.SecretKey, 0, len(keyFiles))}
	for
}

func loadSingleBasicKeyFile(keyFile string, passFile string) {
	if passFile == ""
}

func (loader *basicBLSLoader) validateInput() error {
	emptyKeyFile := !stringIsSet(loader.BLSKeyFile)
	emptyDir := !stringIsSet(loader.BLSDir)

	if emptyKeyFile && emptyDir {
		return fmt.Errorf("BLS key file or key directory has to be provided")
	}
	return nil
}

func (loader *basicBLSLoader) loadPassProvider() {
	var provider passProvider

	if stringIsSet(loader.BLSPassFile) {
		provider = &filePassProvider{*loader.BLSPassFile}
	} else if stringIsSet(loader.BLSKeyFile) {
		provider = &promptPassProvider{}
	} else {
		provider = multiPassProvider{
			&dirPassProvider{*loader.BLSKeyFile},
			&promptPassProvider{},
		}
	}

	loader.passProvider = provider
}

func (loader *basicBLSLoader) getTargetKeyFiles() ([]string, error) {
	if loader.BLSKeyFile != nil && *loader.BLSKeyFile != "" {
		return []string{*loader.BLSKeyFile}, nil
	}
	return loader.getKeyFilesFromDir(*loader.BLSDir)
}

func (loader *basicBLSLoader) getKeyFilesFromDir(dir string) ([]string, error) {
	var keyFilePaths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if isBasicKeyFile(info) {
			keyFilePaths = append(keyFilePaths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keyFilePaths, nil
}

func stringIsSet(val *string) bool {
	return val != nil && *val != ""
}

// isBasicKeyFile return whether the given file is a bls file
func isBasicKeyFile(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	if !strings.HasSuffix(info.Name(), basicKeyExt) {
		return false
	}
	return true
}

func loadBasicKey(keyStr string, keyFile string, provider passProvider) (*ffi_bls.SecretKey, error) {
	pass, err := provider.getPassphrase(keyStr)
	if err != nil {
		return nil, err
	}
	return blsgen.LoadBLSKeyWithPassPhrase(keyFile, pass)
}
