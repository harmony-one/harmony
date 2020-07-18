package main

import (
	"io/ioutil"
	"time"

	"github.com/pelletier/go-toml"
)

var globalConfig *parsedConfig

type parsedConfig struct {
	Run       runConfig
	Network   networkConfig
	P2P       p2pConfig
	RPC       rpcConfig
	Consensus consensusConfig
	BLSKey    blsConfig
	TxPool    txPoolConfig
	Storage   storageConfig
	Pprof     pprofConfig
	Log       logConfig
	Devnet    *devnetConfig `toml:",omitempty"`
}

type runConfig struct {
	NodeType  string
	IsStaking bool
}

type networkConfig struct {
	NetworkType string
	BootNodes   []string

	LegacySyncing bool // if true, use LegacySyncingPeerProvider
	DNSZone       string
	DNSPort       int
}

type p2pConfig struct {
	IP      string
	Port    int
	KeyFile string
}

type consensusConfig struct {
	DelayCommit time.Duration
	BlockTime   time.Duration
}

type blsConfig struct {
	KeyDir     string
	KeyFiles   []string
	maxBLSKeys int

	PassSrcType      string
	PassFile         string
	SavePassphrase   bool
	KmsConfigSrcType string
	KmsConfigFile    string
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

func loadConfig(file string) (persistConfig, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return persistConfig{}, err
	}

	var config persistConfig
	if err := toml.Unmarshal(b, &config); err != nil {
		return persistConfig{}, err
	}
	return config, nil
}

func writeConfigToFile(config persistConfig, file string) error {
	b, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0644)
}
