package config

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/harmony-one/harmony/internal/cli"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/pelletier/go-toml"
	"github.com/spf13/cobra"
)

// TODO: use specific type wise validation instead of general string types assertion.
func validateHarmonyConfig(config harmonyconfig.HarmonyConfig) error {
	var accepts []string

	nodeType := config.General.NodeType
	accepts = []string{NodeTypeValidator, NodeTypeExplorer}
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

	if config.General.NodeType == NodeTypeExplorer && config.General.ShardID < 0 {
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

func sanityFixHarmonyConfig(hc *harmonyconfig.HarmonyConfig) {
	// When running sync downloader, set sync.Enabled to true
	if hc.Sync.Downloader && !hc.Sync.Enabled {
		fmt.Println("Set Sync.Enabled to true when running stream downloader")
		hc.Sync.Enabled = true
	}
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

func GetDefaultDNSSyncConfig(nt nodeconfig.NetworkType) harmonyconfig.DnsSync {
	zone := nodeconfig.GetDefaultDNSZone(nt)
	port := nodeconfig.GetDefaultDNSPort(nt)
	dnsSync := harmonyconfig.DnsSync{
		Port:       port,
		Zone:       zone,
		ServerPort: nodeconfig.DefaultDNSPort,
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
		dnsSync.Client = false
	default:
		dnsSync.Server = true
		dnsSync.Client = false
	}
	return dnsSync
}

func GetDefaultNetworkConfig(nt nodeconfig.NetworkType) harmonyconfig.NetworkConfig {
	bn := nodeconfig.GetDefaultBootNodes(nt)
	return harmonyconfig.NetworkConfig{
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

func GetDefaultSyncConfig(nt nodeconfig.NetworkType) harmonyconfig.SyncConfig {
	switch nt {
	case nodeconfig.Mainnet:
		return defaultMainnetSyncConfig
	case nodeconfig.Testnet:
		return defaultTestNetSyncConfig
	case nodeconfig.Localnet:
		return defaultLocalNetSyncConfig
	case nodeconfig.Partner:
		return defaultPartnerSyncConfig
	default:
		return defaultElseSyncConfig
	}
}

func GetDefaultCacheConfig(nt nodeconfig.NetworkType) harmonyconfig.CacheConfig {
	cacheConfig := harmonyconfig.CacheConfig{
		Disabled:        defaultCacheConfig.Disabled,
		TrieNodeLimit:   defaultCacheConfig.TrieNodeLimit,
		TriesInMemory:   defaultCacheConfig.TriesInMemory,
		TrieTimeLimit:   defaultCacheConfig.TrieTimeLimit,
		SnapshotLimit:   defaultCacheConfig.SnapshotLimit,
		SnapshotWait:    defaultCacheConfig.SnapshotWait,
		Preimages:       defaultCacheConfig.Preimages,
		SnapshotNoBuild: defaultCacheConfig.SnapshotNoBuild,
	}

	switch nt {
	case nodeconfig.Mainnet:
		cacheConfig.Disabled = true
		cacheConfig.Preimages = true
		cacheConfig.SnapshotLimit = 0
	case nodeconfig.Testnet:
		cacheConfig.Disabled = true
		cacheConfig.Preimages = true
		cacheConfig.SnapshotLimit = 0
	case nodeconfig.Partner:
		cacheConfig.Disabled = true
		cacheConfig.Preimages = true
		cacheConfig.SnapshotLimit = 0
	case nodeconfig.Localnet:
		cacheConfig.Disabled = false
		cacheConfig.Preimages = false
		cacheConfig.SnapshotLimit = 0
	default:
		cacheConfig.Disabled = false
		cacheConfig.Preimages = true
	}
	return cacheConfig
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

func dumpConfig(cmd *cobra.Command, args []string) {
	nt := getNetworkType(cmd)
	config := GetDefaultHmyConfigCopy(nt)

	if err := writeHarmonyConfigToFile(config, args[0]); err != nil {
		fmt.Println(err)
		os.Exit(128)
	}
}

var dumpConfigCmd = &cobra.Command{
	Use:   "dump [config_file]",
	Short: "dump default config file for harmony binary configurations",
	Long:  "dump default config file for harmony binary configurations",
	Args:  cobra.MinimumNArgs(1),
	Run:   dumpConfig,
}

var dumpConfigLegacyCmd = &cobra.Command{
	Use:    "dumpconfig [config_file]",
	Short:  "depricated - use config dump instead",
	Long:   "depricated - use config dump instead",
	Args:   cobra.MinimumNArgs(1),
	Hidden: true,
	Run:    dumpConfig,
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

func loadHarmonyConfig(file string) (harmonyconfig.HarmonyConfig, string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return harmonyconfig.HarmonyConfig{}, "", err
	}
	config, migratedVer, err := migrateConf(b)
	if err != nil {
		return harmonyconfig.HarmonyConfig{}, "", err
	}

	return config, migratedVer, nil
}

func updateConfigFile(file string) error {
	configBytes, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	backup := fmt.Sprintf("%s.backup", file)
	if err := os.WriteFile(backup, configBytes, 0664); err != nil {
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

func writeHarmonyConfigToFile(config harmonyconfig.HarmonyConfig, file string) error {
	b, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(file, b, 0644)
}

var configFlag = cli.StringFlag{
	Name:      "config",
	Usage:     "load node config from the config toml file.",
	Shorthand: "c",
	DefValue:  "",
}

func applyRootFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	// Misc flags shall be applied first since legacy ip / port is overwritten
	// by new ip / port flags
	applyLegacyMiscFlags(cmd, config)
	applyGeneralFlags(cmd, config)
	applyNetworkFlags(cmd, config)
	applyDNSSyncFlags(cmd, config)
	applyP2PFlags(cmd, config)
	applyHTTPFlags(cmd, config)
	applyWSFlags(cmd, config)
	applyRPCOptFlags(cmd, config)
	applyBLSFlags(cmd, config)
	applyConsensusFlags(cmd, config)
	applyTxPoolFlags(cmd, config)
	applyPprofFlags(cmd, config)
	applyLogFlags(cmd, config)
	applySysFlags(cmd, config)
	applyDevnetFlags(cmd, config)
	applyRevertFlags(cmd, config)
	applyPreimageFlags(cmd, config)
	applyPrometheusFlags(cmd, config)
	applySyncFlags(cmd, config)
	applyShardDataFlags(cmd, config)
	applyGPOFlags(cmd, config)
	applyCacheFlags(cmd, config)
}

func registerRootCmdFlags(rootCmd *cobra.Command) error {
	flags := getRootFlags()

	return cli.RegisterFlags(rootCmd, flags)
}

func GetHarmonyConfig(cmd *cobra.Command) (harmonyconfig.HarmonyConfig, error) {
	var (
		config         harmonyconfig.HarmonyConfig
		err            error
		migratedFrom   string
		configFile     string
		isUsingDefault bool
	)
	if cli.IsFlagChanged(cmd, configFlag) {
		configFile = cli.GetStringFlagValue(cmd, configFlag)
		config, migratedFrom, err = loadHarmonyConfig(configFile)
	} else {
		nt := getNetworkType(cmd)
		config = GetDefaultHmyConfigCopy(nt)
		isUsingDefault = true
	}
	if err != nil {
		return harmonyconfig.HarmonyConfig{}, err
	}
	if migratedFrom != defaultConfig.Version && !isUsingDefault {
		fmt.Printf("Old config version detected %s\n",
			migratedFrom)
		stat, _ := os.Stdin.Stat()
		// Ask to update if only using terminal
		if stat.Mode()&os.ModeCharDevice != 0 {
			if promptConfigUpdate() {
				err := updateConfigFile(configFile)
				if err != nil {
					fmt.Printf("Could not update config - %s", err.Error())
					fmt.Println("Update config manually with `./harmony config update [config_file]`")
				}
			}

		} else {
			fmt.Println("Update saved config with `./harmony config update [config_file]`")
		}
	}

	applyRootFlags(cmd, &config)

	if err := validateHarmonyConfig(config); err != nil {
		return harmonyconfig.HarmonyConfig{}, err
	}
	sanityFixHarmonyConfig(&config)
	return config, nil
}

func Init(rootCmd *cobra.Command) {
	rand.Seed(time.Now().UnixNano())
	cli.SetParseErrorHandle(func(err error) {
		os.Exit(128) // 128 - invalid command line arguments
	})
	configCmd.AddCommand(dumpConfigCmd)
	configCmd.AddCommand(updateConfigCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(dumpConfigLegacyCmd)
	rootCmd.AddCommand(dumpDBCmd)
	rootCmd.AddCommand(inspectDBCmd)

	if err := registerRootCmdFlags(rootCmd); err != nil {
		os.Exit(2)
	}
	if err := registerDumpConfigFlags(); err != nil {
		os.Exit(2)
	}
	if err := registerDumpDBFlags(); err != nil {
		os.Exit(2)
	}
	if err := registerInspectionFlags(); err != nil {
		os.Exit(2)
	}
}
