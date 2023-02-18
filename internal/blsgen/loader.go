package blsgen

import (
	"errors"
	"fmt"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/multibls"
)

// LoadKeys load all BLS keys with the given config. If loading keys from files, the
// file extension will decide which decryption algorithm to use.
func LoadKeys(cfg Config) (multibls.PrivateKeys, error) {
	decrypters, err := getKeyDecrypters(cfg)
	if err != nil {
		return nil, err
	}
	helper, err := getHelper(cfg, decrypters)
	if err != nil {
		return nil, err
	}
	return helper.loadKeys()
}

// Config is the config structure for LoadKeys.
type Config struct {
	// source for bls key loading. At least one of the MultiBlsKeys and BlsDir
	// need to be provided.
	//
	// MultiBlsKeys defines a slice of key files to load from.
	MultiBlsKeys []string
	// BlsDir defines a file directory to load keys from.
	BlsDir *string

	// Passphrase related settings. Used for passphrase encrypted key files.
	//
	// PassSrcType defines the source to get passphrase. Three source types are available
	//   PassSrcNil    - do not use passphrase decryption
	//   PassSrcFile   - get passphrase from a .pass file
	//   PassSrcPrompt - get passphrase from prompt
	//   PassSrcAuto   - try to unlock with .pass file. If not success, ask user with prompt
	PassSrcType PassSrcType
	// PassFile specifies the .pass file to be used when loading passphrase from file.
	// If not set, default to the .pass file in the same directory as the key file.
	PassFile *string
	// PersistPassphrase set whether to persist the passphrase to a .pass file when
	// prompt the user for passphrase. Persisted pass file is a file with .pass extension
	// under the same directory as the key file.
	PersistPassphrase bool

	// KMS related settings, including AWS credentials and region info.
	// Used for KMS encrypted passphrase files.
	//
	// AwsCfgSrcType defines the source to get aws config. Three types available:
	//   AwsCfgSrcNil    - do not use Aws KMS decryption service.
	//   AwsCfgSrcFile   - get AWS config through a json file. See AwsConfig for content fields.
	//   AwsCfgSrcPrompt - get AWS config through prompt.
	//   AwsCfgSrcShared - Use the default AWS config settings (from env and $HOME/.aws/config)
	AwsCfgSrcType AwsCfgSrcType
	// AwsConfigFile set the json file to load aws config.
	AwsConfigFile *string
}

func (cfg *Config) getPassProviderConfig() passDecrypterConfig {
	return passDecrypterConfig{
		passSrcType:       cfg.PassSrcType,
		passFile:          cfg.PassFile,
		persistPassphrase: cfg.PersistPassphrase,
	}
}

func (cfg *Config) getKmsProviderConfig() kmsDecrypterConfig {
	return kmsDecrypterConfig{
		awsCfgSrcType: cfg.AwsCfgSrcType,
		awsConfigFile: cfg.AwsConfigFile,
	}
}

// keyDecrypter is the interface to decrypt the bls key file. Currently, two
// implementations are supported:
//
//	passDecrypter - decrypt with passphrase for file name with extension .key
//	kmsDecrypter  - decrypt with aws kms service for file name with extension .bls
type keyDecrypter interface {
	extension() string
	decryptFile(keyFile string) (*bls_core.SecretKey, error)
}

func getKeyDecrypters(cfg Config) ([]keyDecrypter, error) {
	var decrypters []keyDecrypter
	if cfg.PassSrcType != PassSrcNil {
		pd, err := newPassDecrypter(cfg.getPassProviderConfig())
		if err != nil {
			return nil, err
		}
		decrypters = append(decrypters, pd)
	}
	if cfg.AwsCfgSrcType != AwsCfgSrcNil {
		kd, err := newKmsDecrypter(cfg.getKmsProviderConfig())
		if err != nil {
			return nil, err
		}
		decrypters = append(decrypters, kd)
	}
	if len(decrypters) == 0 {
		return nil, fmt.Errorf("must provide at least one bls key decryption")
	}
	return decrypters, nil
}

func getHelper(cfg Config, decrypters []keyDecrypter) (loadHelper, error) {
	switch {
	case len(cfg.MultiBlsKeys) != 0:
		return newMultiKeyLoader(cfg.MultiBlsKeys, decrypters)
	case stringIsSet(cfg.BlsDir):
		return newBlsDirLoader(*cfg.BlsDir, decrypters)
	default:
		return nil, errors.New("either MultiBlsKeys or BlsDir must be set")
	}
}
