package blsloader

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/multibls"
)

type Loader struct {
	BlsKeyFile *string
	AwsBlsKey  *string
	BlsDir     *string

	PassSrcType       PassSrcType
	PassFile          *string
	PersistPassphrase bool

	AwsCfgSrcType AwsCfgSrcType
	AwsConfigFile *string
}

// LoadKeys load all keys from the input fields provided
func (loader *Loader) LoadKeys() (multibls.PrivateKeys, error) {
	helper, err := loader.getHelper()
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	return helper.loadKeys()
}

// getHelper parse the Loader structure to helper for further computation
func (loader *Loader) getHelper() (loadHelper, error) {
	switch {
	case stringIsSet(loader.BlsKeyFile):
		return &basicSingleBlsLoader{
			blsKeyFile:        *loader.BlsKeyFile,
			passFile:          loader.PassFile,
			passSrcType:       loader.PassSrcType,
			persistPassphrase: loader.PersistPassphrase,
		}, nil
	case stringIsSet(loader.AwsBlsKey):
		return &kmsSingleBlsLoader{
			awsBlsKey:     *loader.AwsBlsKey,
			awsCfgSrcType: loader.AwsCfgSrcType,
			awsConfigFile: loader.AwsConfigFile,
		}, nil
	case stringIsSet(loader.BlsDir):
		return &blsDirLoader{
			dirPath:           *loader.BlsDir,
			pst:               loader.PassSrcType,
			passFile:          loader.PassFile,
			persistPassphrase: loader.PersistPassphrase,
			act:               loader.AwsCfgSrcType,
			awsConfigFile:     loader.AwsConfigFile,
		}, nil
	default:
		return nil, errors.New("empty bls key data source")
	}
}

// loadHelper defines the interface to help load bls keys
type loadHelper interface {
	loadKeys() (multibls.PrivateKeys, error)
}

// basicSingleBlsLoader loads a single bls key file with passphrase
type basicSingleBlsLoader struct {
	blsKeyFile        string
	passSrcType       PassSrcType
	passFile          *string
	persistPassphrase bool
}

func (loader *basicSingleBlsLoader) loadKeys() (multibls.PrivateKeys, error) {
	providers, err := loader.getPassProviders()
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	secretKey, err := loadBasicKey(loader.blsKeyFile, providers)
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	return secretKeyToMultiPrivateKey(secretKey), nil
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
	awsBlsKey string

	awsCfgSrcType AwsCfgSrcType
	awsConfigFile *string
}

func (loader *kmsSingleBlsLoader) loadKeys() (multibls.PrivateKeys, error) {
	provider, err := loader.getKmsClientProvider()
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	secretKey, err := loadKmsKeyFromFile(loader.awsBlsKey, provider)
	if err != nil {
		return multibls.PrivateKeys{}, err
	}
	return secretKeyToMultiPrivateKey(secretKey), nil
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
	// pass provider fields
	pst               PassSrcType
	passFile          *string
	persistPassphrase bool
	// kms provider fields
	act           AwsCfgSrcType
	awsConfigFile *string

	// providers in process
	pps []passProvider
	kcp kmsClientProvider
	// result field
	secretKeys []*bls.SecretKey
}

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
	switch loader.pst {
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
	switch loader.act {
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
	return secretKeyToMultiPrivateKey(loader.secretKeys...), nil
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

func (loader *blsDirLoader) loadKeyFromFile(path string, info os.FileInfo) (*bls.SecretKey, error) {
	var (
		key *bls.SecretKey
		err error
	)
	switch {
	case isBasicKeyFile(info):
		key, err = loadBasicKey(path, loader.pps)
	case isKMSKeyFile(info):
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
