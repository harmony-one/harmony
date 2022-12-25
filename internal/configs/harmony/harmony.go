package harmony

import (
	"reflect"
	"strings"
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
	LocalAccountsFile string
	GlobalSlots       uint64
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
	Enabled        bool             // enable the stream sync protocol
	Downloader     bool             // start the sync downloader client
	StagedSync     bool             // use staged sync
	StagedSyncCfg  StagedSyncConfig // staged sync configurations
	Concurrency    int              // concurrency used for stream sync protocol
	MinPeers       int              // minimum streams to start a sync task.
	InitStreams    int              // minimum streams in bootstrap to start sync loop.
	DiscSoftLowCap int              // when number of streams is below this value, spin discover during check
	DiscHardLowCap int              // when removing stream, num is below this value, spin discovery immediately
	DiscHighCap    int              // upper limit of streams in one sync protocol
	DiscBatch      int              // size of each discovery
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
}
