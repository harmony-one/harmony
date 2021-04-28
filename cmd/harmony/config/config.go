package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/pelletier/go-toml"
)

// HarmonyConfig contains all the configs user can set for running harmony binary. Served as the bridge
// from user set flags to internal node configs. Also user can persist this structure to a toml file
// to avoid inputting all arguments.
type HarmonyConfig struct {
	Version    string
	General    GeneralConfig
	Network    NetworkConfig
	P2P        P2PConfig
	HTTP       HTTPConfig
	WS         WSConfig
	RPCOpt     RPCOptConfig
	BLSKeys    BLSConfig
	TxPool     TxPoolConfig
	Pprof      PprofConfig
	Log        LogConfig
	Sync       SyncConfig
	DNSSync    DNSSyncConfig
	Sys        *SysConfig        `toml:",omitempty"`
	Consensus  *ConsensusConfig  `toml:",omitempty"`
	Devnet     *DevnetConfig     `toml:",omitempty"`
	Revert     *RevertConfig     `toml:",omitempty"`
	Legacy     *LegacyConfig     `toml:",omitempty"`
	Prometheus *PrometheusConfig `toml:",omitempty"`
}

// IsLatestVersion returns whether this config was loaded from the latest version format.
func (c HarmonyConfig) IsLatestVersion() bool {
	return c.Version == TOMLConfigVersion
}

// NetworkConfig contains network related config options.
type NetworkConfig struct {
	NetworkType string
	BootNodes   []string
}

type P2PConfig struct {
	Port         int
	IP           string
	KeyFile      string
	DHTDataStore *string `toml:",omitempty"`
}

type GeneralConfig struct {
	NodeType         string
	NoStaking        bool
	ShardID          int
	IsArchival       bool
	IsBeaconArchival bool
	IsOffline        bool
	DataDir          string
}

type ConsensusConfig struct {
	MinPeers     int
	AggregateSig bool
}

type BLSConfig struct {
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

type TxPoolConfig struct {
	BlacklistFile string
}

type PprofConfig struct {
	Enabled    bool
	ListenAddr string
}

type LogConfig struct {
	Folder     string
	FileName   string
	RotateSize int
	Verbosity  int
	Context    *LogContext `toml:",omitempty"`
}

type LogContext struct {
	IP   string
	Port int
}

type SysConfig struct {
	NtpServer string
}

type HTTPConfig struct {
	Enabled        bool
	IP             string
	Port           int
	RosettaEnabled bool
	RosettaPort    int
}

type WSConfig struct {
	Enabled bool
	IP      string
	Port    int
}

type RPCOptConfig struct {
	DebugEnabled bool // Enables PrivateDebugService APIs, including the EVM tracer
}

type DevnetConfig struct {
	NumShards   int
	ShardSize   int
	HmyNodeSize int
}

// TODO: make `revert` to a separate command
type RevertConfig struct {
	RevertBeacon bool
	RevertTo     int
	RevertBefore int
}

type LegacyConfig struct {
	WebHookConfig         *string `toml:",omitempty"`
	TPBroadcastInvalidTxn *bool   `toml:",omitempty"`
}

type PrometheusConfig struct {
	Enabled    bool
	IP         string
	Port       int
	EnablePush bool
	Gateway    string
}

type SyncConfig struct {
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

// IsEmpty returns whether this structure is zero value.
func (c SyncConfig) IsEmpty() bool {
	return c == SyncConfig{}
}

type DNSSyncConfig struct {
	LegacySyncing bool // if true, use LegacySyncingPeerProvider
	DNSZone       string
	DNSPort       int
	LegacyServer  bool // provide the gRPC sync protocol server
	LegacyClient  bool // aside from stream sync protocol, also run gRPC client to get blocks
}

// TODO: use specific type wise validation instead of general string types assertion.
func ValidateHarmonyConfig(config HarmonyConfig) error {
	var accepts []string

	nodeType := config.General.NodeType
	accepts = []string{NodeTypeValidator, NodeTypeExplorer}
	if err := checkStringAccepted("--run", nodeType, accepts); err != nil {
		return err
	}

	netType := config.Network.NetworkType
	parsed := ParseNetworkType(netType)
	if len(parsed) == 0 {
		return fmt.Errorf("unknown network type: %v", netType)
	}

	passType := config.BLSKeys.PassSrcType
	accepts = []string{BLSPassTypeAuto, BLSPassTypeFile, BLSPassTypePrompt}
	if err := checkStringAccepted("--bls.pass.src", passType, accepts); err != nil {
		return err
	}

	kmsType := config.BLSKeys.KMSConfigSrcType
	accepts = []string{KMSConfigTypeShared, KMSConfigTypePrompt, KMSConfigTypeFile}
	if err := checkStringAccepted("--bls.kms.src", kmsType, accepts); err != nil {
		return err
	}

	if config.General.NodeType == NodeTypeExplorer && config.General.ShardID < 0 {
		return errors.New("flag --run.shard must be specified for explorer node")
	}

	if config.General.IsOffline && config.P2P.IP != nodeconfig.DefaultLocalListenIP {
		return fmt.Errorf("flag --run.offline must have p2p IP be %v", nodeconfig.DefaultLocalListenIP)
	}

	if !config.Sync.Downloader && !config.DNSSync.LegacyClient {
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

func GetDefaultNetworkConfig(nt nodeconfig.NetworkType) NetworkConfig {
	bn := nodeconfig.GetDefaultBootNodes(nt)
	return NetworkConfig{
		NetworkType: string(nt),
		BootNodes:   bn,
	}
}

func ParseNetworkType(nt string) nodeconfig.NetworkType {
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

func GetDefaultSyncConfig(nt nodeconfig.NetworkType) SyncConfig {
	switch nt {
	case nodeconfig.Mainnet:
		return DefaultMainnetSyncConfig
	case nodeconfig.Testnet:
		return DefaultTestNetSyncConfig
	case nodeconfig.Localnet:
		return defaultLocalNetSyncConfig
	default:
		return defaultElseSyncConfig
	}
}

func GetDefaultDNSSyncConfig(nt nodeconfig.NetworkType) DNSSyncConfig {
	config := DNSSyncConfig{
		LegacySyncing: false,
		DNSZone:       nodeconfig.GetDefaultDNSZone(nt),
		DNSPort:       nodeconfig.GetDefaultDNSPort(nt),
		LegacyServer:  true,
		LegacyClient:  true,
	}
	switch nt {
	case nodeconfig.Mainnet:
	case nodeconfig.Testnet:
	case nodeconfig.Localnet:
	default:
		config.LegacyClient = false
	}
	return config
}

// LoadHarmonyConfig loads toml configuration for the binary node from the specified file on disk.
func LoadHarmonyConfig(file string) (HarmonyConfig, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return HarmonyConfig{}, err
	}
	var header struct {
		Version string
	}
	if err := toml.Unmarshal(b, &header); err != nil {
		return HarmonyConfig{}, fmt.Errorf("reading config version: %w", err)
	}
	var config HarmonyConfig
	// lexicographicaly compare
	if header.Version <= v1TOMLConfigVersion {
		config, err = loadHarmonyConfigV1(b)
	} else {
		err = toml.Unmarshal(b, &config)
	}
	if err != nil {
		return HarmonyConfig{}, err
	}

	// Correct for old config version load (port 0 is invalid anyways)
	if config.HTTP.RosettaPort == 0 {
		config.HTTP.RosettaPort = DefaultConfig.HTTP.RosettaPort
	}
	if config.P2P.IP == "" {
		config.P2P.IP = DefaultConfig.P2P.IP
	}
	if config.Prometheus == nil {
		config.Prometheus = DefaultConfig.Prometheus
	}
	return config, nil
}

func WriteHarmonyConfigToFile(config HarmonyConfig, file string) error {
	b, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0644)
}

// BackupAndUpgradeConfigToTheLatestVersion backups previous version of config file and persists on the disk the latest format.
// It returns file name of the backup.
func BackupAndUpgradeConfigToTheLatestVersion(cfg HarmonyConfig, fileName string) (string, error) {
	oldFileName := fileName + ".old"
	success := false
	for tries := 0; tries < 5; tries++ {
		_, err := os.Stat(oldFileName)
		if os.IsNotExist(err) {
			success = true
			break
		}
		oldFileName = oldFileName + ".old"
	}
	if !success {
		return "", fmt.Errorf("unable to locate appropriate non-existing file name for backing up the old config: %s", oldFileName)
	}
	if err := os.Rename(fileName, oldFileName); err != nil {
		return "", fmt.Errorf("renaming %s to %s: %w", fileName, oldFileName, err)
	}
	cfg.Version = TOMLConfigVersion
	if err := WriteHarmonyConfigToFile(cfg, fileName); err != nil {
		// try to undo the rename
		rErr := os.Rename(oldFileName, fileName)
		if rErr != nil {
			return "", fmt.Errorf("renaming %s to %s: %s, original error: %w", oldFileName, fileName, rErr.Error(), err)
		}
		return "", err
	}
	return oldFileName, nil
}
