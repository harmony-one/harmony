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
	DNSSync    DnsSync
	ShardData  ShardDataConfig
}

type DnsSync struct {
	Port          int    // replaces: Network.DNSSyncPort
	Zone          string // replaces: Network.DNSZone
	LegacySyncing bool   // replaces: Network.LegacySyncing
	Client        bool   // replaces: Sync.LegacyClient
	Server        bool   // replaces: Sync.LegacyServer
	ServerPort    int
}

type NetworkConfig struct {
	NetworkType string
	BootNodes   []string
}

type P2pConfig struct {
	Port            int
	IP              string
	KeyFile         string
	DHTDataStore    *string `toml:",omitempty"`
	DiscConcurrency int     // Discovery Concurrency value
	MaxConnsPerIP   int
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
	EnablePruneBeaconChain bool
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
	BlacklistFile  string
	RosettaFixFile string
	AccountSlots   uint64
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
	DebugEnabled      bool // Enables PrivateDebugService APIs, including the EVM tracer
	RateLimterEnabled bool // Enable Rate limiter for RPC
	RequestsPerSecond int  // for RPC rate limiter
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
	Enabled        bool // enable the stream sync protocol
	Downloader     bool // start the sync downloader client
	Concurrency    int  // concurrency used for stream sync protocol
	MinPeers       int  // minimum streams to start a sync task.
	InitStreams    int  // minimum streams in bootstrap to start sync loop.
	DiscSoftLowCap int  // when number of streams is below this value, spin discover during check
	DiscHardLowCap int  // when removing stream, num is below this value, spin discovery immediately
	DiscHighCap    int  // upper limit of streams in one sync protocol
	DiscBatch      int  // size of each discovery
}
