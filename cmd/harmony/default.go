package main

import (
	conf "github.com/harmony-one/harmony/cmd/harmony/config"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

const tomlConfigVersion = "2.0.0"

const (
	defNetworkType = nodeconfig.Mainnet
)

var defaultConfig = conf.HarmonyConfig{
	Version: tomlConfigVersion,
	General: conf.GeneralConfig{
		NodeType:         "validator",
		NoStaking:        false,
		ShardID:          -1,
		IsArchival:       false,
		IsBeaconArchival: false,
		IsOffline:        false,
		DataDir:          "./",
	},
	Network: getDefaultNetworkConfig(defNetworkType),
	P2P: conf.P2pConfig{
		Port:    nodeconfig.DefaultP2PPort,
		IP:      nodeconfig.DefaultPublicListenIP,
		KeyFile: "./.hmykey",
	},
	HTTP: conf.HttpConfig{
		Enabled:        true,
		RosettaEnabled: false,
		IP:             "127.0.0.1",
		Port:           nodeconfig.DefaultRPCPort,
		RosettaPort:    nodeconfig.DefaultRosettaPort,
	},
	WS: conf.WsConfig{
		Enabled: true,
		IP:      "127.0.0.1",
		Port:    nodeconfig.DefaultWSPort,
	},
	RPCOpt: conf.RpcOptConfig{
		DebugEnabled:      false,
		RateLimterEnabled: true,
		RequestsPerSecond: nodeconfig.DefaultRPCRateLimit,
	},
	BLSKeys: conf.BlsConfig{
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
	TxPool: conf.TxPoolConfig{
		BlacklistFile: "./.hmy/blacklist.txt",
	},
	Sync: getDefaultSyncConfig(defNetworkType),
	Pprof: conf.PprofConfig{
		Enabled:    false,
		ListenAddr: "127.0.0.1:6060",
	},
	Log: conf.LogConfig{
		Folder:     "./latest",
		FileName:   "harmony.log",
		RotateSize: 100,
		Verbosity:  3,
	},
	DNSSync: getDefaultDNSSyncConfig(defNetworkType),
}

var defaultSysConfig = conf.SysConfig{
	NtpServer: "1.pool.ntp.org",
}

var defaultDevnetConfig = conf.DevnetConfig{
	NumShards:   2,
	ShardSize:   10,
	HmyNodeSize: 10,
}

var defaultRevertConfig = conf.RevertConfig{
	RevertBeacon: false,
	RevertBefore: 0,
	RevertTo:     0,
}

var defaultLogContext = conf.LogContext{
	IP:   "127.0.0.1",
	Port: 9000,
}

var defaultConsensusConfig = conf.ConsensusConfig{
	MinPeers:     6,
	AggregateSig: true,
}

var defaultPrometheusConfig = conf.PrometheusConfig{
	Enabled:    true,
	IP:         "0.0.0.0",
	Port:       9900,
	EnablePush: false,
	Gateway:    "https://gateway.harmony.one",
}

var (
	defaultMainnetSyncConfig = conf.SyncConfig{
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

	defaultTestNetSyncConfig = conf.SyncConfig{
		Enabled:        true,
		Downloader:     false,
		Concurrency:    4,
		MinPeers:       4,
		InitStreams:    4,
		DiscSoftLowCap: 4,
		DiscHardLowCap: 4,
		DiscHighCap:    1024,
		DiscBatch:      8,
	}

	defaultLocalNetSyncConfig = conf.SyncConfig{
		Enabled:        true,
		Downloader:     false,
		Concurrency:    4,
		MinPeers:       4,
		InitStreams:    4,
		DiscSoftLowCap: 4,
		DiscHardLowCap: 4,
		DiscHighCap:    1024,
		DiscBatch:      8,
	}

	defaultElseSyncConfig = conf.SyncConfig{
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
	defaultBroadcastInvalidTx = true
)

func getDefaultHmyConfigCopy(nt nodeconfig.NetworkType) conf.HarmonyConfig {
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

func getDefaultSysConfigCopy() conf.SysConfig {
	config := defaultSysConfig
	return config
}

func getDefaultDevnetConfigCopy() conf.DevnetConfig {
	config := defaultDevnetConfig
	return config
}

func getDefaultRevertConfigCopy() conf.RevertConfig {
	config := defaultRevertConfig
	return config
}

func getDefaultLogContextCopy() conf.LogContext {
	config := defaultLogContext
	return config
}

func getDefaultConsensusConfigCopy() conf.ConsensusConfig {
	config := defaultConsensusConfig
	return config
}

func getDefaultPrometheusConfigCopy() conf.PrometheusConfig {
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
