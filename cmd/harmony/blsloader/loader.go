package blsloader

import (
	"errors"

	ffibls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/multibls"
)

type Loader struct {
	BlsKeyFile         *string
	AwsBlsKey          *string
	BlsDir             *string
	BlsPassFile        *string
	AwsConfigFile      *string
	UsePromptAwsConfig bool
	UsePromptPassword  bool
	PersistPassphrase  bool
}

func (loader *Loader) LoadKeys() (multibls.PrivateKey, error) {
	switch {
	case stringIsSet(loader.BlsKeyFile):
		return loader.loadSingleBasicBlsKey()
	case stringIsSet(loader.AwsBlsKey):
		return loader.loadSingleKmsBlsKey()
	case stringIsSet(loader.BlsDir):
		return loader.loadDirBlsKeys()
	default:
		return multibls.PrivateKey{}, errors.New("empty bls key data source")
	}
}

func (loader *Loader) loadSingleBasicBlsKey() (multibls.PrivateKey, error) {
	provider := loader.getPassProviderSingleBasic()
	secretKey, err := loadBasicKeyFromFile(*loader.BlsPassFile, provider)
	if err != nil {
		return multibls.PrivateKey{}, err
	}
	return secretKeyToMultiPrivateKey(secretKey), nil
}

func (loader *Loader) getPassProviderSingleBasic() passProvider {
	switch {
	case stringIsSet(loader.BlsPassFile):
		return newFilePassProvider(*loader.BlsPassFile)
	default:
		// TODO(Jacky): multi pass provider
		return newPromptPassProvider()
	}
}

func (loader *Loader) loadSingleKmsBlsKey() (multibls.PrivateKey, error) {
	provider := loader.getKmsProviderSingleKms()
	secretKey, err := loadKMSKeyFromFile(*loader.AwsBlsKey, provider)
	if err != nil {
		return multibls.PrivateKey{}, err
	}
	return secretKeyToMultiPrivateKey(secretKey), nil
}

func (loader *Loader) getKmsProviderSingleKms() kmsClientProvider {
	switch {
	case loader.UsePromptAwsConfig:
		return newPromptKMSProvider(defKmsPromptTimeout)
	case stringIsSet(loader.AwsConfigFile):
		return newFileKMSProvider(*loader.AwsConfigFile)
	default:
		return newSharedKMSProvider()
	}
}

func (loader *Loader) loadDirBlsKeys() (multibls.PrivateKey, error) {
	pp := loader.getPassProviderDirKeys()
	kcp := loader.getKmsClientProviderDirKeys()

	helper := &blsDirLoadHelper{
		dirPath: *loader.BlsDir,
		pp:      pp,
		kcp:     kcp,
	}
	return helper.getKeyFilesFromDir()
}

func (loader *Loader) getPassProviderDirKeys() passProvider {
	switch {
	case stringIsSet(loader.BlsPassFile):
		return newFilePassProvider(*loader.BlsPassFile)
	case loader.UsePromptPassword:
		return newPromptPassProvider()
	default:
		if loader.PersistPassphrase {
			return multiPassProvider{
				newDirPassProvider(*loader.BlsDir),
				newPromptPassProvider().
					setPersist(*loader.BlsDir, defWritePassFileMode),
			}
		}
		return multiPassProvider{
			newDirPassProvider(*loader.BlsDir),
			newPromptPassProvider(),
		}
	}
}

func (loader *Loader) getKmsClientProviderDirKeys() kmsClientProvider {
	switch {
	case stringIsSet(loader.AwsConfigFile):
		return newFileKMSProvider(*loader.AwsConfigFile)
	case loader.UsePromptAwsConfig:
		return newPromptKMSProvider(defKmsPromptTimeout)
	default:
		return newSharedKMSProvider()
	}
}

func secretKeyToMultiPrivateKey(secretKeys ...*ffibls.SecretKey) multibls.PrivateKey {
	return multibls.PrivateKey{PrivateKey: secretKeys}
}

func stringIsSet(val *string) bool {
	return val != nil && *val != ""
}
