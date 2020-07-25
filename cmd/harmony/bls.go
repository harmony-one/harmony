package main

import (
	"fmt"
	"os"
	"sync"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

	"github.com/harmony-one/harmony/internal/blsgen"
	"github.com/harmony-one/harmony/multibls"
)

var (
	multiBLSPriKey multibls.PrivateKeys
	onceLoadBLSKey sync.Once
)

// setupConsensusKeys load bls keys and set the keys to nodeConfig. Return the loaded public keys.
func setupConsensusKeys(hc harmonyConfig, config *nodeconfig.ConfigType) multibls.PublicKeys {
	onceLoadBLSKey.Do(func() {
		var err error
		multiBLSPriKey, err = loadBLSKeys(hc.BLSKeys)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR when loading bls key: %v\n", err)
			os.Exit(100)
		}
		fmt.Printf("Successfully loaded %v BLS keys\n", len(multiBLSPriKey))
	})
	config.ConsensusPriKey = multiBLSPriKey
	return multiBLSPriKey.GetPublicKeys()
}

func loadBLSKeys(raw blsConfig) (multibls.PrivateKeys, error) {
	config, err := parseBLSLoadingConfig(raw)
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
	if len(keys) >= raw.MaxKeys {
		return nil, fmt.Errorf("bls keys exceed maximum count %v", raw.MaxKeys)
	}
	return keys, err
}

func parseBLSLoadingConfig(raw blsConfig) (blsgen.Config, error) {
	var (
		config blsgen.Config
		err    error
	)
	if len(raw.KeyFiles) != 0 {
		config.MultiBlsKeys = raw.KeyFiles
	}
	config.BlsDir = &raw.KeyDir

	config, err = parseBLSPassConfig(config, raw)
	if err != nil {
		return blsgen.Config{}, err
	}
	config, err = parseBLSKmsConfig(config, raw)
	if err != nil {
		return blsgen.Config{}, err
	}
	return config, nil
}

func parseBLSPassConfig(cfg blsgen.Config, raw blsConfig) (blsgen.Config, error) {
	if !raw.PassEnabled {
		cfg.PassSrcType = blsgen.PassSrcNil
		return blsgen.Config{}, nil
	}
	switch raw.PassSrcType {
	case "auto":
		cfg.PassSrcType = blsgen.PassSrcAuto
	case "file":
		cfg.PassSrcType = blsgen.PassSrcFile
	case "prompt":
		cfg.PassSrcType = blsgen.PassSrcPrompt
	default:
		return blsgen.Config{}, fmt.Errorf("unknown pass source type [%v]", raw.PassSrcType)
	}
	cfg.PassFile = &raw.PassFile
	cfg.PersistPassphrase = raw.SavePassphrase

	return cfg, nil
}

func parseBLSKmsConfig(cfg blsgen.Config, raw blsConfig) (blsgen.Config, error) {
	if !raw.KMSEnabled {
		cfg.AwsCfgSrcType = blsgen.AwsCfgSrcNil
		return cfg, nil
	}
	switch raw.KMSConfigSrcType {
	case "shared":
		cfg.AwsCfgSrcType = blsgen.AwsCfgSrcShared
	case "file":
		cfg.AwsCfgSrcType = blsgen.AwsCfgSrcFile
	case "prompt":
		cfg.AwsCfgSrcType = blsgen.AwsCfgSrcPrompt
	default:
		return blsgen.Config{}, fmt.Errorf("unknown aws config source type [%v]", raw.KMSConfigSrcType)
	}
	cfg.AwsConfigFile = &raw.KMSConfigFile

	return cfg, nil
}
