package blsloader

import (
	"errors"
	"os"
	"path/filepath"

	ffibls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/multibls"
	errors2 "github.com/pkg/errors"
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
	providers := loader.getPassProvidersSingleBasic()
	secretKey, err := loadBasicKey(*loader.BlsPassFile, providers)
	if err != nil {
		return multibls.PrivateKey{}, errors2.Wrap(err, "")
	}
	return secretKeyToMultiPrivateKey(secretKey), nil
}

func (loader *Loader) getPassProvidersSingleBasic() []passProvider {
	switch {
	case stringIsSet(loader.BlsPassFile):
		provider := newFilePassProvider(*loader.BlsPassFile)
		return []passProvider{provider}
	default:
		passFile, passFileExist := loader.checkPassFileExistSingleBasic()
		if passFileExist {
			return []passProvider{
				newFilePassProvider(passFile),
				newPromptPassProvider(),
			}
		}
		return []passProvider{newPromptPassProvider()}
	}
}

func (loader *Loader) checkPassFileExistSingleBasic() (string, bool) {
	keyFile := *loader.BlsPassFile
	dir, base := filepath.Dir(keyFile), filepath.Base(keyFile)
	passBase := keyFileToPassFile(base)
	passFile := filepath.Join(dir, passBase)

	if info, err := os.Stat(passFile); err != nil {
		if isPassFile(info) {
			return passFile, true
		}
	}
	return "", false
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
	pps := loader.getPassProvidersDirKeys()
	kcp := loader.getKmsClientProviderDirKeys()

	helper := &blsDirLoadHelper{
		dirPath: *loader.BlsDir,
		pps:     pps,
		kcp:     kcp,
	}
	return helper.getKeyFilesFromDir()
}

func (loader *Loader) getPassProvidersDirKeys() []passProvider {
	switch {
	case stringIsSet(loader.BlsPassFile):
		return []passProvider{newFilePassProvider(*loader.BlsPassFile)}
	case loader.UsePromptPassword:
		return []passProvider{newPromptPassProvider()}
	default:
		if loader.PersistPassphrase {
			return []passProvider{
				newDirPassProvider(*loader.BlsDir),
				newPromptPassProvider().
					setPersist(*loader.BlsDir, defWritePassFileMode),
			}
		}
		return []passProvider{
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
