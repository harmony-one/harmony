package main

import (
	"sync"

	"github.com/harmony-one/harmony/multibls"
)

var (
	multiBLSPriKey multibls.PrivateKeys
	onceLoadBLSKey sync.Once
)

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
