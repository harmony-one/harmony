package main

import (
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

const tomlConfigVersion = "2.5.1" // bump from 2.5.0 for AccountSlots

const (
	defNetworkType = nodeconfig.Mainnet
)

var defaultConfig = harmonyconfig.HarmonyConfig{
	Version: tomlConfigVersion,
	General: harmonyconfig.GeneralConfig{
		NodeType:         "validator",
		NoStaking:        false,
		ShardID:          -1,
		IsArchival:       false,
		IsBeaconArchival: false,
		IsOffline:        false,
		DataDir:          "./",
	},
	Network: getDefaultNetworkConfig(defNetworkType),
	P2P: harmonyconfig.P2pConfig{
		Port:            nodeconfig.DefaultP2PPort,
		IP:              nodeconfig.DefaultPublicListenIP,
		KeyFile:         "./.hmykey",
		DiscConcurrency: nodeconfig.DefaultP2PConcurrency,
		MaxConnsPerIP:   nodeconfig.DefaultMaxConnPerIP,
	},
	HTTP: harmonyconfig.HttpConfig{
		Enabled:        true,
		RosettaEnabled: false,
		IP:             "127.0.0.1",
		Port:           nodeconfig.DefaultRPCPort,
		AuthPort:       nodeconfig.DefaultAuthRPCPort,
		RosettaPort:    nodeconfig.DefaultRosettaPort,
	},
	WS: harmonyconfig.WsConfig{
		Enabled:  true,
		IP:       "127.0.0.1",
		Port:     nodeconfig.DefaultWSPort,
		AuthPort: nodeconfig.DefaultAuthWSPort,
	},
	RPCOpt: harmonyconfig.RpcOptConfig{
		DebugEnabled:      false,
		RateLimterEnabled: true,
		RequestsPerSecond: nodeconfig.DefaultRPCRateLimit,
	},
	BLSKeys: harmonyconfig.BlsConfig{
		KeyDir:   "./.hmy/blskeys",
		KeyFiles: []string{},
		MaxKeys:  10,

		PassEnabled:      true,
		PassSrcType:      blsPassTypeAuto,
		PassFile:         "",
		SavePassphrase:   false,
		KMSEnabled:       false,
		KMSConfigSrcType: kmsConfigTypeShared,
		KMSConfigFile:    "",
	},
	TxPool: harmonyconfig.TxPoolConfig{
		BlacklistFile:  "./.hmy/blacklist.txt",
		RosettaFixFile: "",
		AccountSlots:   16,
	},
	Sync: getDefaultSyncConfig(defNetworkType),
	Pprof: harmonyconfig.PprofConfig{
		Enabled:            false,
		ListenAddr:         "127.0.0.1:6060",
		Folder:             "./profiles",
		ProfileNames:       []string{},
		ProfileIntervals:   []int{600},
		ProfileDebugValues: []int{0},
	},
	Log: harmonyconfig.LogConfig{
		Folder:       "./latest",
		FileName:     "harmony.log",
		RotateSize:   100,
		RotateCount:  0,
		RotateMaxAge: 0,
		Verbosity:    3,
		VerbosePrints: harmonyconfig.LogVerbosePrints{
			Config: true,
		},
	},
	DNSSync: getDefaultDNSSyncConfig(defNetworkType),
	ShardData: harmonyconfig.ShardDataConfig{
		EnableShardData: false,
		DiskCount:       8,
		ShardCount:      4,
		CacheTime:       10,
		CacheSize:       512,
	},
}

var defaultSysConfig = harmonyconfig.SysConfig{
	NtpServer: "1.pool.ntp.org",
}

var defaultDevnetConfig = harmonyconfig.DevnetConfig{
	NumShards:   2,
	ShardSize:   10,
	HmyNodeSize: 10,
}

var defaultRevertConfig = harmonyconfig.RevertConfig{
	RevertBeacon: false,
	RevertBefore: 0,
	RevertTo:     0,
}

var defaultLogContext = harmonyconfig.LogContext{
	IP:   "127.0.0.1",
	Port: 9000,
}

var defaultConsensusConfig = harmonyconfig.ConsensusConfig{
	MinPeers:     6,
	AggregateSig: true,
}

var defaultPrometheusConfig = harmonyconfig.PrometheusConfig{
	Enabled:    true,
	IP:         "0.0.0.0",
	Port:       9900,
	EnablePush: false,
	Gateway:    "https://gateway.harmony.one",
}

var (
	defaultMainnetSyncConfig = harmonyconfig.SyncConfig{
		Enabled:        false,
		Downloader:     false,
		Concurrency:    6,
		MinPeers:       6,
		InitStreams:    8,
		DiscSoftLowCap: 8,
		DiscHardLowCap: 6,
		DiscHighCap:    128,
		DiscBatch:      8,
	}

	defaultTestNetSyncConfig = harmonyconfig.SyncConfig{
		Enabled:        true,
		Downloader:     false,
		Concurrency:    2,
		MinPeers:       2,
		InitStreams:    2,
		DiscSoftLowCap: 2,
		DiscHardLowCap: 2,
		DiscHighCap:    1024,
		DiscBatch:      3,
	}

	defaultLocalNetSyncConfig = harmonyconfig.SyncConfig{
		Enabled:        true,
		Downloader:     true,
		Concurrency:    2,
		MinPeers:       2,
		InitStreams:    2,
		DiscSoftLowCap: 2,
		DiscHardLowCap: 2,
		DiscHighCap:    1024,
		DiscBatch:      3,
	}

	defaultElseSyncConfig = harmonyconfig.SyncConfig{
		Enabled:        true,
		Downloader:     true,
		Concurrency:    4,
		MinPeers:       4,
		InitStreams:    4,
		DiscSoftLowCap: 4,
		DiscHardLowCap: 4,
		DiscHighCap:    1024,
		DiscBatch:      8,
	}
)

const (
	defaultBroadcastInvalidTx = false
)

func getDefaultHmyConfigCopy(nt nodeconfig.NetworkType) harmonyconfig.HarmonyConfig {
	config := defaultConfig

	config.Network = getDefaultNetworkConfig(nt)
	if nt == nodeconfig.Devnet {
		devnet := getDefaultDevnetConfigCopy()
		config.Devnet = &devnet
	}
	config.Sync = getDefaultSyncConfig(nt)
	config.DNSSync = getDefaultDNSSyncConfig(nt)

	return config
}

func getDefaultSysConfigCopy() harmonyconfig.SysConfig {
	config := defaultSysConfig
	return config
}

func getDefaultDevnetConfigCopy() harmonyconfig.DevnetConfig {
	config := defaultDevnetConfig
	return config
}

func getDefaultRevertConfigCopy() harmonyconfig.RevertConfig {
	config := defaultRevertConfig
	return config
}

func getDefaultLogContextCopy() harmonyconfig.LogContext {
	config := defaultLogContext
	return config
}

func getDefaultConsensusConfigCopy() harmonyconfig.ConsensusConfig {
	config := defaultConsensusConfig
	return config
}

func getDefaultPrometheusConfigCopy() harmonyconfig.PrometheusConfig {
	config := defaultPrometheusConfig
	return config
}

const (
	nodeTypeValidator = "validator"
	nodeTypeExplorer  = "explorer"
)

const (
	blsPassTypeAuto   = "auto"
	blsPassTypeFile   = "file"
	blsPassTypePrompt = "prompt"

	kmsConfigTypeShared = "shared"
	kmsConfigTypePrompt = "prompt"
	kmsConfigTypeFile   = "file"

	legacyBLSPassTypeDefault = "default"
	legacyBLSPassTypeStdin   = "stdin"
	legacyBLSPassTypeDynamic = "no-prompt"
	legacyBLSPassTypePrompt  = "prompt"
	legacyBLSPassTypeStatic  = "file"
	legacyBLSPassTypeNone    = "none"

	legacyBLSKmsTypeDefault = "default"
	legacyBLSKmsTypePrompt  = "prompt"
	legacyBLSKmsTypeFile    = "file"
	legacyBLSKmsTypeNone    = "none"
)
