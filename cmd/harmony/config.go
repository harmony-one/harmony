package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

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
	DNSSync    dnsSync
}

type dnsSync struct {
	Port          int    // replaces: Network.DNSSyncPort
	Zone          string // replaces: Network.DNSZone
	LegacySyncing bool   // replaces: Network.LegacySyncing
	Client        bool   // replaces: Sync.LegacyClient
	Server        bool   // replaces Sync.LegacyServer
	ServerPort    int
}

type networkConfig struct {
	NetworkType string
	BootNodes   []string
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
	// TODO: Remove this bool after stream sync is fully up.
	Downloader     bool // start the sync downloader client
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

	if !config.Sync.Downloader && !config.DNSSync.Client {
		// There is no module up for sync
		return errors.New("either --sync.downloader or --sync.legacy.client shall be enabled")
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

func getDefaultDNSSyncConfig(nt nodeconfig.NetworkType) dnsSync {
	zone := nodeconfig.GetDefaultDNSZone(nt)
	port := nodeconfig.GetDefaultDNSPort(nt)
	dnsSync := dnsSync{
		Port:          port,
		Zone:          zone,
		LegacySyncing: false,
		ServerPort:    nodeconfig.DefaultDNSPort,
	}
	switch nt {
	case nodeconfig.Mainnet:
		dnsSync.Server = true
		dnsSync.Client = true
	case nodeconfig.Testnet:
		dnsSync.Server = true
		dnsSync.Client = true
	case nodeconfig.Localnet:
		dnsSync.Server = true
		dnsSync.Client = true
	default:
		dnsSync.Server = true
		dnsSync.Client = false
	}
	return dnsSync
}

func getDefaultNetworkConfig(nt nodeconfig.NetworkType) networkConfig {
	bn := nodeconfig.GetDefaultBootNodes(nt)
	return networkConfig{
		NetworkType: string(nt),
		BootNodes:   bn,
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
	case nodeconfig.Localnet:
		return defaultLocalNetSyncConfig
	default:
		return defaultElseSyncConfig
	}
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "dump or update config",
	Long:  "",
}

var updateConfigCmd = &cobra.Command{
	Use:   "update [config_file]",
	Short: "update config to latest version",
	Long:  "updates config file to latest version, preserving values",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := updateConfigFile(args[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(128)
		}
	},
}

var dumpConfigCmd = &cobra.Command{
	Use:   "dump [config_file]",
	Short: "dump default file for harmony binary configurations",
	Long:  "dump default config file for harmony binary configurations",
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

func promptConfigUpdate() bool {
	var readStr string
	fmt.Println("Do you want to update config to the latest version: [y/N]")
	timeoutTimer := time.NewTimer(time.Second * 15)
	read := make(chan string)
	go func() {
		fmt.Scanf("%s", &readStr)
		read <- readStr
	}()
	select {
	case <-timeoutTimer.C:
		fmt.Println("Timed out - update manually with ./harmony config update [config_file]")
		return false
	case <-read:
		readStr = strings.TrimSpace(readStr)
		if len(readStr) > 1 {
			readStr = readStr[0:1]
		}
		readStr = strings.ToLower(readStr)
		return readStr == "y"
	}
}

func loadHarmonyConfig(file string) (harmonyConfig, string, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return harmonyConfig{}, "", err
	}
	config, migratedVer, err := migrateConf(b)
	if err != nil {
		return harmonyConfig{}, "", err
	}

	return config, migratedVer, nil
}

func updateConfigFile(file string) error {
	configBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	backup := fmt.Sprintf("%s.backup", file)
	if err := ioutil.WriteFile(backup, configBytes, 0664); err != nil {
		return err
	}
	fmt.Printf("Original config backed up to %s\n", fmt.Sprintf("%s.backup", file))
	config, migratedFromVer, err := migrateConf(configBytes)
	if err != nil {
		return err
	}
	if err := writeHarmonyConfigToFile(config, file); err != nil {
		return err
	}
	fmt.Printf("Successfully migrated %s from %s to %s \n", file, migratedFromVer, config.Version)
	return nil
}

func writeHarmonyConfigToFile(config harmonyConfig, file string) error {
	b, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0644)
}
