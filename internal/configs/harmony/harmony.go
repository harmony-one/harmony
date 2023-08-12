package harmony

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
)

// HarmonyConfig contains all the configs user can set for running harmony binary. Served as the bridge
// from user set flags to internal node configs. Also user can persist this structure to a toml file
// to avoid inputting all arguments.
type HarmonyConfig struct {
	Version    string
	General    GeneralConfig
	Network    NetworkConfig
	P2P        P2pConfig
	HTTP       HttpConfig
	WS         WsConfig
	RPCOpt     RpcOptConfig
	BLSKeys    BlsConfig
	TxPool     TxPoolConfig
	Pprof      PprofConfig
	Log        LogConfig
	Sync       SyncConfig
	Sys        *SysConfig        `toml:",omitempty"`
	Consensus  *ConsensusConfig  `toml:",omitempty"`
	Devnet     *DevnetConfig     `toml:",omitempty"`
	Revert     *RevertConfig     `toml:",omitempty"`
	Legacy     *LegacyConfig     `toml:",omitempty"`
	Prometheus *PrometheusConfig `toml:",omitempty"`
	TiKV       *TiKVConfig       `toml:",omitempty"`
	DNSSync    DnsSync
	ShardData  ShardDataConfig
	GPO        GasPriceOracleConfig
}

func (hc HarmonyConfig) ToRPCServerConfig() nodeconfig.RPCServerConfig {
	readTimeout, err := time.ParseDuration(hc.HTTP.ReadTimeout)
	if err != nil {
		readTimeout, _ = time.ParseDuration(nodeconfig.DefaultHTTPTimeoutRead)
		utils.Logger().Warn().
			Str("provided", hc.HTTP.ReadTimeout).
			Dur("updated", readTimeout).
			Msg("Sanitizing invalid http read timeout")
	}
	writeTimeout, err := time.ParseDuration(hc.HTTP.WriteTimeout)
	if err != nil {
		writeTimeout, _ = time.ParseDuration(nodeconfig.DefaultHTTPTimeoutWrite)
		utils.Logger().Warn().
			Str("provided", hc.HTTP.WriteTimeout).
			Dur("updated", writeTimeout).
			Msg("Sanitizing invalid http write timeout")
	}
	idleTimeout, err := time.ParseDuration(hc.HTTP.IdleTimeout)
	if err != nil {
		idleTimeout, _ = time.ParseDuration(nodeconfig.DefaultHTTPTimeoutIdle)
		utils.Logger().Warn().
			Str("provided", hc.HTTP.IdleTimeout).
			Dur("updated", idleTimeout).
			Msg("Sanitizing invalid http idle timeout")
	}
	evmCallTimeout, err := time.ParseDuration(hc.RPCOpt.EvmCallTimeout)
	if err != nil {
		evmCallTimeout, _ = time.ParseDuration(nodeconfig.DefaultEvmCallTimeout)
		utils.Logger().Warn().
			Str("provided", hc.RPCOpt.EvmCallTimeout).
			Dur("updated", evmCallTimeout).
			Msg("Sanitizing invalid evm_call timeout")
	}
	return nodeconfig.RPCServerConfig{
		HTTPEnabled:        hc.HTTP.Enabled,
		HTTPIp:             hc.HTTP.IP,
		HTTPPort:           hc.HTTP.Port,
		HTTPAuthPort:       hc.HTTP.AuthPort,
		HTTPTimeoutRead:    readTimeout,
		HTTPTimeoutWrite:   writeTimeout,
		HTTPTimeoutIdle:    idleTimeout,
		WSEnabled:          hc.WS.Enabled,
		WSIp:               hc.WS.IP,
		WSPort:             hc.WS.Port,
		WSAuthPort:         hc.WS.AuthPort,
		DebugEnabled:       hc.RPCOpt.DebugEnabled,
		EthRPCsEnabled:     hc.RPCOpt.EthRPCsEnabled,
		StakingRPCsEnabled: hc.RPCOpt.StakingRPCsEnabled,
		LegacyRPCsEnabled:  hc.RPCOpt.LegacyRPCsEnabled,
		RpcFilterFile:      hc.RPCOpt.RpcFilterFile,
		RateLimiterEnabled: hc.RPCOpt.RateLimterEnabled,
		RequestsPerSecond:  hc.RPCOpt.RequestsPerSecond,
		EvmCallTimeout:     evmCallTimeout,
	}
}

type DnsSync struct {
	Port       int    // replaces: Network.DNSSyncPort
	Zone       string // replaces: Network.DNSZone
	Client     bool   // replaces: Sync.LegacyClient
	Server     bool   // replaces: Sync.LegacyServer
	ServerPort int
}

type NetworkConfig struct {
	NetworkType string
	BootNodes   []string
}

type P2pConfig struct {
	Port                 int
	IP                   string
	KeyFile              string
	DHTDataStore         *string `toml:",omitempty"`
	DiscConcurrency      int     // Discovery Concurrency value
	MaxConnsPerIP        int
	DisablePrivateIPScan bool
	MaxPeers             int64
	// In order to disable Connection Manager, it only needs to
	// set both the high and low watermarks to zero. In this way,
	// using Connection Manager will be an optional feature.
	ConnManagerLowWatermark  int
	ConnManagerHighWatermark int
	WaitForEachPeerToConnect bool
}

type GeneralConfig struct {
	NodeType               string
	NoStaking              bool
	ShardID                int
	IsArchival             bool
	IsBackup               bool
	IsBeaconArchival       bool
	IsOffline              bool
	DataDir                string
	TraceEnable            bool
	EnablePruneBeaconChain bool
	RunElasticMode         bool
	TriesInMemory          int
}

type TiKVConfig struct {
	Debug bool

	PDAddr                      []string
	Role                        string
	StateDBCacheSizeInMB        uint32
	StateDBCachePersistencePath string
	StateDBRedisServerAddr      []string
	StateDBRedisLRUTimeInDay    uint32
}

type ShardDataConfig struct {
	EnableShardData bool
	DiskCount       int
	ShardCount      int
	CacheTime       int
	CacheSize       int
}

type GasPriceOracleConfig struct {
	// the number of blocks to sample
	Blocks int
	// the number of transactions to sample, per block
	Transactions int
	// the percentile to pick from there
	Percentile int
	// the default gas price, if the above data is not available
	DefaultPrice int64
	// the maximum suggested gas price
	MaxPrice int64
	// when block usage (gas) for last `Blocks` blocks is below `LowUsageThreshold`,
	// we return the Default price
	LowUsageThreshold int
	// hack: our block header reports an 80m gas limit, but it is actually 30M.
	// if set to non-zero, this is applied UNCHECKED
	BlockGasLimit int
}

type ConsensusConfig struct {
	MinPeers     int
	AggregateSig bool
}

type BlsConfig struct {
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
	BlacklistFile     string
	AllowedTxsFile    string
	RosettaFixFile    string
	AccountSlots      uint64
	AccountQueue      uint64
	GlobalQueue       uint64
	LocalAccountsFile string
	GlobalSlots       uint64
	Lifetime          time.Duration
	PriceLimit        PriceLimit
	PriceBump         uint64
}

type PprofConfig struct {
	Enabled            bool
	ListenAddr         string
	Folder             string
	ProfileNames       []string
	ProfileIntervals   []int
	ProfileDebugValues []int
}

type LogConfig struct {
	Console       bool
	Folder        string
	FileName      string
	RotateSize    int
	RotateCount   int
	RotateMaxAge  int
	Verbosity     int
	VerbosePrints LogVerbosePrints
	Context       *LogContext `toml:",omitempty"`
}

type LogVerbosePrints struct {
	Config bool
}

func FlagSliceToLogVerbosePrints(verbosePrintsFlagSlice []string) LogVerbosePrints {
	verbosePrints := LogVerbosePrints{}
	verbosePrintsReflect := reflect.Indirect(reflect.ValueOf(&verbosePrints))
	for _, verbosePrint := range verbosePrintsFlagSlice {
		verbosePrint = strings.Title(verbosePrint)
		field := verbosePrintsReflect.FieldByName(verbosePrint)
		if field.IsValid() && field.CanSet() {
			field.SetBool(true)
		}
	}

	return verbosePrints
}

type LogContext struct {
	IP   string
	Port int
}

type SysConfig struct {
	NtpServer string
}

type HttpConfig struct {
	Enabled        bool
	IP             string
	Port           int
	AuthPort       int
	RosettaEnabled bool
	RosettaPort    int
	ReadTimeout    string
	WriteTimeout   string
	IdleTimeout    string
}

type WsConfig struct {
	Enabled  bool
	IP       string
	Port     int
	AuthPort int
}

type RpcOptConfig struct {
	DebugEnabled       bool   // Enables PrivateDebugService APIs, including the EVM tracer
	EthRPCsEnabled     bool   // Expose Eth RPCs
	StakingRPCsEnabled bool   // Expose Staking RPCs
	LegacyRPCsEnabled  bool   // Expose Legacy RPCs
	RpcFilterFile      string // Define filters to enable/disable RPC exposure
	RateLimterEnabled  bool   // Enable Rate limiter for RPC
	RequestsPerSecond  int    // for RPC rate limiter
	EvmCallTimeout     string // Timeout for eth_call
}

type DevnetConfig struct {
	NumShards   int
	ShardSize   int
	HmyNodeSize int
	SlotsLimit  int // HIP-16: The absolute number of maximum effective slots per shard limit for each validator. 0 means no limit.
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
	Enabled              bool             // enable the stream sync protocol
	Downloader           bool             // start the sync downloader client
	StagedSync           bool             // use staged sync
	StagedSyncCfg        StagedSyncConfig // staged sync configurations
	Concurrency          int              // concurrency used for stream sync protocol
	MinPeers             int              // minimum streams to start a sync task.
	InitStreams          int              // minimum streams in bootstrap to start sync loop.
	MaxAdvertiseWaitTime int              // maximum time duration between advertisements
	DiscSoftLowCap       int              // when number of streams is below this value, spin discover during check
	DiscHardLowCap       int              // when removing stream, num is below this value, spin discovery immediately
	DiscHighCap          int              // upper limit of streams in one sync protocol
	DiscBatch            int              // size of each discovery
}

type StagedSyncConfig struct {
	TurboMode              bool   // turn on turbo mode
	DoubleCheckBlockHashes bool   // double check all block hashes before download blocks
	MaxBlocksPerSyncCycle  uint64 // max number of blocks per each sync cycle, if set to zero, all blocks will be synced in one full cycle
	MaxBackgroundBlocks    uint64 // max number of background blocks in turbo mode
	InsertChainBatchSize   int    // number of blocks to build a batch and insert to chain in staged sync
	MaxMemSyncCycleSize    uint64 // max number of blocks to use a single transaction for staged sync
	VerifyAllSig           bool   // verify signatures for all blocks regardless of height and batch size
	VerifyHeaderBatchSize  uint64 // batch size to verify header before insert to chain
	UseMemDB               bool   // it uses memory by default. set it to false to use disk
	LogProgress            bool   // log the full sync progress in console
	DebugMode              bool   // log every single process and error to help to debug syncing issues (DebugMode is not accessible to the end user and is only an aid for development)
}

type PriceLimit int64

func (s *PriceLimit) UnmarshalTOML(data interface{}) error {
	switch v := data.(type) {
	case float64:
		*s = PriceLimit(v)
	case int64:
		*s = PriceLimit(v)
	case PriceLimit:
		*s = v
	default:
		return fmt.Errorf("PriceLimit.UnmarshalTOML: %T", data)
	}
	return nil
}

func (s PriceLimit) MarshalTOML() ([]byte, error) {
	if s > 1_000_000_000 {
		return []byte(fmt.Sprintf("%de9", s/1_000_000_000)), nil
	}
	return []byte(fmt.Sprintf("%d", s)), nil
}
