package config

import nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

const TOMLConfigVersion = "2.0.0"

const (
	DefNetworkType = nodeconfig.Mainnet
)

var DefaultConfig = HarmonyConfig{
	Version: TOMLConfigVersion,
	General: GeneralConfig{
		NodeType:         "validator",
		NoStaking:        false,
		ShardID:          -1,
		IsArchival:       false,
		IsBeaconArchival: false,
		IsOffline:        false,
		DataDir:          "./",
	},
	Network: GetDefaultNetworkConfig(DefNetworkType),
	P2P: P2PConfig{
		Port:    nodeconfig.DefaultP2PPort,
		IP:      nodeconfig.DefaultPublicListenIP,
		KeyFile: "./.hmykey",
	},
	HTTP: HTTPConfig{
		Enabled:        true,
		RosettaEnabled: false,
		IP:             "127.0.0.1",
		Port:           nodeconfig.DefaultRPCPort,
		RosettaPort:    nodeconfig.DefaultRosettaPort,
	},
	WS: WSConfig{
		Enabled: true,
		IP:      "127.0.0.1",
		Port:    nodeconfig.DefaultWSPort,
	},
	RPCOpt: RPCOptConfig{
		DebugEnabled: false,
	},
	BLSKeys: BLSConfig{
		KeyDir:   "./.hmy/blskeys",
		KeyFiles: []string{},
		MaxKeys:  10,

		PassEnabled:      true,
		PassSrcType:      BLSPassTypeAuto,
		PassFile:         "",
		SavePassphrase:   false,
		KMSEnabled:       false,
		KMSConfigSrcType: KMSConfigTypeShared,
		KMSConfigFile:    "",
	},
	TxPool: TxPoolConfig{
		BlacklistFile: "./.hmy/blacklist.txt",
	},
	Pprof: PprofConfig{
		Enabled:    false,
		ListenAddr: "127.0.0.1:6060",
	},
	Log: LogConfig{
		Folder:     "./latest",
		FileName:   "harmony.log",
		RotateSize: 100,
		Verbosity:  3,
	},
	DNSSync: GetDefaultDNSSyncConfig(DefNetworkType),
}

var DefaultSysConfig = SysConfig{
	NtpServer: "1.pool.ntp.org",
}

var DefaultDevnetConfig = DevnetConfig{
	NumShards:   2,
	ShardSize:   10,
	HmyNodeSize: 10,
}

var DefaultRevertConfig = RevertConfig{
	RevertBeacon: false,
	RevertBefore: 0,
	RevertTo:     0,
}

var DefaultLogContext = LogContext{
	IP:   "127.0.0.1",
	Port: 9000,
}

var DefaultConsensusConfig = ConsensusConfig{
	MinPeers:     6,
	AggregateSig: true,
}

var DefaultPrometheusConfig = PrometheusConfig{
	Enabled:    true,
	IP:         "0.0.0.0",
	Port:       9900,
	EnablePush: false,
	Gateway:    "https://gateway.harmony.one",
}

var (
	DefaultMainnetSyncConfig = SyncConfig{
		Downloader:     false,
		Concurrency:    6,
		MinPeers:       6,
		InitStreams:    8,
		DiscSoftLowCap: 8,
		DiscHardLowCap: 6,
		DiscHighCap:    128,
		DiscBatch:      8,
	}

	DefaultTestNetSyncConfig = SyncConfig{
		Downloader:     false,
		Concurrency:    4,
		MinPeers:       4,
		InitStreams:    4,
		DiscSoftLowCap: 4,
		DiscHardLowCap: 4,
		DiscHighCap:    1024,
		DiscBatch:      8,
	}

	defaultLocalNetSyncConfig = SyncConfig{
		Downloader:     false,
		Concurrency:    4,
		MinPeers:       4,
		InitStreams:    4,
		DiscSoftLowCap: 4,
		DiscHardLowCap: 4,
		DiscHighCap:    1024,
		DiscBatch:      8,
	}

	defaultElseSyncConfig = SyncConfig{
		Downloader:     true,
		Concurrency:    4,
		MinPeers:       4,
		InitStreams:    4,
		DiscSoftLowCap: 4,
		DiscHardLowCap: 4,
		DiscHighCap:    1024,
		DiscBatch:      8,
	}
)

const (
	DefaultBroadcastInvalidTx = true
)

func GetDefaultHmyConfigCopy(nt nodeconfig.NetworkType) HarmonyConfig {
	config := DefaultConfig

	config.Network = GetDefaultNetworkConfig(nt)
	if nt == nodeconfig.Devnet {
		devnet := GetDefaultDevnetConfigCopy()
		config.Devnet = &devnet
	}
	config.Sync = GetDefaultSyncConfig(nt)
	config.DNSSync = GetDefaultDNSSyncConfig(nt)

	return config
}

func GetDefaultSysConfigCopy() SysConfig {
	config := DefaultSysConfig
	return config
}

func GetDefaultDevnetConfigCopy() DevnetConfig {
	config := DefaultDevnetConfig
	return config
}

func GetDefaultRevertConfigCopy() RevertConfig {
	config := DefaultRevertConfig
	return config
}

func GetDefaultLogContextCopy() LogContext {
	config := DefaultLogContext
	return config
}

func GetDefaultConsensusConfigCopy() ConsensusConfig {
	config := DefaultConsensusConfig
	return config
}

func GetDefaultPrometheusConfigCopy() PrometheusConfig {
	config := DefaultPrometheusConfig
	return config
}

const (
	NodeTypeValidator = "validator"
	NodeTypeExplorer  = "explorer"
)

const (
	BLSPassTypeAuto   = "auto"
	BLSPassTypeFile   = "file"
	BLSPassTypePrompt = "prompt"

	KMSConfigTypeShared = "shared"
	KMSConfigTypePrompt = "prompt"
	KMSConfigTypeFile   = "file"
)
