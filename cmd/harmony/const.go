package main

import nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

const (
	defNetworkType = nodeconfig.Mainnet
)

const (
	mainnetDnsZone = "t.hmny.io"
)

var defaultConfig = harmonyConfig{
	Version: tomlConfigVersion,
	General: generalConfig{
		NodeType:   "validator",
		IsStaking:  true,
		ShardID:    -1,
		IsArchival: false,
		DataDir:    "./",
	},
	Network: getDefaultNetworkConfig(defNetworkType),
	P2P: p2pConfig{
		IP:      "127.0.0.1",
		Port:    nodeconfig.DefaultP2PPort,
		KeyFile: "./.hmykey",
	},
	RPC: rpcConfig{
		Enabled: true,
		IP:      "127.0.0.1",
		Port:    nodeconfig.DefaultRPCPort,
	},
	WS: wsConfig{
		Enabled: true,
		IP:      "127.0.0.1",
		Port:    nodeconfig.DefaultWSPort,
	},
	BLSKeys: blsConfig{
		KeyDir:   "./hmy/blskeys",
		KeyFiles: nil,
		MaxKeys:  10,

		PassEnabled:      true,
		PassSrcType:      blsPassTypeAuto,
		PassFile:         "",
		SavePassphrase:   false,
		KMSEnabled:       true,
		KMSConfigSrcType: kmsConfigTypeShared,
		KMSConfigFile:    "",
	},
	Consensus: consensusConfig{
		DelayCommit: "0ms",
		BlockTime:   "8s",
		MinPeers:    32,
	},
	TxPool: txPoolConfig{
		BlacklistFile:      "./.hmy/blacklist.txt",
		BroadcastInvalidTx: false,
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

func getDefaultHmyConfigCopy() harmonyConfig {
	config := defaultConfig
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
