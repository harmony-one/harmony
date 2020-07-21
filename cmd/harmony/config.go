package main

import (
	"io/ioutil"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/pelletier/go-toml"
)

const configVersion = "1.0.0"

type hmyConfig struct {
	Version   string
	General   generalConfig
	Network   networkConfig
	P2P       p2pConfig
	RPC       rpcConfig
	Consensus consensusConfig
	BLSKeys   blsConfig
	TxPool    txPoolConfig
	Pprof     pprofConfig
	Log       logConfig
	Devnet    *devnetConfig `toml:",omitempty"`
	Revert    *revertConfig `toml:",omitempty"`
}

type networkConfig struct {
	NetworkType string
	BootNodes   []string

	LegacySyncing bool // if true, use LegacySyncingPeerProvider
	DNSZone       string
	DNSPort       int
}

type p2pConfig struct {
	Port    int
	KeyFile string
}

type generalConfig struct {
	NodeType   string
	IsStaking  bool
	ShardID    int
	IsArchival bool
	DataDir    string
}

type consensusConfig struct {
	DelayCommit string
	BlockTime   string
}

type blsConfig struct {
	KeyDir   string
	KeyFiles []string
	MaxKeys  int

	PassEnabled    bool
	PassSrcType    string
	PassFile       string
	SavePassphrase bool

	KMSEnabled       bool
	KMSConfigSrcType string
	KMSConfigFile    string
}

type txPoolConfig struct {
	BlacklistFile      string
	BroadcastInvalidTx bool
}

type pprofConfig struct {
	Enabled    bool
	ListenAddr string
}

type logConfig struct {
	LogFolder     string
	LogRotateSize int
}

type rpcConfig struct {
	Enabled bool
	IP      string
	Port    int
}

type devnetConfig struct {
	NumShards   int
	ShardSize   int
	HmyNodeSize int
}

// TODO: make this revert to a seperate command
type revertConfig struct {
	RevertBeacon bool
	RevertTo     int
	RevertBefore int
}

var defaultConfig = hmyConfig{
	Version: configVersion,
	General: generalConfig{
		NodeType:   "validator",
		IsStaking:  true,
		ShardID:    -1,
		IsArchival: false,
		DataDir:    "./",
	},
	Network: getDefaultNetworkConfig(nodeconfig.Mainnet),
	P2P: p2pConfig{
		Port:    nodeconfig.DefaultP2PPort,
		KeyFile: "./.hmykey",
	},
	RPC: rpcConfig{
		Enabled: false,
		IP:      "127.0.0.1",
		Port:    nodeconfig.DefaultRPCPort,
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
		LogFolder:     "./latest",
		LogRotateSize: 100,
	},
}

var defaultDevnetConfig = devnetConfig{
	NumShards:   2,
	ShardSize:   10,
	HmyNodeSize: -1,
}

var defaultRevertConfig = revertConfig{
	RevertBeacon: false,
	RevertBefore: -1,
	RevertTo:     -1,
}

func getDefaultDevnetConfigCopy() devnetConfig {
	return defaultDevnetConfig
}

func getDefaultRevertConfigCopy() revertConfig {
	return defaultRevertConfig
}

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

func loadConfig(file string) (hmyConfig, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return hmyConfig{}, err
	}

	var config hmyConfig
	if err := toml.Unmarshal(b, &config); err != nil {
		return hmyConfig{}, err
	}
	return config, nil
}

func writeConfigToFile(config hmyConfig, file string) error {
	b, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0644)
}
