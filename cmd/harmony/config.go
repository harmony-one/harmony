package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/pelletier/go-toml"
)

const tomlConfigVersion = "1.0.0"

type harmonyConfig struct {
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
	Folder     string
	FileName   string
	RotateSize int
	Verbosity  int
	Context    *logContext `toml:",omitempty"`
}

type logContext struct {
	IP   string
	Port int
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

var defaultConfig = harmonyConfig{
	Version: tomlConfigVersion,
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
	RevertBefore: -1,
	RevertTo:     -1,
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

// TODO: use specific type wise validation instead of general string types assertion.
func validateHarmonyConfig(config harmonyConfig) error {
	var accepts []string

	nodeType := config.General.NodeType
	accepts = []string{nodeTypeValidator, nodeTypeExplorer}
	if err := checkStringAccepted("--run", nodeType, accepts); err != nil {
		return err
	}

	netType := config.Network.NetworkType
	parsed := parseNetworkType(netType)
	if len(parsed) == 0 {
		return fmt.Errorf("unknown network type: %v", netType)
	}

	passType := config.BLSKeys.PassSrcType
	accepts = []string{blsPassTypeAuto, blsPassTypeFile, blsPassTypePrompt}
	if err := checkStringAccepted("--bls.pass.src", passType, accepts); err != nil {
		return err
	}

	kmsType := config.BLSKeys.KMSConfigSrcType
	accepts = []string{kmsConfigTypeShared, kmsConfigTypePrompt, kmsConfigTypeFile}
	if err := checkStringAccepted("--bls.kms.src", kmsType, accepts); err != nil {
		return err
	}

	return nil
}

func checkStringAccepted(flag string, val string, accepts []string) error {
	for _, accept := range accepts {
		if val == accept {
			return nil
		}
	}
	acceptsStr := strings.Join(accepts, ", ")
	return fmt.Errorf("unknown arg for %s: %s (%v)", flag, val, acceptsStr)
}

func getDefaultNetworkConfig(raw string) networkConfig {
	nt := parseNetworkType(raw)

	bn := nodeconfig.GetDefaultBootNodes(nt)
	zone := nodeconfig.GetDefaultDNSZone(nt)
	port := nodeconfig.GetDefaultDNSPort(nt)
	return networkConfig{
		NetworkType:   string(nt),
		BootNodes:     bn,
		LegacySyncing: false,
		DNSZone:       zone,
		DNSPort:       port,
	}
}

func parseNetworkType(nt string) nodeconfig.NetworkType {
	switch nt {
	case "mainnet":
		return nodeconfig.Mainnet
	case "testnet":
		return nodeconfig.Testnet
	case "pangaea", "staking", "stk":
		return nodeconfig.Pangaea
	case "partner":
		return nodeconfig.Partner
	case "stressnet", "stress", "stn":
		return nodeconfig.Stressnet
	case "localnet":
		return nodeconfig.Localnet
	case "devnet", "dev":
		return nodeconfig.Devnet
	default:
		return ""
	}
}

func loadHarmonyConfig(file string) (harmonyConfig, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return harmonyConfig{}, err
	}

	var config harmonyConfig
	if err := toml.Unmarshal(b, &config); err != nil {
		return harmonyConfig{}, err
	}
	return config, nil
}

func writeHarmonyConfigToFile(config *harmonyConfig, file string) error {
	b, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0644)
}
