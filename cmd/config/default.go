package config

import (
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/hmy"
	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

const tomlConfigVersion = "2.6.2"

const (
	defNetworkType = nodeconfig.Mainnet
)

var defaultConfig = harmonyconfig.HarmonyConfig{
	Version: tomlConfigVersion,
	General: harmonyconfig.GeneralConfig{
		NodeType:         "validator",
		NoStaking:        false,
		ShardID:          -1,
		IsArchival:       false,
		IsBeaconArchival: false,
		IsOffline:        false,
		DataDir:          "./",
		TraceEnable:      false,
	},
	Network: GetDefaultNetworkConfig(defNetworkType),
	P2P: harmonyconfig.P2pConfig{
		Port:                     nodeconfig.DefaultP2PPort,
		IP:                       nodeconfig.DefaultPublicListenIP,
		KeyFile:                  "./.hmykey",
		DiscConcurrency:          nodeconfig.DefaultP2PConcurrency,
		MaxConnsPerIP:            nodeconfig.DefaultMaxConnPerIP,
		DisablePrivateIPScan:     false,
		MaxPeers:                 nodeconfig.DefaultMaxPeers,
		ConnManagerLowWatermark:  nodeconfig.DefaultConnManagerLowWatermark,
		ConnManagerHighWatermark: nodeconfig.DefaultConnManagerHighWatermark,
		WaitForEachPeerToConnect: nodeconfig.DefaultWaitForEachPeerToConnect,
		NoTransportSecurity:      nodeconfig.DefaultNoTransportSecurity,
		NAT:                      nodeconfig.DefaultNAT,
		UserAgent:                nodeconfig.DefaultUserAgent,
		DialTimeout:              nodeconfig.DefaultDialTimeout,
		Muxer:                    nodeconfig.DefaultMuxer,
		NoRelay:                  nodeconfig.DefaultNoRelay,
	},
	HTTP: harmonyconfig.HttpConfig{
		Enabled:        true,
		RosettaEnabled: false,
		IP:             "127.0.0.1",
		Port:           nodeconfig.DefaultRPCPort,
		AuthPort:       nodeconfig.DefaultAuthRPCPort,
		RosettaPort:    nodeconfig.DefaultRosettaPort,
		ReadTimeout:    nodeconfig.DefaultHTTPTimeoutRead,
		WriteTimeout:   nodeconfig.DefaultHTTPTimeoutWrite,
		IdleTimeout:    nodeconfig.DefaultHTTPTimeoutIdle,
	},
	WS: harmonyconfig.WsConfig{
		Enabled:  true,
		IP:       "127.0.0.1",
		Port:     nodeconfig.DefaultWSPort,
		AuthPort: nodeconfig.DefaultAuthWSPort,
	},
	RPCOpt: harmonyconfig.RpcOptConfig{
		DebugEnabled:       false,
		EthRPCsEnabled:     true,
		StakingRPCsEnabled: true,
		LegacyRPCsEnabled:  true,
		RpcFilterFile:      "./.hmy/rpc_filter.txt",
		RateLimterEnabled:  true,
		RequestsPerSecond:  nodeconfig.DefaultRPCRateLimit,
		EvmCallTimeout:     nodeconfig.DefaultEvmCallTimeout,
		PreimagesEnabled:   false,
	},
	BLSKeys: harmonyconfig.BlsConfig{
		KeyDir:   "./.hmy/blskeys",
		KeyFiles: []string{},
		MaxKeys:  10,

		PassEnabled:      true,
		PassSrcType:      blsPassTypeAuto,
		PassFile:         "",
		SavePassphrase:   false,
		KMSEnabled:       false,
		KMSConfigSrcType: kmsConfigTypeShared,
		KMSConfigFile:    "",
	},
	TxPool: harmonyconfig.TxPoolConfig{
		BlacklistFile:     "./.hmy/blacklist.txt",
		AllowedTxsFile:    "./.hmy/allowedtxs.txt",
		RosettaFixFile:    "",
		AccountSlots:      core.DefaultTxPoolConfig.AccountSlots,
		LocalAccountsFile: "./.hmy/locals.txt",
		GlobalSlots:       core.DefaultTxPoolConfig.GlobalSlots,
		AccountQueue:      core.DefaultTxPoolConfig.AccountQueue,
		GlobalQueue:       core.DefaultTxPoolConfig.GlobalQueue,
		Lifetime:          core.DefaultTxPoolConfig.Lifetime,
		PriceLimit:        harmonyconfig.PriceLimit(core.DefaultTxPoolConfig.PriceLimit),
		PriceBump:         core.DefaultTxPoolConfig.PriceBump,
	},
	Sync: GetDefaultSyncConfig(defNetworkType),
	Pprof: harmonyconfig.PprofConfig{
		Enabled:            false,
		ListenAddr:         "127.0.0.1:6060",
		Folder:             "./profiles",
		ProfileNames:       []string{},
		ProfileIntervals:   []int{600},
		ProfileDebugValues: []int{0},
	},
	Log: harmonyconfig.LogConfig{
		Console:      false,
		Folder:       "./latest",
		FileName:     "harmony.log",
		RotateSize:   100,
		RotateCount:  0,
		RotateMaxAge: 0,
		Verbosity:    3,
		VerbosePrints: harmonyconfig.LogVerbosePrints{
			Config: true,
		},
	},
	DNSSync: GetDefaultDNSSyncConfig(defNetworkType),
	ShardData: harmonyconfig.ShardDataConfig{
		EnableShardData: false,
		DiskCount:       8,
		ShardCount:      4,
		CacheTime:       10,
		CacheSize:       512,
	},
	GPO: harmonyconfig.GasPriceOracleConfig{
		Blocks:            hmy.DefaultGPOConfig.Blocks,
		Transactions:      hmy.DefaultGPOConfig.Transactions,
		Percentile:        hmy.DefaultGPOConfig.Percentile,
		DefaultPrice:      hmy.DefaultGPOConfig.DefaultPrice,
		MaxPrice:          hmy.DefaultGPOConfig.MaxPrice,
		LowUsageThreshold: hmy.DefaultGPOConfig.LowUsageThreshold,
		BlockGasLimit:     hmy.DefaultGPOConfig.BlockGasLimit,
	},
	Cache: GetDefaultCacheConfig(defNetworkType),
}

var defaultSysConfig = harmonyconfig.SysConfig{
	NtpServer: "1.pool.ntp.org,0.beevik-ntp.pool.ntp.org",
}

var defaultDevnetConfig = harmonyconfig.DevnetConfig{
	NumShards:   2,
	ShardSize:   10,
	HmyNodeSize: 10,
	SlotsLimit:  0, // 0 means no limit
}

var defaultRevertConfig = harmonyconfig.RevertConfig{
	RevertBeacon: false,
	RevertBefore: 0,
	RevertTo:     0,
}

var defaultPreimageConfig = harmonyconfig.PreimageConfig{
	ImportFrom:    "",
	ExportTo:      "",
	GenerateStart: 0,
	GenerateEnd:   0,
}

var defaultLogContext = harmonyconfig.LogContext{
	IP:   "127.0.0.1",
	Port: 9000,
}

var defaultConsensusConfig = harmonyconfig.ConsensusConfig{
	MinPeers:     6,
	AggregateSig: true,
}

var defaultPrometheusConfig = harmonyconfig.PrometheusConfig{
	Enabled:    true,
	IP:         "0.0.0.0",
	Port:       9900,
	EnablePush: false,
	Gateway:    "https://gateway.harmony.one",
}

var defaultStagedSyncConfig = harmonyconfig.StagedSyncConfig{
	TurboMode:              true,
	DoubleCheckBlockHashes: false,
	MaxBlocksPerSyncCycle:  512,   // sync new blocks in each cycle, if set to zero means all blocks in one full cycle
	MaxBackgroundBlocks:    512,   // max blocks to be downloaded at background process in turbo mode
	InsertChainBatchSize:   128,   // number of blocks to build a batch and insert to chain in staged sync
	VerifyAllSig:           false, // whether it should verify signatures for all blocks
	VerifyHeaderBatchSize:  100,   // batch size to verify block header before insert to chain
	MaxMemSyncCycleSize:    1024,  // max number of blocks to use a single transaction for staged sync
	UseMemDB:               true,  // it uses memory by default. set it to false to use disk
	LogProgress:            false, // log the full sync progress in console
	DebugMode:              false, // log every single process and error to help to debug the syncing (DebugMode is not accessible to the end user and is only an aid for development)
}

var (
	defaultMainnetSyncConfig = harmonyconfig.SyncConfig{
		Enabled:              false,
		SyncMode:             0,
		Downloader:           false,
		StagedSync:           false,
		StagedSyncCfg:        defaultStagedSyncConfig,
		Concurrency:          6,
		MinPeers:             6,
		InitStreams:          8,
		MaxAdvertiseWaitTime: 60, //minutes
		DiscSoftLowCap:       8,
		DiscHardLowCap:       6,
		DiscHighCap:          128,
		DiscBatch:            8,
	}

	defaultTestNetSyncConfig = harmonyconfig.SyncConfig{
		Enabled:              true,
		SyncMode:             0,
		Downloader:           false,
		StagedSync:           false,
		StagedSyncCfg:        defaultStagedSyncConfig,
		Concurrency:          2,
		MinPeers:             2,
		InitStreams:          2,
		MaxAdvertiseWaitTime: 5, //minutes
		DiscSoftLowCap:       2,
		DiscHardLowCap:       2,
		DiscHighCap:          1024,
		DiscBatch:            3,
	}

	defaultLocalNetSyncConfig = harmonyconfig.SyncConfig{
		Enabled:              true,
		SyncMode:             0,
		Downloader:           true,
		StagedSync:           true,
		StagedSyncCfg:        defaultStagedSyncConfig,
		Concurrency:          4,
		MinPeers:             4,
		InitStreams:          4,
		MaxAdvertiseWaitTime: 5, //minutes
		DiscSoftLowCap:       4,
		DiscHardLowCap:       4,
		DiscHighCap:          1024,
		DiscBatch:            8,
	}

	defaultPartnerSyncConfig = harmonyconfig.SyncConfig{
		Enabled:              false,
		SyncMode:             0,
		Downloader:           false,
		StagedSync:           false,
		StagedSyncCfg:        defaultStagedSyncConfig,
		Concurrency:          2,
		MinPeers:             2,
		InitStreams:          2,
		MaxAdvertiseWaitTime: 2, //minutes
		DiscSoftLowCap:       2,
		DiscHardLowCap:       2,
		DiscHighCap:          1024,
		DiscBatch:            4,
	}

	defaultElseSyncConfig = harmonyconfig.SyncConfig{
		Enabled:              false,
		SyncMode:             0,
		Downloader:           false,
		StagedSync:           false,
		StagedSyncCfg:        defaultStagedSyncConfig,
		Concurrency:          4,
		MinPeers:             4,
		InitStreams:          4,
		MaxAdvertiseWaitTime: 2, //minutes
		DiscSoftLowCap:       4,
		DiscHardLowCap:       4,
		DiscHighCap:          1024,
		DiscBatch:            8,
	}
)

var defaultCacheConfig = harmonyconfig.CacheConfig{
	Disabled:        false,
	TrieNodeLimit:   256,
	TriesInMemory:   128,
	TrieTimeLimit:   2 * time.Minute,
	SnapshotLimit:   0,
	SnapshotWait:    true,
	Preimages:       true,
	SnapshotNoBuild: false,
}

const (
	DefaultBroadcastInvalidTx = false
)

func GetDefaultConfigCopy() harmonyconfig.HarmonyConfig {
	config := defaultConfig
	return config
}

func GetDefaultHmyConfigCopy(nt nodeconfig.NetworkType) harmonyconfig.HarmonyConfig {
	config := defaultConfig

	config.Network = GetDefaultNetworkConfig(nt)
	if nt == nodeconfig.Devnet {
		devnet := GetDefaultDevnetConfigCopy()
		config.Devnet = &devnet
	}
	config.Sync = GetDefaultSyncConfig(nt)
	config.DNSSync = GetDefaultDNSSyncConfig(nt)
	config.Cache = GetDefaultCacheConfig(nt)

	return config
}

func GetDefaultSysConfigCopy() harmonyconfig.SysConfig {
	config := defaultSysConfig
	return config
}

func GetDefaultDevnetConfigCopy() harmonyconfig.DevnetConfig {
	config := defaultDevnetConfig
	return config
}

func GetDefaultRevertConfigCopy() harmonyconfig.RevertConfig {
	config := defaultRevertConfig
	return config
}

func GetDefaultPreimageConfigCopy() harmonyconfig.PreimageConfig {
	config := defaultPreimageConfig
	return config
}

func GetDefaultLogContextCopy() harmonyconfig.LogContext {
	config := defaultLogContext
	return config
}

func GetDefaultConsensusConfigCopy() harmonyconfig.ConsensusConfig {
	config := defaultConsensusConfig
	return config
}

func GetDefaultPrometheusConfigCopy() harmonyconfig.PrometheusConfig {
	config := defaultPrometheusConfig
	return config
}

func GetDefaultCacheConfigCopy() harmonyconfig.CacheConfig {
	config := defaultCacheConfig
	return config
}

const (
	NodeTypeValidator = "validator"
	NodeTypeExplorer  = "explorer"
)

const (
	blsPassTypeAuto   = "auto"
	blsPassTypeFile   = "file"
	blsPassTypePrompt = "prompt"

	kmsConfigTypeShared = "shared"
	kmsConfigTypePrompt = "prompt"
	kmsConfigTypeFile   = "file"

	legacyBLSPassTypeDefault = "default"
	legacyBLSPassTypeStdin   = "stdin"
	legacyBLSPassTypeDynamic = "no-prompt"
	legacyBLSPassTypePrompt  = "prompt"
	legacyBLSPassTypeStatic  = "file"
	legacyBLSPassTypeNone    = "none"

	legacyBLSKmsTypeDefault = "default"
	legacyBLSKmsTypePrompt  = "prompt"
	legacyBLSKmsTypeFile    = "file"
	legacyBLSKmsTypeNone    = "none"
)
