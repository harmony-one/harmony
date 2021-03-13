package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/harmony-one/harmony/internal/cli"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/pelletier/go-toml"
	"github.com/spf13/cobra"
)

// harmonyConfig contains all the configs user can set for running harmony binary. Served as the bridge
// from user set flags to internal node configs. Also user can persist this structure to a toml file
// to avoid inputting all arguments.
type harmonyConfig struct {
	Version    string
	General    generalConfig
	Network    networkConfig
	P2P        p2pConfig
	HTTP       httpConfig
	WS         wsConfig
	RPCOpt     rpcOptConfig
	BLSKeys    blsConfig
	TxPool     txPoolConfig
	Pprof      pprofConfig
	Log        logConfig
	Sync       syncConfig
	Sys        *sysConfig        `toml:",omitempty"`
	Consensus  *consensusConfig  `toml:",omitempty"`
	Devnet     *devnetConfig     `toml:",omitempty"`
	Revert     *revertConfig     `toml:",omitempty"`
	Legacy     *legacyConfig     `toml:",omitempty"`
	Prometheus *prometheusConfig `toml:",omitempty"`
}

type networkConfig struct {
	NetworkType string
	BootNodes   []string

	LegacySyncing bool // if true, use LegacySyncingPeerProvider
	DNSZone       string
	DNSPort       int
}

type p2pConfig struct {
	Port         int
	IP           string
	KeyFile      string
	DHTDataStore *string `toml:",omitempty"`
}

type generalConfig struct {
	NodeType         string
	NoStaking        bool
	ShardID          int
	IsArchival       bool
	IsBeaconArchival bool
	IsOffline        bool
	DataDir          string
}

type consensusConfig struct {
	MinPeers     int
	AggregateSig bool
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
	BlacklistFile string
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

type sysConfig struct {
	NtpServer string
}

type httpConfig struct {
	Enabled        bool
	IP             string
	Port           int
	RosettaEnabled bool
	RosettaPort    int
}

type wsConfig struct {
	Enabled bool
	IP      string
	Port    int
}

type rpcOptConfig struct {
	DebugEnabled bool // Enables PrivateDebugService APIs, including the EVM tracer
}

type devnetConfig struct {
	NumShards   int
	ShardSize   int
	HmyNodeSize int
}

// TODO: make `revert` to a separate command
type revertConfig struct {
	RevertBeacon bool
	RevertTo     int
	RevertBefore int
}

type legacyConfig struct {
	WebHookConfig         *string `toml:",omitempty"`
	TPBroadcastInvalidTxn *bool   `toml:",omitempty"`
}

type prometheusConfig struct {
	Enabled    bool
	IP         string
	Port       int
	EnablePush bool
	Gateway    string
}

type syncConfig struct {
	LegacyServer   bool // provide the gRPC sync protocol server
	LegacyClient   bool // aside from stream sync protocol, also run gRPC client to get blocks
	Concurrency    int  // concurrency used for stream sync protocol
	MinPeers       int  // minimum streams to start a sync task.
	InitStreams    int  // minimum streams in bootstrap to start sync loop.
	DiscSoftLowCap int  // when number of streams is below this value, spin discover during check
	DiscHardLowCap int  // when removing stream, num is below this value, spin discovery immediately
	DiscHighCap    int  // upper limit of streams in one sync protocol
	DiscBatch      int  // size of each discovery
}

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

	if config.General.NodeType == nodeTypeExplorer && config.General.ShardID < 0 {
		return errors.New("flag --run.shard must be specified for explorer node")
	}

	if config.General.IsOffline && config.P2P.IP != nodeconfig.DefaultLocalListenIP {
		return fmt.Errorf("flag --run.offline must have p2p IP be %v", nodeconfig.DefaultLocalListenIP)
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

func getDefaultNetworkConfig(nt nodeconfig.NetworkType) networkConfig {
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

func getDefaultSyncConfig(nt nodeconfig.NetworkType) syncConfig {
	switch nt {
	case nodeconfig.Mainnet:
		return defaultMainnetSyncConfig
	case nodeconfig.Testnet:
		return defaultTestNetSyncConfig
	default:
		return defaultElseSyncConfig
	}
}

var dumpConfigCmd = &cobra.Command{
	Use:   "dumpconfig [config_file]",
	Short: "dump the config file for harmony binary configurations",
	Long:  "dump the config file for harmony binary configurations",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		nt := getNetworkType(cmd)
		config := getDefaultHmyConfigCopy(nt)

		if err := writeHarmonyConfigToFile(config, args[0]); err != nil {
			fmt.Println(err)
			os.Exit(128)
		}
	},
}

func registerDumpConfigFlags() error {
	return cli.RegisterFlags(dumpConfigCmd, []cli.Flag{networkTypeFlag})
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

	// Correct for old config version load (port 0 is invalid anyways)
	if config.HTTP.RosettaPort == 0 {
		config.HTTP.RosettaPort = defaultConfig.HTTP.RosettaPort
	}
	if config.P2P.IP == "" {
		config.P2P.IP = defaultConfig.P2P.IP
	}
	if config.Prometheus == nil {
		config.Prometheus = defaultConfig.Prometheus
	}
	return config, nil
}

func writeHarmonyConfigToFile(config harmonyConfig, file string) error {
	b, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0644)
}
