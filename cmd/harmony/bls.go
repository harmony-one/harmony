package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/harmony-one/harmony/internal/cli"

	"github.com/harmony-one/harmony/internal/blsgen"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/multibls"
)

var (
	blsKeyFile        = flag.String("blskey_file", "", "The encrypted file of bls serialized private key by passphrase.")
	blsFolder         = flag.String("blsfolder", ".hmy/blskeys", "The folder that stores the bls keys and corresponding passphrases; e.g. <blskey>.key and <blskey>.pass; all bls keys mapped to same shard")
	maxBLSKeysPerNode = flag.Int("max_bls_keys_per_node", 10, "Maximum number of bls keys allowed per node (default 4)")

	blsPass         = flag.String("blspass", "default", "The source for bls passphrases. (default, no-prompt, prompt, file:$PASS_FILE, none)")
	persistPass     = flag.Bool("save-passphrase", false, "Whether the prompt passphrase is saved after prompt.")
	awsConfigSource = flag.String("aws-config-source", "default", "The source for aws config. (default, prompt, file:$CONFIG_FILE, none)")
)

var (
	multiBLSPriKey multibls.PrivateKeys
	onceLoadBLSKey sync.Once
)

var blsFlags = []cli.Flag{
	blsDirFlag,
	blsKeyFilesFlag,
	maxBLSKeyFilesFlag,
	passEnabledFlag,
	passSrcTypeFlag,
	passSrcFileFlag,
	passSaveFlag,
	kmsEnabledFlag,
	kmsConfigSrcTypeFlag,
	kmsConfigFileFlag,
	legacyBLSKeyFileFlag,
	legacyBLSFolderFlag,
	legacyBLSKeysPerNodeFlag,
	legacyBLSPassFlag,
	legacyBLSPersistPassFlag,
	legacyKMSConfigSourceFlag,
}

var (
	blsDirFlag = cli.StringFlag{
		Name:     "bls.dir",
		Usage:    "directory for BLS keys",
		DefValue: defaultConfig.BLSKeys.KeyDir,
	}
	blsKeyFilesFlag = cli.StringSliceFlag{
		Name:     "bls.keys",
		Usage:    "a list of BLS key files (separated by ,)",
		DefValue: defaultConfig.BLSKeys.KeyFiles,
	}
	// TODO: shall we move this to a hard coded parameter?
	maxBLSKeyFilesFlag = cli.IntFlag{
		Name:     "bls.maxkeys",
		Usage:    "maximum number of BLS keys for a node",
		DefValue: defaultConfig.BLSKeys.MaxKeys,
	}
	passEnabledFlag = cli.BoolFlag{
		Name:     "bls.pass",
		Usage:    "whether BLS key decryption with passphrase is enabled",
		DefValue: defaultConfig.BLSKeys.PassEnabled,
	}
	passSrcTypeFlag = cli.StringFlag{
		Name:     "bls.pass.src",
		Usage:    "source for BLS passphrase (auto, file, prompt)",
		DefValue: defaultConfig.BLSKeys.PassSrcType,
	}
	passSrcFileFlag = cli.StringFlag{
		Name:     "bls.pass.file",
		Usage:    "the pass file used for BLS decryption. If specified, this pass file will be used for all BLS keys",
		DefValue: defaultConfig.BLSKeys.PassFile,
	}
	passSaveFlag = cli.BoolFlag{
		Name:     "bls.pass.save",
		Usage:    "after input the BLS passphrase from console, whether to persist the input passphrases in .pass file",
		DefValue: defaultConfig.BLSKeys.SavePassphrase,
	}
	kmsEnabledFlag = cli.BoolFlag{
		Name:     "bls.kms",
		Usage:    "whether BLS key decryption with AWS KMS service is enabled",
		DefValue: defaultConfig.BLSKeys.KMSEnabled,
	}
	kmsConfigSrcTypeFlag = cli.StringFlag{
		Name:     "bls.kms.src",
		Usage:    "the AWS config source (region and credentials) for KMS service (shared, prompt, file)",
		DefValue: defaultConfig.BLSKeys.KMSConfigSrcType,
	}
	kmsConfigFileFlag = cli.StringFlag{
		Name:     "bls.kms.config",
		Usage:    "json config file for KMS service (region and credentials)",
		DefValue: defaultConfig.BLSKeys.KMSConfigFile,
	}
	legacyBLSKeyFileFlag = cli.StringSliceFlag{
		Name:       "blskey_file",
		Usage:      "The encrypted file of bls serialized private key by passphrase.",
		DefValue:   defaultConfig.BLSKeys.KeyFiles,
		Deprecated: "use --bls.keys",
	}
	legacyBLSFolderFlag = cli.StringFlag{
		Name:       "blsfolder",
		Usage:      "The folder that stores the bls keys and corresponding passphrases; e.g. <blskey>.key and <blskey>.pass; all bls keys mapped to same shard",
		DefValue:   defaultConfig.BLSKeys.KeyDir,
		Deprecated: "use --bls.dir",
	}
	legacyBLSKeysPerNodeFlag = cli.IntFlag{
		Name:       "max_bls_keys_per_node",
		Usage:      "Maximum number of bls keys allowed per node (default 4)",
		DefValue:   defaultConfig.BLSKeys.MaxKeys,
		Deprecated: "use --bls.maxkeys",
	}
	legacyBLSPassFlag = cli.StringFlag{
		Name:       "blspass",
		Usage:      "The source for bls passphrases. (default, stdin, no-prompt, prompt, file:$PASS_FILE, none)",
		DefValue:   "default",
		Deprecated: "use --bls.pass, --bls.pass.src, --bls.pass.file",
	}
	legacyBLSPersistPassFlag = cli.BoolFlag{
		Name:       "save-passphrase",
		Usage:      "Whether the prompt passphrase is saved after prompt.",
		DefValue:   defaultConfig.BLSKeys.SavePassphrase,
		Deprecated: "use --bls.pass.save",
	}
	legacyKMSConfigSourceFlag = cli.StringFlag{
		Name:       "aws-config-source",
		Usage:      "The source for aws config. (default, prompt, file:$CONFIG_FILE, none)",
		DefValue:   "default",
		Deprecated: "use --bls.kms, --bls.kms.src, --bls.kms.config",
	}
)

// setupConsensusKeys load bls keys and set the keys to nodeConfig. Return the loaded public keys.
func setupConsensusKeys(config *nodeconfig.ConfigType) multibls.PublicKeys {
	onceLoadBLSKey.Do(func() {
		var err error
		multiBLSPriKey, err = loadBLSKeys()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR when loading bls key: %v\n", err)
			os.Exit(100)
		}
		fmt.Printf("Successfully loaded %v BLS keys\n", len(multiBLSPriKey))
	})
	config.ConsensusPriKey = multiBLSPriKey
	return multiBLSPriKey.GetPublicKeys()
}

func loadBLSKeys() (multibls.PrivateKeys, error) {
	config, err := parseBLSLoadingConfig()
	if err != nil {
		return nil, err
	}
	keys, err := blsgen.LoadKeys(config)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("0 bls keys loaded")
	}
	if len(keys) > *maxBLSKeysPerNode {
		return nil, fmt.Errorf("bls keys exceed maximum count %v", *maxBLSKeysPerNode)
	}
	return keys, err
}

func parseBLSLoadingConfig() (blsgen.Config, error) {
	var (
		config blsgen.Config
		err    error
	)
	if len(*blsKeyFile) != 0 {
		config.MultiBlsKeys = strings.Split(*blsKeyFile, ",")
	}
	config.BlsDir = blsFolder

	config, err = parseBLSPass(config, *blsPass)
	if err != nil {
		return blsgen.Config{}, err
	}
	config, err = parseAwsConfigSrc(config, *awsConfigSource)
	if err != nil {
		return blsgen.Config{}, err
	}
	return config, nil
}

func parseBLSPass(config blsgen.Config, src string) (blsgen.Config, error) {
	methodArgs := strings.SplitN(src, ":", 2)
	method := methodArgs[0]

	switch method {
	case "default", "stdin":
		config.PassSrcType = blsgen.PassSrcAuto
	case "file":
		config.PassSrcType = blsgen.PassSrcFile
		if len(methodArgs) < 2 {
			return blsgen.Config{}, errors.New("must specify passphrase file")
		}
		config.PassFile = &methodArgs[1]
	case "no-prompt":
		config.PassSrcType = blsgen.PassSrcFile
	case "prompt":
		config.PassSrcType = blsgen.PassSrcPrompt
		config.PersistPassphrase = *persistPass
	case "none":
		config.PassSrcType = blsgen.PassSrcNil
	}
	config.PersistPassphrase = *persistPass
	return config, nil
}

func parseAwsConfigSrc(config blsgen.Config, src string) (blsgen.Config, error) {
	methodArgs := strings.SplitN(src, ":", 2)
	method := methodArgs[0]
	switch method {
	case "default":
		config.AwsCfgSrcType = blsgen.AwsCfgSrcShared
	case "file":
		config.AwsCfgSrcType = blsgen.AwsCfgSrcFile
		if len(methodArgs) < 2 {
			return blsgen.Config{}, errors.New("must specify aws config file")
		}
		config.AwsConfigFile = &methodArgs[1]
	case "prompt":
		config.AwsCfgSrcType = blsgen.AwsCfgSrcPrompt
	case "none":
		config.AwsCfgSrcType = blsgen.AwsCfgSrcNil
	}
	return config, nil
}
