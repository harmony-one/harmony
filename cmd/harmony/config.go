package main

import (
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
	Version   string
	General   generalConfig
	Network   networkConfig
	P2P       p2pConfig
	HTTP      httpConfig
	WS        wsConfig
	Consensus consensusConfig
	BLSKeys   blsConfig
	TxPool    txPoolConfig
	Pprof     pprofConfig
	Log       logConfig
	Devnet    *devnetConfig `toml:",omitempty"`
	Revert    *revertConfig `toml:",omitempty"`
	Legacy    *legacyConfig `toml:",omitempty"`
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

type generalConfig struct {
	NodeType   string
	NoStaking  bool
	ShardID    int
	IsArchival bool
	DataDir    string
}

type consensusConfig struct {
	DelayCommit string
	BlockTime   string
	MinPeers    int
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

type httpConfig struct {
	Enabled bool
	IP      string
	Port    int
}

type wsConfig struct {
	Enabled bool
	IP      string
	Port    int
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
	WebHookConfig string
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
	return config, nil
}

func writeHarmonyConfigToFile(config harmonyConfig, file string) error {
	b, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0644)
}
