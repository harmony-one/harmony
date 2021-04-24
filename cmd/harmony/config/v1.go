package config

// harmonyConfigV1 contains all the configs user can set for running harmony binary for versions <= 1.0.4.
type harmonyConfigV1 struct {
	Version    string
	General    generalConfigV1
	Network    networkConfigV1
	P2P        p2pConfigV1
	HTTP       httpConfigV1
	WS         wsConfigV1
	RPCOpt     rpcOptConfigV1
	BLSKeys    blsConfigV1
	TxPool     txPoolConfigV1
	Pprof      pprofConfigV1
	Log        logConfigV1
	Sync       syncConfigV1
	Sys        *sysConfigV1        `toml:",omitempty"`
	Consensus  *consensusConfigV1  `toml:",omitempty"`
	Devnet     *devnetConfigV1     `toml:",omitempty"`
	Revert     *revertConfigV1     `toml:",omitempty"`
	Legacy     *legacyConfigV1     `toml:",omitempty"`
	Prometheus *prometheusConfigV1 `toml:",omitempty"`
}

type networkConfigV1 struct {
	NetworkType string
	BootNodes   []string

	LegacySyncing bool // if true, use LegacySyncingPeerProvider
	DNSZone       string
	DNSPort       int
}

type p2pConfigV1 struct {
	Port         int
	IP           string
	KeyFile      string
	DHTDataStore *string `toml:",omitempty"`
}

type generalConfigV1 struct {
	NodeType         string
	NoStaking        bool
	ShardID          int
	IsArchival       bool
	IsBeaconArchival bool
	IsOffline        bool
	DataDir          string
}

type consensusConfigV1 struct {
	MinPeers     int
	AggregateSig bool
}

type blsConfigV1 struct {
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

type txPoolConfigV1 struct {
	BlacklistFile string
}

type pprofConfigV1 struct {
	Enabled    bool
	ListenAddr string
}

type logConfigV1 struct {
	Folder     string
	FileName   string
	RotateSize int
	Verbosity  int
	Context    *logContextV1 `toml:",omitempty"`
}

type logContextV1 struct {
	IP   string
	Port int
}

type sysConfigV1 struct {
	NtpServer string
}

type httpConfigV1 struct {
	Enabled        bool
	IP             string
	Port           int
	RosettaEnabled bool
	RosettaPort    int
}

type wsConfigV1 struct {
	Enabled bool
	IP      string
	Port    int
}

type rpcOptConfigV1 struct {
	DebugEnabled bool // Enables PrivateDebugService APIs, including the EVM tracer
}

type devnetConfigV1 struct {
	NumShards   int
	ShardSize   int
	HmyNodeSize int
}

// TODO: make `revert` to a separate command
type revertConfigV1 struct {
	RevertBeacon bool
	RevertTo     int
	RevertBefore int
}

type legacyConfigV1 struct {
	WebHookConfig         *string `toml:",omitempty"`
	TPBroadcastInvalidTxn *bool   `toml:",omitempty"`
}

type prometheusConfigV1 struct {
	Enabled    bool
	IP         string
	Port       int
	EnablePush bool
	Gateway    string
}

type syncConfigV1 struct {
	// TODO: Remove this bool after stream sync is fully up.
	Downloader     bool // start the sync downloader client
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
