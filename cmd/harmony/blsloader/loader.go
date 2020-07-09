package blsloader

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/harmony-one/harmony/multibls"
)

func LoadKeys(cfg Config) (multibls.PrivateKeys, error) {
	fmt.Println("start")
	cfg.applyDefault()
	if err := cfg.validate(); err != nil {
		fmt.Println("validate")
		return multibls.PrivateKeys{}, err
	}
	helper, err := getHelper(cfg)
	if err != nil {
		return nil, err
	}
	return helper.loadKeys()
}

// Loader is the structure to load bls keys.
type Config struct {
	// source for bls key loading. At least one of the BlsKeyFile and BlsDir
	// need to be provided.
	//
	// BlsKeyFile defines a single key file to load from. Based on the file
	// extension, decryption with either passphrase or aws kms will be used.
	BlsKeyFile *string
	// BlsDir defines a file directory to load keys from.
	BlsDir *string

	// Passphrase related settings. Used for passphrase encrypted key files.
	//
	// PassSrcType defines the source to get passphrase. Three source types are available
	//   PassSrcFile   - get passphrase from a .pass file
	//   PassSrcPrompt - get passphrase from prompt
	//   PassSrcAuto   - try to unlock with .pass file. If not success, ask user with prompt
	// Value is default to PassSrcAuto.
	PassSrcType PassSrcType
	// PassFile specifies the .pass file to be used when loading passphrase from file.
	// If not set, default to the .pass file in the same directory as the key file.
	PassFile *string
	// PersistPassphrase set whether to persist the passphrase to a .pass file when
	// prompt the user for password. Persisted pass file is a file with .pass extension
	// under the same directory as the key file.
	PersistPassphrase bool

	// Aws configuration related settings, including AWS credentials and region info.
	// Used for KMS encrypted passphrase files.
	//
	// AwsCfgSrcType defines the source to get aws config. Three types available:
	//   AwsCfgSrcFile   - get AWS config through a json file. See AwsConfig for content fields.
	//   AwsCfgSrcPrompt - get AWS config through prompt.
	//   AwsCfgSrcShared - Use the default AWS config settings (from env and $HOME/.aws/config)
	// Default to AwsCfgSrcShared.
	AwsCfgSrcType AwsCfgSrcType
	// AwsConfigFile set the json file to load aws config.
	AwsConfigFile *string
}

func (cfg *Config) applyDefault() {
	if cfg.PassSrcType == PassSrcNil {
		cfg.PassSrcType = PassSrcAuto
	}
	if cfg.AwsCfgSrcType == AwsCfgSrcNil {
		cfg.AwsCfgSrcType = AwsCfgSrcShared
	}
}

func (cfg *Config) validate() error {
	if stringIsSet(cfg.BlsKeyFile) {
		if !isFile(*cfg.BlsKeyFile) {
			return fmt.Errorf("key file not exist %v", *cfg.BlsKeyFile)
		}
		switch ext := filepath.Ext(*cfg.BlsKeyFile); ext {
		case basicKeyExt, kmsKeyExt:
		default:
			return fmt.Errorf("unknown key file extension %v", ext)
		}
	} else if stringIsSet(cfg.BlsDir) {
		if !isDir(*cfg.BlsDir) {
			return fmt.Errorf("dir not exist %v", *cfg.BlsDir)
		}
	} else {
		return errors.New("either BlsKeyFile or BlsDir must be set")
	}
	if err := cfg.getPassProviderConfig().validate(); err != nil {
		return err
	}
	return cfg.getKmsProviderConfig().validate()
}

func (cfg *Config) getPassProviderConfig() passProviderConfig {
	return passProviderConfig{
		passSrcType:       cfg.PassSrcType,
		passFile:          cfg.PassFile,
		persistPassphrase: cfg.PersistPassphrase,
	}
}

func (cfg *Config) getKmsProviderConfig() kmsProviderConfig {
	return kmsProviderConfig{
		awsCfgSrcType: cfg.AwsCfgSrcType,
		awsConfigFile: cfg.AwsConfigFile,
	}
}

func getHelper(cfg Config) (loadHelper, error) {
	fmt.Println("getting helper")
	switch {
	case stringIsSet(cfg.BlsKeyFile):
		switch filepath.Ext(*cfg.BlsKeyFile) {
		case basicKeyExt:
			fmt.Println("basic")
			return &basicSingleBlsLoader{
				blsKeyFile:         *cfg.BlsKeyFile,
				passProviderConfig: cfg.getPassProviderConfig(),
			}, nil
		case kmsKeyExt:
			fmt.Println("kms")
			return &kmsSingleBlsLoader{
				blsKeyFile:        *cfg.BlsKeyFile,
				kmsProviderConfig: cfg.getKmsProviderConfig(),
			}, nil
		default:
			return nil, errors.New("unknown extension")
		}
	case stringIsSet(cfg.BlsDir):
		return &blsDirLoader{
			dirPath:            *cfg.BlsDir,
			passProviderConfig: cfg.getPassProviderConfig(),
			kmsProviderConfig:  cfg.getKmsProviderConfig(),
		}, nil
	default:
		return nil, errors.New("either BlsKeyFile or BlsDir must be set")
	}
}
