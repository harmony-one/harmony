package main

import (
	"io/ioutil"
	"time"

	"github.com/pelletier/go-toml"
)

type hmyConfig struct {
	Run       runConfig
	Network   networkConfig
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
	ShardID   int
}

type networkConfig struct {
	Network    string
	IP         string
	Port       int
	MinPeers   int
	P2PKeyFile string
	PublicRPC  bool

	BootNodes []string
	DNSZone   string
	DNSPort   int
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
