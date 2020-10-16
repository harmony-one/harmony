package main

import nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

const tomlConfigVersion = "1.0.2"

const (
	defNetworkType = nodeconfig.Mainnet
)

var defaultConfig = harmonyConfig{
	Version: tomlConfigVersion,
	General: generalConfig{
		NodeType:   "validator",
		NoStaking:  false,
		ShardID:    -1,
		IsArchival: false,
		IsOffline:  false,
		DataDir:    "./",
	},
	Network: getDefaultNetworkConfig(defNetworkType),
	P2P: p2pConfig{
		Port:    nodeconfig.DefaultP2PPort,
		IP:      nodeconfig.DefaultPublicListenIP,
		KeyFile: "./.hmykey",
	},
	HTTP: httpConfig{
		Enabled:        true,
		RosettaEnabled: false,
		IP:             "127.0.0.1",
		Port:           nodeconfig.DefaultRPCPort,
		RosettaPort:    nodeconfig.DefaultRosettaPort,
	},
	WS: wsConfig{
		Enabled: true,
		IP:      "127.0.0.1",
		Port:    nodeconfig.DefaultWSPort,
	},
	RPCOpt: rpcOptConfig{
		Enabled: false,
	},
	BLSKeys: blsConfig{
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
	TxPool: txPoolConfig{
		BlacklistFile: "./.hmy/blacklist.txt",
	},
	Pprof: pprofConfig{
		Enabled:    false,
		ListenAddr: "127.0.0.1:6060",
	},
	Log: logConfig{
		Folder:     "./latest",
		FileName:   "harmony.log",
		RotateSize: 100,
		Verbosity:  3,
	},
}

var defaultSysConfig = sysConfig{
	NtpServer: "1.pool.ntp.org",
}

var defaultDevnetConfig = devnetConfig{
	NumShards:   2,
	ShardSize:   10,
	HmyNodeSize: 10,
}

var defaultRevertConfig = revertConfig{
	RevertBeacon: false,
	RevertBefore: 0,
	RevertTo:     0,
}

var defaultLogContext = logContext{
	IP:   "127.0.0.1",
	Port: 9000,
}

var defaultConsensusConfig = consensusConfig{
	MinPeers: 6,
}

const (
	defaultBroadcastInvalidTx = true
)

func getDefaultHmyConfigCopy(nt nodeconfig.NetworkType) harmonyConfig {
	config := defaultConfig

	config.Network = getDefaultNetworkConfig(nt)
	if nt == nodeconfig.Devnet {
		devnet := getDefaultDevnetConfigCopy()
		config.Devnet = &devnet
	}
	return config
}

func getDefaultSysConfigCopy() sysConfig {
	config := defaultSysConfig
	return config
}

func getDefaultDevnetConfigCopy() devnetConfig {
	config := defaultDevnetConfig
	return config
}

func getDefaultRevertConfigCopy() revertConfig {
	config := defaultRevertConfig
	return config
}

func getDefaultLogContextCopy() logContext {
	config := defaultLogContext
	return config
}

func getDefaultConsensusConfigCopy() consensusConfig {
	config := defaultConsensusConfig
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
