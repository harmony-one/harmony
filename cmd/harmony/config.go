package main

import (
	"io/ioutil"
	"time"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/pelletier/go-toml"
)

var defaultConfig = hmyConfig{
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
		PassSrcType:      "auto",
		PassFile:         "",
		SavePassphrase:   false,
		KMSEnabled:       false,
		KMSConfigSrcType: "shared",
		KMSConfigFile:    "",
	},
}

type hmyConfig struct {
	General   generalConfig
	Network   networkConfig
	P2P       p2pConfig
	RPC       rpcConfig
	Consensus consensusConfig
	BLSKeys   blsConfig
	TxPool    txPoolConfig
	Storage   storageConfig
	Pprof     pprofConfig
	Log       logConfig
	Devnet    *devnetConfig `toml:",omitempty"`
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
	NodeType  string
	IsStaking bool
}

type consensusConfig struct {
	DelayCommit time.Duration
	BlockTime   time.Duration
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

type storageConfig struct {
	IsArchival  bool
	DatabaseDir string
}

type pprofConfig struct {
	Enabled    bool
	ListenAddr string
}

type logConfig struct {
	LogFolder  string
	LogMaxSize int
}

type rpcConfig struct {
	Enabled bool
	IP      string
	Port    int
}

type devnetConfig struct {
	NumShards   uint
	ShardSize   int
	HmyNodeSize int
}

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
