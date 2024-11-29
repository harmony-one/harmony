package shardingconfig

import "sync"

type LocalnetConfig struct {
	BlocksPerEpoch   uint64
	BlocksPerEpochV2 uint64
}

var lnc *LocalnetConfig
var once sync.Once

// GetLocalnetConfig returns localnet config
func GetLocalnetConfig() *LocalnetConfig {
	if lnc == nil {
		panic("localnet config is not set")
	}
	return lnc
}

// InitLocalnetConfig initialize localnet config
func InitLocalnetConfig(blocksPerEpoch, blocksPerEpochV2 uint64) {
	once.Do(func() {
		lnc = &LocalnetConfig{
			BlocksPerEpoch:   blocksPerEpoch,
			BlocksPerEpochV2: blocksPerEpochV2,
		}
		InitLocalnetInstances()
	})
}
