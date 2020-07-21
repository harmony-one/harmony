package main

import (
	"strings"
	"sync"

	"github.com/harmony-one/harmony/internal/cli"
	"github.com/harmony-one/harmony/multibls"
	"github.com/spf13/cobra"
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
}

var legacyBLSFlags = []cli.Flag{
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
		Hidden:   true,
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
		Usage:      "Maximum number of bls keys allowed per node",
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

func applyBLSFlags(cmd *cobra.Command, config *hmyConfig) {
	if cli.IsFlagChanged(cmd, blsDirFlag) {
		config.BLSKeys.KeyDir = cli.GetStringFlagValue(cmd, blsDirFlag)
	} else if cli.IsFlagChanged(cmd, legacyBLSFolderFlag) {
		config.BLSKeys.KeyDir = cli.GetStringFlagValue(cmd, legacyBLSFolderFlag)
	}

	if cli.IsFlagChanged(cmd, blsKeyFilesFlag) {
		config.BLSKeys.KeyFiles = cli.GetStringSliceFlagValue(cmd, blsKeyFilesFlag)
	} else if cli.IsFlagChanged(cmd, legacyBLSKeyFileFlag) {
		config.BLSKeys.KeyFiles = cli.GetStringSliceFlagValue(cmd, legacyBLSKeyFileFlag)
	}

	if cli.IsFlagChanged(cmd, maxBLSKeyFilesFlag) {
		config.BLSKeys.MaxKeys = cli.GetIntFlagValue(cmd, maxBLSKeyFilesFlag)
	} else if cli.IsFlagChanged(cmd, legacyBLSKeysPerNodeFlag) {
		config.BLSKeys.MaxKeys = cli.GetIntFlagValue(cmd, legacyBLSKeysPerNodeFlag)
	}

	if cli.HasFlagsChanged(cmd, blsFlags) {
		applyBLSPassFlags(cmd, config)
		applyKMSFlags(cmd, config)
	} else if cli.HasFlagsChanged(cmd, legacyBLSFlags) {
		applyLegacyBLSPassFlags(cmd, config)
		applyLegacyKMSFlags(cmd, config)
	}
}

func applyBLSPassFlags(cmd *cobra.Command, config *hmyConfig) {
	if cli.IsFlagChanged(cmd, passEnabledFlag) {
		config.BLSKeys.PassEnabled = cli.GetBoolFlagValue(cmd, passEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, passSrcTypeFlag) {
		config.BLSKeys.PassSrcType = cli.GetStringFlagValue(cmd, passSrcTypeFlag)
	}
	if cli.IsFlagChanged(cmd, passSrcFileFlag) {
		config.BLSKeys.PassFile = cli.GetStringFlagValue(cmd, passSrcFileFlag)
	}
	if cli.IsFlagChanged(cmd, passSaveFlag) {
		config.BLSKeys.SavePassphrase = cli.GetBoolFlagValue(cmd, passSaveFlag)
	}
}

func applyKMSFlags(cmd *cobra.Command, config *hmyConfig) {
	var fileSpecified bool

	if cli.IsFlagChanged(cmd, kmsEnabledFlag) {
		config.BLSKeys.KMSEnabled = cli.GetBoolFlagValue(cmd, kmsEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, kmsConfigFileFlag) {
		config.BLSKeys.KMSConfigFile = cli.GetStringFlagValue(cmd, kmsConfigFileFlag)
		fileSpecified = true
	}
	if cli.IsFlagChanged(cmd, kmsConfigSrcTypeFlag) {
		config.BLSKeys.KMSConfigSrcType = cli.GetStringFlagValue(cmd, kmsConfigSrcTypeFlag)
	} else if fileSpecified {
		config.BLSKeys.KMSConfigSrcType = blsPassTypeFile
	}
}

func applyLegacyBLSPassFlags(cmd *cobra.Command, config *hmyConfig) {
	if cli.IsFlagChanged(cmd, legacyBLSPassFlag) {
		val := cli.GetStringFlagValue(cmd, legacyBLSPassFlag)
		legacyApplyBLSPassVal(val, config)
	}
	if cli.IsFlagChanged(cmd, legacyBLSPersistPassFlag) {
		config.BLSKeys.SavePassphrase = cli.GetBoolFlagValue(cmd, legacyBLSPersistPassFlag)
	}
}

func applyLegacyKMSFlags(cmd *cobra.Command, config *hmyConfig) {
	if cli.IsFlagChanged(cmd, legacyKMSConfigSourceFlag) {
		val := cli.GetStringFlagValue(cmd, legacyKMSConfigSourceFlag)
		legacyApplyKMSSourceVal(val, config)
	}
}

func legacyApplyBLSPassVal(src string, config *hmyConfig) {
	methodArgs := strings.SplitN(src, ":", 2)
	method := methodArgs[0]

	switch method {
	case legacyBLSPassTypeDefault, legacyBLSPassTypeStdin:
		config.BLSKeys.PassSrcType = blsPassTypeAuto
	case legacyBLSPassTypeStatic:
		config.BLSKeys.PassSrcType = blsPassTypeFile
		if len(methodArgs) >= 2 {
			config.BLSKeys.PassFile = methodArgs[1]
		}
	case legacyBLSPassTypeDynamic:
		config.BLSKeys.PassSrcType = blsPassTypePrompt
	case legacyBLSPassTypePrompt:
		config.BLSKeys.PassSrcType = blsPassTypePrompt
	case legacyBLSPassTypeNone:
		config.BLSKeys.PassEnabled = false
	}
}

func legacyApplyKMSSourceVal(src string, config *hmyConfig) {
	methodArgs := strings.SplitN(src, ":", 2)
	method := methodArgs[0]

	switch method {
	case legacyBLSKmsTypeDefault:
		config.BLSKeys.KMSConfigSrcType = kmsConfigTypeShared
	case legacyBLSKmsTypePrompt:
		config.BLSKeys.KMSConfigSrcType = kmsConfigTypePrompt
	case legacyBLSKmsTypeFile:
		config.BLSKeys.KMSConfigSrcType = kmsConfigTypeFile
		if len(methodArgs) >= 2 {
			config.BLSKeys.KMSConfigFile = methodArgs[1]
		}
	case legacyBLSKmsTypeNone:
		config.BLSKeys.KMSEnabled = false
	}
}

//// TODO: refactor this
//func loadBLSKeys() (multibls.PrivateKeys, error) {
//	config, err := parseBLSLoadingConfig()
//	if err != nil {
//		return nil, err
//	}
//	keys, err := blsgen.LoadKeys(config)
//	if err != nil {
//		return nil, err
//	}
//	if len(keys) == 0 {
//		return nil, fmt.Errorf("0 bls keys loaded")
//	}
//	if len(keys) >= *maxBLSKeysPerNode {
//		return nil, fmt.Errorf("bls keys exceed maximum count %v", *maxBLSKeysPerNode)
//	}
//	return keys, err
//}
//
//func parseBLSLoadingConfig() (blsgen.Config, error) {
//	var (
//		config blsgen.Config
//		err    error
//	)
//	if len(*blsKeyFile) != 0 {
//		config.MultiBlsKeys = strings.Split(*blsKeyFile, ",")
//	}
//	config.BlsDir = blsFolder
//
//	config, err = parseBLSPass(config, *blsPass)
//	if err != nil {
//		return blsgen.Config{}, err
//	}
//	config, err = parseAwsConfigSrc(config, *awsConfigSource)
//	if err != nil {
//		return blsgen.Config{}, err
//	}
//	return config, nil
//}
