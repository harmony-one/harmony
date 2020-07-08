package blsloader

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/multibls"
)

// loadHelper defines the interface to help load bls keys
type loadHelper interface {
	loadKeys() (multibls.PrivateKeys, error)
}

// basicSingleBlsLoader loads a single bls key file with passphrase
type basicSingleBlsLoader struct {
	blsKeyFile string

	passProviderConfig
}

// loadKeys load bls keys from a single bls file
func (loader *basicSingleBlsLoader) loadKeys() (multibls.PrivateKeys, error) {
	providers, err := loader.getPassProviders()
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	secretKey, err := loadBasicKey(loader.blsKeyFile, providers)
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	return multibls.GetPrivateKeys(secretKey), nil
}

func (loader *basicSingleBlsLoader) getPassProviders() ([]passProvider, error) {
	switch loader.passSrcType {
	case PassSrcFile:
		return []passProvider{loader.getFilePassProvider()}, nil
	case PassSrcPrompt:
		return []passProvider{loader.getPromptPassProvider()}, nil
	case PassSrcAuto:
		return []passProvider{
			loader.getFilePassProvider(),
			loader.getPromptPassProvider(),
		}, nil
	default:
		return nil, errors.New("unknown passphrase source type")
	}
}

func (loader *basicSingleBlsLoader) getFilePassProvider() passProvider {
	if stringIsSet(loader.passFile) {
		return newFilePassProvider(*loader.passFile)
	}
	passFile := keyFileToPassFileFull(loader.blsKeyFile)
	return newFilePassProvider(passFile)
}

func (loader *basicSingleBlsLoader) getPromptPassProvider() passProvider {
	provider := newPromptPassProvider()
	if loader.persistPassphrase {
		provider.setPersist(filepath.Dir(loader.blsKeyFile))
	}
	return provider
}

type kmsSingleBlsLoader struct {
	blsKeyFile string

	kmsProviderConfig
}

// loadKeys load a single kms key file
func (loader *kmsSingleBlsLoader) loadKeys() (multibls.PrivateKeys, error) {
	provider, err := loader.getKmsClientProvider()
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	secretKey, err := loadKmsKeyFromFile(loader.blsKeyFile, provider)
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	return multibls.GetPrivateKeys(secretKey), nil
}

func (loader *kmsSingleBlsLoader) getKmsClientProvider() (kmsClientProvider, error) {
	switch loader.awsCfgSrcType {
	case AwsCfgSrcFile:
		if stringIsSet(loader.awsConfigFile) {
			return newFileKmsProvider(*loader.awsConfigFile), nil
		}
		return newSharedKmsProvider(), nil
	case AwsCfgSrcPrompt:
		return newPromptKmsProvider(defKmsPromptTimeout), nil
	case AwsCfgSrcShared:
		return newSharedKmsProvider(), nil
	default:
		return nil, errors.New("unknown aws config source type")
	}
}

// blsDirLoader is the helper structure for loading bls keys in a directory
type blsDirLoader struct {
	// input fields
	dirPath string
	passProviderConfig
	kmsProviderConfig

	// providers in process
	pps []passProvider
	kcp kmsClientProvider
	// result field
	secretKeys []*bls_core.SecretKey
}

// loadKeys load all keys from the directory
func (loader *blsDirLoader) loadKeys() (multibls.PrivateKeys, error) {
	var err error
	if loader.pps, err = loader.getPassProviders(); err != nil {
		return multibls.PrivateKeys{}, err
	}
	if loader.kcp, err = loader.getKmsClientProvider(); err != nil {
		return multibls.PrivateKeys{}, err
	}
	return loader.loadKeyFiles()
}

func (loader *blsDirLoader) getPassProviders() ([]passProvider, error) {
	switch loader.passSrcType {
	case PassSrcFile:
		return []passProvider{loader.getFilePassProvider()}, nil
	case PassSrcPrompt:
		return []passProvider{loader.getPromptPassProvider()}, nil
	case PassSrcAuto:
		return []passProvider{
			loader.getFilePassProvider(),
			loader.getPromptPassProvider(),
		}, nil
	default:
		return nil, errors.New("unknown pass source type")
	}
}

func (loader *blsDirLoader) getFilePassProvider() passProvider {
	if stringIsSet(loader.passFile) {
		return newFilePassProvider(*loader.passFile)
	}
	return newDirPassProvider(loader.dirPath)
}

func (loader *blsDirLoader) getPromptPassProvider() passProvider {
	provider := newPromptPassProvider()
	if loader.persistPassphrase {
		provider.setPersist(loader.dirPath)
	}
	return provider
}

func (loader *blsDirLoader) getKmsClientProvider() (kmsClientProvider, error) {
	switch loader.awsCfgSrcType {
	case AwsCfgSrcFile:
		if stringIsSet(loader.awsConfigFile) {
			return newFileKmsProvider(*loader.awsConfigFile), nil
		}
		return newSharedKmsProvider(), nil
	case AwsCfgSrcPrompt:
		return newPromptKmsProvider(defKmsPromptTimeout), nil
	case AwsCfgSrcShared:
		return newSharedKmsProvider(), nil
	default:
		return nil, errors.New("unknown aws config source type")
	}
}

func (loader *blsDirLoader) loadKeyFiles() (multibls.PrivateKeys, error) {
	err := filepath.Walk(loader.dirPath, loader.processFileWalk)
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	return multibls.GetPrivateKeys(loader.secretKeys...), nil
}

func (loader *blsDirLoader) processFileWalk(path string, info os.FileInfo, err error) error {
	key, err := loader.loadKeyFromFile(path, info)
	if err != nil {
		if !errIsErrors(err, loader.skippingErrors()) {
			// unexpected error, return the error and break the file walk loop
			return err
		}
		// expected error. Skipping these files
		skipStr := fmt.Sprintf("Skipping [%s]: %v\n", path, err)
		console.println(skipStr)
		return nil
	}
	loader.secretKeys = append(loader.secretKeys, key)
	return nil
}

// errors to be neglected for directory bls loading
func (loader *blsDirLoader) skippingErrors() []error {
	return []error{
		errUnknownExtension,
		errNilPassProvider,
		errNilKMSClientProvider,
	}
}

func (loader *blsDirLoader) loadKeyFromFile(path string, info os.FileInfo) (*bls_core.SecretKey, error) {
	var (
		key *bls_core.SecretKey
		err error
	)
	switch {
	case isBasicKeyFile(path):
		key, err = loadBasicKey(path, loader.pps)
	case isKMSKeyFile(path):
		key, err = loadKmsKeyFromFile(path, loader.kcp)
	default:
		err = errUnknownExtension
	}
	return key, err
}

// errIsErrors return whether the err is one of the errs
func errIsErrors(err error, errs []error) bool {
	for _, targetErr := range errs {
		if errors.Is(err, targetErr) {
			return true
		}
	}
	return false
}
