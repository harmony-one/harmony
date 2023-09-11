package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/api/service/legacysync"
	"github.com/harmony-one/harmony/internal/cli"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

var (
	generalFlags = []cli.Flag{
		nodeTypeFlag,
		noStakingFlag,
		shardIDFlag,
		isArchiveFlag,
		isBeaconArchiveFlag,
		isOfflineFlag,
		dataDirFlag,

		legacyNodeTypeFlag,
		legacyIsStakingFlag,
		legacyShardIDFlag,
		legacyIsArchiveFlag,
		legacyDataDirFlag,

		taraceFlag,
		triesInMemoryFlag,
	}

	dnsSyncFlags = []cli.Flag{
		dnsZoneFlag,
		dnsPortFlag,
		dnsServerPortFlag,
		dnsClientFlag,
		dnsServerFlag,

		syncLegacyClientFlag,
		syncLegacyServerFlag,
		legacyDNSZoneFlag,
		legacyDNSPortFlag,
		legacyDNSFlag,
	}

	networkFlags = []cli.Flag{
		networkTypeFlag,
		bootNodeFlag,
		legacyNetworkTypeFlag,
	}

	p2pFlags = []cli.Flag{
		p2pPortFlag,
		p2pIPFlag,
		p2pKeyFileFlag,
		p2pDHTDataStoreFlag,
		p2pDiscoveryConcurrencyFlag,
		legacyKeyFileFlag,
		p2pDisablePrivateIPScanFlag,
		maxConnPerIPFlag,
		maxPeersFlag,
		connManagerLowWatermarkFlag,
		connManagerHighWatermarkFlag,
	}

	httpFlags = []cli.Flag{
		httpEnabledFlag,
		httpRosettaEnabledFlag,
		httpIPFlag,
		httpPortFlag,
		httpAuthPortFlag,
		httpRosettaPortFlag,
		httpReadTimeoutFlag,
		httpWriteTimeoutFlag,
		httpIdleTimeoutFlag,
	}

	wsFlags = []cli.Flag{
		wsEnabledFlag,
		wsIPFlag,
		wsPortFlag,
		wsAuthPortFlag,
	}

	rpcOptFlags = []cli.Flag{
		rpcDebugEnabledFlag,
		rpcPreimagesEnabledFlag,
		rpcEthRPCsEnabledFlag,
		rpcStakingRPCsEnabledFlag,
		rpcLegacyRPCsEnabledFlag,
		rpcFilterFileFlag,
		rpcRateLimiterEnabledFlag,
		rpcRateLimitFlag,
		rpcEvmCallTimeoutFlag,
	}

	blsFlags = append(newBLSFlags, legacyBLSFlags...)

	newBLSFlags = []cli.Flag{
		blsDirFlag,
		blsKeyFilesFlag,
		maxBLSKeyFilesFlag,
		passEnabledFlag,
		passSrcTypeFlag,
		passSrcFileFlag,
		passSaveFlag,
		kmsEnabledFlag,
		kmsConfigSrcTypeFlag,
		kmsConfigFileFlag,
	}

	legacyBLSFlags = []cli.Flag{
		legacyBLSKeyFileFlag,
		legacyBLSFolderFlag,
		legacyBLSKeysPerNodeFlag,
		legacyBLSPassFlag,
		legacyBLSPersistPassFlag,
		legacyKMSConfigSourceFlag,
	}

	consensusFlags = append(consensusValidFlags, consensusInvalidFlags...)

	// consensusValidFlags are flags that are effective
	consensusValidFlags = []cli.Flag{
		consensusMinPeersFlag,
		consensusAggregateSigFlag,
		legacyConsensusMinPeersFlag,
	}

	// consensusInvalidFlags are flags that are no longer effective
	consensusInvalidFlags = []cli.Flag{
		legacyDelayCommitFlag,
		legacyBlockTimeFlag,
	}

	txPoolFlags = []cli.Flag{
		tpAccountSlotsFlag,
		tpGlobalSlotsFlag,
		tpAccountQueueFlag,
		tpGlobalQueueFlag,
		tpLifetimeFlag,
		rosettaFixFileFlag,
		tpBlacklistFileFlag,
		legacyTPBlacklistFileFlag,
		localAccountsFileFlag,
		allowedTxsFileFlag,
		tpPriceLimitFlag,
		tpPriceBumpFlag,
	}

	pprofFlags = []cli.Flag{
		pprofEnabledFlag,
		pprofListenAddrFlag,
		pprofFolderFlag,
		pprofProfileNamesFlag,
		pprofProfileIntervalFlag,
		pprofProfileDebugFlag,
	}

	logFlags = []cli.Flag{
		logFolderFlag,
		logRotateSizeFlag,
		logRotateCountFlag,
		logRotateMaxAgeFlag,
		logFileNameFlag,
		logContextIPFlag,
		logContextPortFlag,
		logVerbosityFlag,
		logVerbosePrintsFlag,
		legacyVerbosityFlag,
		logConsoleFlag,

		legacyLogFolderFlag,
		legacyLogRotateSizeFlag,
	}

	sysFlags = []cli.Flag{
		sysNtpServerFlag,
	}

	devnetFlags = append(newDevnetFlags, legacyDevnetFlags...)

	newDevnetFlags = []cli.Flag{
		devnetNumShardsFlag,
		devnetShardSizeFlag,
		devnetHmyNodeSizeFlag,
	}

	legacyDevnetFlags = []cli.Flag{
		legacyDevnetNumShardsFlag,
		legacyDevnetShardSizeFlag,
		legacyDevnetHmyNodeSizeFlag,
	}

	revertFlags = append(newRevertFlags, legacyRevertFlags...)

	newRevertFlags = []cli.Flag{
		revertBeaconFlag,
		revertToFlag,
		revertBeforeFlag,
	}

	preimageFlags = []cli.Flag{
		preimageImportFlag,
		preimageExportFlag,
		preimageGenerateStartFlag,
		preimageGenerateEndFlag,
	}

	legacyRevertFlags = []cli.Flag{
		legacyRevertBeaconFlag,
		legacyRevertBeforeFlag,
		legacyRevertToFlag,
	}

	// legacyMiscFlags are legacy flags that cannot be categorized to a single category.
	legacyMiscFlags = []cli.Flag{
		legacyPortFlag,
		legacyIPFlag,
		legacyPublicRPCFlag,
		legacyWebHookConfigFlag,
		legacyTPBroadcastInvalidTxFlag,
	}

	prometheusFlags = []cli.Flag{
		prometheusEnabledFlag,
		prometheusIPFlag,
		prometheusPortFlag,
		prometheusGatewayFlag,
		prometheusEnablePushFlag,
	}

	syncFlags = []cli.Flag{
		syncStreamEnabledFlag,
		syncDownloaderFlag,
		syncStagedSyncFlag,
		syncConcurrencyFlag,
		syncMinPeersFlag,
		syncInitStreamsFlag,
		syncDiscSoftLowFlag,
		syncDiscHardLowFlag,
		syncDiscHighFlag,
		syncDiscBatchFlag,
	}

	shardDataFlags = []cli.Flag{
		enableShardDataFlag,
		diskCountFlag,
		shardCountFlag,
		cacheTimeFlag,
		cacheSizeFlag,
	}

	gpoFlags = []cli.Flag{
		gpoBlocksFlag,
		gpoTransactionsFlag,
		gpoPercentileFlag,
		gpoDefaultPriceFlag,
		gpoMaxPriceFlag,
		gpoLowUsageThresholdFlag,
		gpoBlockGasLimitFlag,
	}

	metricsFlags = []cli.Flag{
		metricsETHFlag,
		metricsExpensiveETHFlag,
	}
)

var (
	nodeTypeFlag = cli.StringFlag{
		Name:     "run",
		Usage:    "run node type (validator, explorer)",
		DefValue: defaultConfig.General.NodeType,
	}
	// TODO: Can we rename the legacy to internal?
	noStakingFlag = cli.BoolFlag{
		Name:     "run.legacy",
		Usage:    "whether to run node in legacy mode",
		DefValue: defaultConfig.General.NoStaking,
	}
	shardIDFlag = cli.IntFlag{
		Name:     "run.shard",
		Usage:    "run node on the given shard ID (-1 automatically configured by BLS keys)",
		DefValue: defaultConfig.General.ShardID,
	}
	isArchiveFlag = cli.BoolFlag{
		Name:     "run.archive",
		Usage:    "run shard chain in archive mode",
		DefValue: defaultConfig.General.IsArchival,
	}
	isBeaconArchiveFlag = cli.BoolFlag{
		Name:     "run.beacon-archive",
		Usage:    "run beacon chain in archive mode",
		DefValue: defaultConfig.General.IsBeaconArchival,
	}
	isOfflineFlag = cli.BoolFlag{
		Name:     "run.offline",
		Usage:    "run node in offline mode",
		DefValue: defaultConfig.General.IsOffline,
	}
	isBackupFlag = cli.BoolFlag{
		Name:     "run.backup",
		Usage:    "run node in backup mode",
		DefValue: defaultConfig.General.IsBackup,
	}
	dataDirFlag = cli.StringFlag{
		Name:     "datadir",
		Usage:    "directory of chain database",
		DefValue: defaultConfig.General.DataDir,
	}
	legacyNodeTypeFlag = cli.StringFlag{
		Name:       "node_type",
		Usage:      "run node type (validator, explorer)",
		DefValue:   defaultConfig.General.NodeType,
		Deprecated: "use --run",
	}
	legacyIsStakingFlag = cli.BoolFlag{
		Name:       "staking",
		Usage:      "whether to run node in staking mode",
		DefValue:   !defaultConfig.General.NoStaking,
		Deprecated: "use --run.legacy to run in legacy mode",
	}
	legacyShardIDFlag = cli.IntFlag{
		Name:       "shard_id",
		Usage:      "the shard ID of this node",
		DefValue:   defaultConfig.General.ShardID,
		Deprecated: "use --run.shard",
	}
	legacyIsArchiveFlag = cli.BoolFlag{
		Name:       "is_archival",
		Usage:      "false will enable cached state pruning",
		DefValue:   defaultConfig.General.IsArchival,
		Deprecated: "use --run.archive",
	}
	legacyDataDirFlag = cli.StringFlag{
		Name:       "db_dir",
		Usage:      "blockchain database directory",
		DefValue:   defaultConfig.General.DataDir,
		Deprecated: "use --datadir",
	}

	taraceFlag = cli.BoolFlag{
		Name:     "tracing",
		Usage:    "indicates if full transaction tracing should be enabled",
		DefValue: defaultConfig.General.TraceEnable,
	}
	triesInMemoryFlag = cli.IntFlag{
		Name:     "blockchain.tries_in_memory",
		Usage:    "number of blocks from header stored in disk before exiting",
		DefValue: defaultConfig.General.TriesInMemory,
	}
)

func getRootFlags() []cli.Flag {
	var flags []cli.Flag

	flags = append(flags, versionFlag)
	flags = append(flags, configFlag)
	flags = append(flags, generalFlags...)
	flags = append(flags, networkFlags...)
	flags = append(flags, dnsSyncFlags...)
	flags = append(flags, p2pFlags...)
	flags = append(flags, httpFlags...)
	flags = append(flags, wsFlags...)
	flags = append(flags, rpcOptFlags...)
	flags = append(flags, blsFlags...)
	flags = append(flags, consensusFlags...)
	flags = append(flags, txPoolFlags...)
	flags = append(flags, pprofFlags...)
	flags = append(flags, logFlags...)
	flags = append(flags, sysFlags...)
	flags = append(flags, devnetFlags...)
	flags = append(flags, revertFlags...)
	flags = append(flags, preimageFlags...)
	flags = append(flags, legacyMiscFlags...)
	flags = append(flags, prometheusFlags...)
	flags = append(flags, syncFlags...)
	flags = append(flags, shardDataFlags...)
	flags = append(flags, gpoFlags...)
	flags = append(flags, metricsFlags...)

	return flags
}

func applyGeneralFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, nodeTypeFlag) {
		config.General.NodeType = cli.GetStringFlagValue(cmd, nodeTypeFlag)
	} else if cli.IsFlagChanged(cmd, legacyNodeTypeFlag) {
		config.General.NodeType = cli.GetStringFlagValue(cmd, legacyNodeTypeFlag)
	}

	if cli.IsFlagChanged(cmd, shardIDFlag) {
		config.General.ShardID = cli.GetIntFlagValue(cmd, shardIDFlag)
	} else if cli.IsFlagChanged(cmd, legacyShardIDFlag) {
		config.General.ShardID = cli.GetIntFlagValue(cmd, legacyShardIDFlag)
	}

	if cli.IsFlagChanged(cmd, noStakingFlag) {
		config.General.NoStaking = cli.GetBoolFlagValue(cmd, noStakingFlag)
	} else if cli.IsFlagChanged(cmd, legacyIsStakingFlag) {
		config.General.NoStaking = !cli.GetBoolFlagValue(cmd, legacyIsStakingFlag)
	}

	if cli.IsFlagChanged(cmd, isArchiveFlag) {
		config.General.IsArchival = cli.GetBoolFlagValue(cmd, isArchiveFlag)
	} else if cli.IsFlagChanged(cmd, legacyIsArchiveFlag) {
		config.General.IsArchival = cli.GetBoolFlagValue(cmd, legacyIsArchiveFlag)
	}

	if cli.IsFlagChanged(cmd, isBeaconArchiveFlag) {
		config.General.IsBeaconArchival = cli.GetBoolFlagValue(cmd, isBeaconArchiveFlag)
	}

	if cli.IsFlagChanged(cmd, dataDirFlag) {
		config.General.DataDir = cli.GetStringFlagValue(cmd, dataDirFlag)
	} else if cli.IsFlagChanged(cmd, legacyDataDirFlag) {
		config.General.DataDir = cli.GetStringFlagValue(cmd, legacyDataDirFlag)
	}

	if cli.IsFlagChanged(cmd, isOfflineFlag) {
		config.General.IsOffline = cli.GetBoolFlagValue(cmd, isOfflineFlag)
	}

	if cli.IsFlagChanged(cmd, taraceFlag) {
		config.General.TraceEnable = cli.GetBoolFlagValue(cmd, taraceFlag)
	}

	if cli.IsFlagChanged(cmd, isBackupFlag) {
		config.General.IsBackup = cli.GetBoolFlagValue(cmd, isBackupFlag)
	}

	if cli.IsFlagChanged(cmd, triesInMemoryFlag) {
		value := cli.GetIntFlagValue(cmd, triesInMemoryFlag)
		if value <= 2 {
			panic("Must provide number greater than 2 for General.TriesInMemory")
		}
		config.General.TriesInMemory = value
	}
}

// network flags
var (
	networkTypeFlag = cli.StringFlag{
		Name:      "network",
		Shorthand: "n",
		DefValue:  "mainnet",
		Usage:     "network to join (mainnet, testnet, pangaea, localnet, partner, stressnet, devnet)",
	}
	bootNodeFlag = cli.StringSliceFlag{
		Name:  "bootnodes",
		Usage: "a list of bootnode multiaddress (delimited by ,)",
	}
	legacyDNSZoneFlag = cli.StringFlag{
		Name:       "dns_zone",
		Usage:      "use peers from the zone for state syncing",
		Deprecated: "use --dns.zone",
	}
	legacyNetworkTypeFlag = cli.StringFlag{
		Name:       "network_type",
		Usage:      "network to join (mainnet, testnet, pangaea, localnet, partner, stressnet, devnet)",
		Deprecated: "use --network",
	}
)

// DNSSync flags
var (
	legacyDNSPortFlag = cli.IntFlag{
		Name:       "dns_port",
		Usage:      "port of dns node",
		Deprecated: "use --dns.port",
	}
	legacyDNSFlag = cli.BoolFlag{
		Name:       "dns",
		DefValue:   true,
		Hidden:     true,
		Usage:      "use dns for syncing",
		Deprecated: "only set to false to use self discovered peers for syncing",
	}
	dnsZoneFlag = cli.StringFlag{
		Name:  "dns.zone",
		Usage: "use customized peers from the zone for state syncing",
	}
	dnsPortFlag = cli.IntFlag{
		Name:     "dns.port",
		DefValue: nodeconfig.DefaultDNSPort,
		Usage:    "dns sync remote server port",
	}
	dnsServerPortFlag = cli.IntFlag{
		Name:     "dns.server-port",
		DefValue: nodeconfig.DefaultDNSPort,
		Usage:    "dns sync local server port",
	}
	syncLegacyClientFlag = cli.BoolFlag{
		Name:       "sync.legacy.client",
		Usage:      "Enable the legacy centralized sync service for block synchronization",
		Hidden:     true,
		DefValue:   false,
		Deprecated: "use dns.client instead",
	}
	dnsClientFlag = cli.BoolFlag{
		Name:     "dns.client",
		Usage:    "Enable the legacy centralized sync service for block synchronization",
		Hidden:   true,
		DefValue: false,
	}
	syncLegacyServerFlag = cli.BoolFlag{
		Name:       "sync.legacy.server",
		Usage:      "Enable the gRPC sync server for backward compatibility",
		Hidden:     true,
		DefValue:   true,
		Deprecated: "use dns.server instead",
	}
	dnsServerFlag = cli.BoolFlag{
		Name:     "dns.server",
		Usage:    "Enable the gRPC sync server for backward compatibility",
		Hidden:   true,
		DefValue: true,
	}
)

func getNetworkType(cmd *cobra.Command) nodeconfig.NetworkType {
	var raw string

	if cli.IsFlagChanged(cmd, networkTypeFlag) {
		raw = cli.GetStringFlagValue(cmd, networkTypeFlag)
	} else if cli.IsFlagChanged(cmd, legacyNetworkTypeFlag) {
		raw = cli.GetStringFlagValue(cmd, legacyNetworkTypeFlag)
	} else {
		raw = defaultConfig.Network.NetworkType
	}
	return parseNetworkType(raw)
}

func applyDNSSyncFlags(cmd *cobra.Command, cfg *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, dnsZoneFlag) {
		cfg.DNSSync.Zone = cli.GetStringFlagValue(cmd, dnsZoneFlag)
	} else if cli.IsFlagChanged(cmd, legacyDNSZoneFlag) {
		cfg.DNSSync.Zone = cli.GetStringFlagValue(cmd, legacyDNSZoneFlag)
	} else if cli.IsFlagChanged(cmd, legacyDNSFlag) {
	}

	if cli.IsFlagChanged(cmd, dnsPortFlag) {
		cfg.DNSSync.Port = cli.GetIntFlagValue(cmd, dnsPortFlag)
	} else if cli.IsFlagChanged(cmd, legacyDNSPortFlag) {
		cfg.DNSSync.Port = cli.GetIntFlagValue(cmd, legacyDNSPortFlag)
	}

	if cli.IsFlagChanged(cmd, syncLegacyServerFlag) {
		cfg.DNSSync.Server = cli.GetBoolFlagValue(cmd, syncLegacyServerFlag)
	} else if cli.IsFlagChanged(cmd, dnsServerFlag) {
		cfg.DNSSync.Server = cli.GetBoolFlagValue(cmd, syncLegacyServerFlag)
	}

	if cli.IsFlagChanged(cmd, syncLegacyClientFlag) {
		cfg.DNSSync.Client = cli.GetBoolFlagValue(cmd, syncLegacyClientFlag)
	} else if cli.IsFlagChanged(cmd, dnsClientFlag) {
		cfg.DNSSync.Client = cli.GetBoolFlagValue(cmd, dnsClientFlag)
	}

	if cli.IsFlagChanged(cmd, dnsServerPortFlag) {
		cfg.DNSSync.ServerPort = cli.GetIntFlagValue(cmd, dnsServerPortFlag)
	}

}

func applyNetworkFlags(cmd *cobra.Command, cfg *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, bootNodeFlag) {
		cfg.Network.BootNodes = cli.GetStringSliceFlagValue(cmd, bootNodeFlag)
	}
}

// p2p flags
var (
	p2pPortFlag = cli.IntFlag{
		Name:     "p2p.port",
		Usage:    "port to listen for p2p protocols",
		DefValue: defaultConfig.P2P.Port,
	}
	p2pIPFlag = cli.StringFlag{
		Name:     "p2p.ip",
		Usage:    "ip to listen for p2p protocols",
		DefValue: defaultConfig.P2P.IP,
	}
	p2pKeyFileFlag = cli.StringFlag{
		Name:     "p2p.keyfile",
		Usage:    "the p2p key file of the harmony node",
		DefValue: defaultConfig.P2P.KeyFile,
	}
	p2pDHTDataStoreFlag = cli.StringFlag{
		Name:     "p2p.dht.datastore",
		Usage:    "the datastore file to persist the dht routing table",
		DefValue: "",
		Hidden:   true,
	}
	legacyKeyFileFlag = cli.StringFlag{
		Name:       "key",
		Usage:      "the p2p key file of the harmony node",
		DefValue:   defaultConfig.P2P.KeyFile,
		Deprecated: "use --p2p.keyfile",
	}
	p2pDiscoveryConcurrencyFlag = cli.IntFlag{
		Name:     "p2p.disc.concurrency",
		Usage:    "the pubsub's DHT discovery concurrency num (default with raw libp2p dht option)",
		DefValue: defaultConfig.P2P.DiscConcurrency,
	}
	p2pDisablePrivateIPScanFlag = cli.BoolFlag{
		Name:     "p2p.no-private-ip-scan",
		Usage:    "disable scanning of private ip4/6 addresses by DHT",
		DefValue: defaultConfig.P2P.DisablePrivateIPScan,
	}
	maxConnPerIPFlag = cli.IntFlag{
		Name:     "p2p.security.max-conn-per-ip",
		Usage:    "maximum number of connections allowed per remote node, 0 means no limit",
		DefValue: defaultConfig.P2P.MaxConnsPerIP,
	}
	maxPeersFlag = cli.IntFlag{
		Name:     "p2p.security.max-peers",
		Usage:    "maximum number of peers allowed, 0 means no limit",
		DefValue: defaultConfig.P2P.MaxConnsPerIP,
	}
	connManagerLowWatermarkFlag = cli.IntFlag{
		Name:     "p2p.connmgr-low",
		Usage:    "lowest number of connections that'll be maintained in connection manager. Set both high and low watermarks to zero to disable connection manager",
		DefValue: defaultConfig.P2P.ConnManagerLowWatermark,
	}
	connManagerHighWatermarkFlag = cli.IntFlag{
		Name:     "p2p.connmgr-high",
		Usage:    "highest number of connections that'll be maintained in connection manager. Set both high and low watermarks to zero to disable connection manager",
		DefValue: defaultConfig.P2P.ConnManagerHighWatermark,
	}
	waitForEachPeerToConnectFlag = cli.BoolFlag{
		Name:     "p2p.wait-for-connections",
		Usage:    "node waits for each single peer to connect and it doesn't add them to peers list after timeout",
		DefValue: defaultConfig.P2P.WaitForEachPeerToConnect,
	}
)

func applyP2PFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, p2pPortFlag) {
		config.P2P.Port = cli.GetIntFlagValue(cmd, p2pPortFlag)
	}
	if cli.IsFlagChanged(cmd, p2pIPFlag) {
		config.P2P.IP = cli.GetStringFlagValue(cmd, p2pIPFlag)
	}
	if config.General.IsOffline {
		config.P2P.IP = nodeconfig.DefaultLocalListenIP
	}

	if cli.IsFlagChanged(cmd, p2pKeyFileFlag) {
		config.P2P.KeyFile = cli.GetStringFlagValue(cmd, p2pKeyFileFlag)
	} else if cli.IsFlagChanged(cmd, legacyKeyFileFlag) {
		config.P2P.KeyFile = cli.GetStringFlagValue(cmd, legacyKeyFileFlag)
	}

	if cli.IsFlagChanged(cmd, p2pDHTDataStoreFlag) {
		ds := cli.GetStringFlagValue(cmd, p2pDHTDataStoreFlag)
		config.P2P.DHTDataStore = &ds
	}

	if cli.IsFlagChanged(cmd, p2pDiscoveryConcurrencyFlag) {
		config.P2P.DiscConcurrency = cli.GetIntFlagValue(cmd, p2pDiscoveryConcurrencyFlag)
	}

	if cli.IsFlagChanged(cmd, maxConnPerIPFlag) {
		config.P2P.MaxConnsPerIP = cli.GetIntFlagValue(cmd, maxConnPerIPFlag)
	}

	if cli.IsFlagChanged(cmd, maxPeersFlag) {
		config.P2P.MaxPeers = int64(cli.GetIntFlagValue(cmd, maxPeersFlag))
	}

	if cli.IsFlagChanged(cmd, waitForEachPeerToConnectFlag) {
		config.P2P.WaitForEachPeerToConnect = cli.GetBoolFlagValue(cmd, waitForEachPeerToConnectFlag)
	}

	if cli.IsFlagChanged(cmd, connManagerLowWatermarkFlag) {
		config.P2P.ConnManagerLowWatermark = cli.GetIntFlagValue(cmd, connManagerLowWatermarkFlag)
	}

	if cli.IsFlagChanged(cmd, connManagerHighWatermarkFlag) {
		config.P2P.ConnManagerHighWatermark = cli.GetIntFlagValue(cmd, connManagerHighWatermarkFlag)
	}

	if cli.IsFlagChanged(cmd, p2pDisablePrivateIPScanFlag) {
		config.P2P.DisablePrivateIPScan = cli.GetBoolFlagValue(cmd, p2pDisablePrivateIPScanFlag)
	}
}

// http flags
var (
	httpEnabledFlag = cli.BoolFlag{
		Name:     "http",
		Usage:    "enable HTTP / RPC requests",
		DefValue: defaultConfig.HTTP.Enabled,
	}
	httpIPFlag = cli.StringFlag{
		Name:     "http.ip",
		Usage:    "ip address to listen for RPC calls. Use 0.0.0.0 for public endpoint",
		DefValue: defaultConfig.HTTP.IP,
	}
	httpPortFlag = cli.IntFlag{
		Name:     "http.port",
		Usage:    "rpc port to listen for HTTP requests",
		DefValue: defaultConfig.HTTP.Port,
	}
	httpAuthPortFlag = cli.IntFlag{
		Name:     "http.auth-port",
		Usage:    "rpc port to listen for auth HTTP requests",
		DefValue: defaultConfig.HTTP.AuthPort,
	}
	httpRosettaEnabledFlag = cli.BoolFlag{
		Name:     "http.rosetta",
		Usage:    "enable HTTP / Rosetta requests",
		DefValue: defaultConfig.HTTP.RosettaEnabled,
	}
	httpRosettaPortFlag = cli.IntFlag{
		Name:     "http.rosetta.port",
		Usage:    "rosetta port to listen for HTTP requests",
		DefValue: defaultConfig.HTTP.RosettaPort,
	}
	httpReadTimeoutFlag = cli.StringFlag{
		Name:     "http.timeout.read",
		Usage:    "maximum duration to read the entire request, including the body",
		DefValue: defaultConfig.HTTP.ReadTimeout,
	}
	httpWriteTimeoutFlag = cli.StringFlag{
		Name:     "http.timeout.write",
		Usage:    "maximum duration before timing out writes of the response",
		DefValue: defaultConfig.HTTP.WriteTimeout,
	}
	httpIdleTimeoutFlag = cli.StringFlag{
		Name:     "http.timeout.idle",
		Usage:    "maximum amount of time to wait for the next request when keep-alives are enabled",
		DefValue: defaultConfig.HTTP.IdleTimeout,
	}
)

func applyHTTPFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	var isRPCSpecified, isRosettaSpecified bool

	if cli.IsFlagChanged(cmd, httpIPFlag) {
		config.HTTP.IP = cli.GetStringFlagValue(cmd, httpIPFlag)
		isRPCSpecified = true
	}

	if cli.IsFlagChanged(cmd, httpPortFlag) {
		config.HTTP.Port = cli.GetIntFlagValue(cmd, httpPortFlag)
		isRPCSpecified = true
	}

	if cli.IsFlagChanged(cmd, httpAuthPortFlag) {
		config.HTTP.AuthPort = cli.GetIntFlagValue(cmd, httpAuthPortFlag)
		isRPCSpecified = true
	}

	if cli.IsFlagChanged(cmd, httpRosettaPortFlag) {
		config.HTTP.RosettaPort = cli.GetIntFlagValue(cmd, httpRosettaPortFlag)
		isRosettaSpecified = true
	}

	if cli.IsFlagChanged(cmd, httpRosettaEnabledFlag) {
		config.HTTP.RosettaEnabled = cli.GetBoolFlagValue(cmd, httpRosettaEnabledFlag)
	} else if isRosettaSpecified {
		config.HTTP.RosettaEnabled = true
	}

	if cli.IsFlagChanged(cmd, httpEnabledFlag) {
		config.HTTP.Enabled = cli.GetBoolFlagValue(cmd, httpEnabledFlag)
	} else if isRPCSpecified {
		config.HTTP.Enabled = true
	}

	if cli.IsFlagChanged(cmd, httpReadTimeoutFlag) {
		config.HTTP.ReadTimeout = cli.GetStringFlagValue(cmd, httpReadTimeoutFlag)
	}
	if cli.IsFlagChanged(cmd, httpWriteTimeoutFlag) {
		config.HTTP.WriteTimeout = cli.GetStringFlagValue(cmd, httpWriteTimeoutFlag)
	}
	if cli.IsFlagChanged(cmd, httpIdleTimeoutFlag) {
		config.HTTP.IdleTimeout = cli.GetStringFlagValue(cmd, httpIdleTimeoutFlag)
	}

}

// ws flags
var (
	wsEnabledFlag = cli.BoolFlag{
		Name:     "ws",
		Usage:    "enable websocket endpoint",
		DefValue: defaultConfig.WS.Enabled,
	}
	wsIPFlag = cli.StringFlag{
		Name:     "ws.ip",
		Usage:    "ip endpoint for websocket. Use 0.0.0.0 for public endpoint",
		DefValue: defaultConfig.WS.IP,
	}
	wsPortFlag = cli.IntFlag{
		Name:     "ws.port",
		Usage:    "port for websocket endpoint",
		DefValue: defaultConfig.WS.Port,
	}
	wsAuthPortFlag = cli.IntFlag{
		Name:     "ws.auth-port",
		Usage:    "port for websocket auth endpoint",
		DefValue: defaultConfig.WS.AuthPort,
	}
)

func applyWSFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, wsEnabledFlag) {
		config.WS.Enabled = cli.GetBoolFlagValue(cmd, wsEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, wsIPFlag) {
		config.WS.IP = cli.GetStringFlagValue(cmd, wsIPFlag)
	}
	if cli.IsFlagChanged(cmd, wsPortFlag) {
		config.WS.Port = cli.GetIntFlagValue(cmd, wsPortFlag)
	}
	if cli.IsFlagChanged(cmd, wsAuthPortFlag) {
		config.WS.AuthPort = cli.GetIntFlagValue(cmd, wsAuthPortFlag)
	}
}

// rpc opt flags
var (
	rpcDebugEnabledFlag = cli.BoolFlag{
		Name:     "rpc.debug",
		Usage:    "enable private debug apis",
		DefValue: defaultConfig.RPCOpt.DebugEnabled,
		Hidden:   true,
	}
	rpcPreimagesEnabledFlag = cli.BoolFlag{
		Name:     "rpc.preimages",
		Usage:    "enable preimages export api",
		DefValue: defaultConfig.RPCOpt.PreimagesEnabled,
		Hidden:   true, // not for end users
	}

	rpcEthRPCsEnabledFlag = cli.BoolFlag{
		Name:     "rpc.eth",
		Usage:    "enable eth apis",
		DefValue: defaultConfig.RPCOpt.EthRPCsEnabled,
		Hidden:   true,
	}

	rpcStakingRPCsEnabledFlag = cli.BoolFlag{
		Name:     "rpc.staking",
		Usage:    "enable staking apis",
		DefValue: defaultConfig.RPCOpt.StakingRPCsEnabled,
		Hidden:   true,
	}

	rpcLegacyRPCsEnabledFlag = cli.BoolFlag{
		Name:     "rpc.legacy",
		Usage:    "enable legacy apis",
		DefValue: defaultConfig.RPCOpt.LegacyRPCsEnabled,
		Hidden:   true,
	}

	rpcFilterFileFlag = cli.StringFlag{
		Name:     "rpc.filterspath",
		Usage:    "toml file path for method exposure filters",
		DefValue: defaultConfig.RPCOpt.RpcFilterFile,
		Hidden:   true,
	}

	rpcRateLimiterEnabledFlag = cli.BoolFlag{
		Name:     "rpc.ratelimiter",
		Usage:    "enable rate limiter for RPCs",
		DefValue: defaultConfig.RPCOpt.RateLimterEnabled,
	}

	rpcRateLimitFlag = cli.IntFlag{
		Name:     "rpc.ratelimit",
		Usage:    "the number of requests per second for RPCs",
		DefValue: defaultConfig.RPCOpt.RequestsPerSecond,
	}

	rpcEvmCallTimeoutFlag = cli.StringFlag{
		Name:     "rpc.evm-call-timeout",
		Usage:    "timeout for evm execution (eth_call); 0 means infinite timeout",
		DefValue: defaultConfig.RPCOpt.EvmCallTimeout,
	}
)

func applyRPCOptFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, rpcDebugEnabledFlag) {
		config.RPCOpt.DebugEnabled = cli.GetBoolFlagValue(cmd, rpcDebugEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, rpcPreimagesEnabledFlag) {
		config.RPCOpt.PreimagesEnabled = cli.GetBoolFlagValue(cmd, rpcPreimagesEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, rpcEthRPCsEnabledFlag) {
		config.RPCOpt.EthRPCsEnabled = cli.GetBoolFlagValue(cmd, rpcEthRPCsEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, rpcStakingRPCsEnabledFlag) {
		config.RPCOpt.StakingRPCsEnabled = cli.GetBoolFlagValue(cmd, rpcStakingRPCsEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, rpcLegacyRPCsEnabledFlag) {
		config.RPCOpt.LegacyRPCsEnabled = cli.GetBoolFlagValue(cmd, rpcLegacyRPCsEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, rpcFilterFileFlag) {
		config.RPCOpt.RpcFilterFile = cli.GetStringFlagValue(cmd, rpcFilterFileFlag)
	}
	if cli.IsFlagChanged(cmd, rpcRateLimiterEnabledFlag) {
		config.RPCOpt.RateLimterEnabled = cli.GetBoolFlagValue(cmd, rpcRateLimiterEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, rpcRateLimitFlag) {
		config.RPCOpt.RequestsPerSecond = cli.GetIntFlagValue(cmd, rpcRateLimitFlag)
	}
	if cli.IsFlagChanged(cmd, rpcEvmCallTimeoutFlag) {
		config.RPCOpt.EvmCallTimeout = cli.GetStringFlagValue(cmd, rpcEvmCallTimeoutFlag)
	}
}

// bls flags
var (
	blsDirFlag = cli.StringFlag{
		Name:     "bls.dir",
		Usage:    "directory for BLS keys",
		DefValue: defaultConfig.BLSKeys.KeyDir,
	}
	blsKeyFilesFlag = cli.StringSliceFlag{
		Name:     "bls.keys",
		Usage:    "a list of BLS key files (separated by ,)",
		DefValue: defaultConfig.BLSKeys.KeyFiles,
	}
	// TODO: shall we move this to a hard coded parameter?
	maxBLSKeyFilesFlag = cli.IntFlag{
		Name:     "bls.maxkeys",
		Usage:    "maximum number of BLS keys for a node",
		DefValue: defaultConfig.BLSKeys.MaxKeys,
		Hidden:   true,
	}
	passEnabledFlag = cli.BoolFlag{
		Name:     "bls.pass",
		Usage:    "enable BLS key decryption with passphrase",
		DefValue: defaultConfig.BLSKeys.PassEnabled,
	}
	passSrcTypeFlag = cli.StringFlag{
		Name:     "bls.pass.src",
		Usage:    "source for BLS passphrase (auto, file, prompt)",
		DefValue: defaultConfig.BLSKeys.PassSrcType,
	}
	passSrcFileFlag = cli.StringFlag{
		Name:     "bls.pass.file",
		Usage:    "the pass file used for BLS decryption. If specified, this pass file will be used for all BLS keys",
		DefValue: defaultConfig.BLSKeys.PassFile,
	}
	passSaveFlag = cli.BoolFlag{
		Name:     "bls.pass.save",
		Usage:    "after input the BLS passphrase from console, whether to persist the input passphrases in .pass file",
		DefValue: defaultConfig.BLSKeys.SavePassphrase,
	}
	kmsEnabledFlag = cli.BoolFlag{
		Name:     "bls.kms",
		Usage:    "enable BLS key decryption with AWS KMS service",
		DefValue: defaultConfig.BLSKeys.KMSEnabled,
	}
	kmsConfigSrcTypeFlag = cli.StringFlag{
		Name:     "bls.kms.src",
		Usage:    "the AWS config source (region and credentials) for KMS service (shared, prompt, file)",
		DefValue: defaultConfig.BLSKeys.KMSConfigSrcType,
	}
	kmsConfigFileFlag = cli.StringFlag{
		Name:     "bls.kms.config",
		Usage:    "json config file for KMS service (region and credentials)",
		DefValue: defaultConfig.BLSKeys.KMSConfigFile,
	}
	legacyBLSKeyFileFlag = cli.StringSliceFlag{
		Name:       "blskey_file",
		Usage:      "The encrypted file of bls serialized private key by passphrase.",
		DefValue:   defaultConfig.BLSKeys.KeyFiles,
		Deprecated: "use --bls.keys",
	}
	legacyBLSFolderFlag = cli.StringFlag{
		Name:       "blsfolder",
		Usage:      "The folder that stores the bls keys and corresponding passphrases; e.g. <blskey>.key and <blskey>.pass; all bls keys mapped to same shard",
		DefValue:   defaultConfig.BLSKeys.KeyDir,
		Deprecated: "use --bls.dir",
	}
	legacyBLSKeysPerNodeFlag = cli.IntFlag{
		Name:       "max_bls_keys_per_node",
		Usage:      "Maximum number of bls keys allowed per node",
		DefValue:   defaultConfig.BLSKeys.MaxKeys,
		Deprecated: "use --bls.maxkeys",
	}
	legacyBLSPassFlag = cli.StringFlag{
		Name:       "blspass",
		Usage:      "The source for bls passphrases. (default, stdin, no-prompt, prompt, file:$PASS_FILE, none)",
		DefValue:   "default",
		Deprecated: "use --bls.pass, --bls.pass.src, --bls.pass.file",
	}
	legacyBLSPersistPassFlag = cli.BoolFlag{
		Name:       "save-passphrase",
		Usage:      "Whether the prompt passphrase is saved after prompt.",
		DefValue:   defaultConfig.BLSKeys.SavePassphrase,
		Deprecated: "use --bls.pass.save",
	}
	legacyKMSConfigSourceFlag = cli.StringFlag{
		Name:       "aws-config-source",
		Usage:      "The source for aws config. (default, prompt, file:$CONFIG_FILE, none)",
		DefValue:   "default",
		Deprecated: "use --bls.kms, --bls.kms.src, --bls.kms.config",
	}
)

func applyBLSFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, blsDirFlag) {
		config.BLSKeys.KeyDir = cli.GetStringFlagValue(cmd, blsDirFlag)
	} else if cli.IsFlagChanged(cmd, legacyBLSFolderFlag) {
		config.BLSKeys.KeyDir = cli.GetStringFlagValue(cmd, legacyBLSFolderFlag)
	}

	if cli.IsFlagChanged(cmd, blsKeyFilesFlag) {
		config.BLSKeys.KeyFiles = cli.GetStringSliceFlagValue(cmd, blsKeyFilesFlag)
	} else if cli.IsFlagChanged(cmd, legacyBLSKeyFileFlag) {
		config.BLSKeys.KeyFiles = cli.GetStringSliceFlagValue(cmd, legacyBLSKeyFileFlag)
	}

	if cli.IsFlagChanged(cmd, maxBLSKeyFilesFlag) {
		config.BLSKeys.MaxKeys = cli.GetIntFlagValue(cmd, maxBLSKeyFilesFlag)
	} else if cli.IsFlagChanged(cmd, legacyBLSKeysPerNodeFlag) {
		config.BLSKeys.MaxKeys = cli.GetIntFlagValue(cmd, legacyBLSKeysPerNodeFlag)
	}

	if cli.HasFlagsChanged(cmd, newBLSFlags) {
		applyBLSPassFlags(cmd, config)
		applyKMSFlags(cmd, config)
	} else if cli.HasFlagsChanged(cmd, legacyBLSFlags) {
		applyLegacyBLSPassFlags(cmd, config)
		applyLegacyKMSFlags(cmd, config)
	}
}

func applyBLSPassFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	var passFileSpecified bool

	if cli.IsFlagChanged(cmd, passEnabledFlag) {
		config.BLSKeys.PassEnabled = cli.GetBoolFlagValue(cmd, passEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, passSrcFileFlag) {
		config.BLSKeys.PassFile = cli.GetStringFlagValue(cmd, passSrcFileFlag)
		passFileSpecified = true
	}
	if cli.IsFlagChanged(cmd, passSaveFlag) {
		config.BLSKeys.SavePassphrase = cli.GetBoolFlagValue(cmd, passSaveFlag)
	}
	if cli.IsFlagChanged(cmd, passSrcTypeFlag) {
		config.BLSKeys.PassSrcType = cli.GetStringFlagValue(cmd, passSrcTypeFlag)
	} else if passFileSpecified {
		config.BLSKeys.PassSrcType = blsPassTypeFile
	}
}

func applyKMSFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	var fileSpecified bool

	if cli.IsFlagChanged(cmd, kmsEnabledFlag) {
		config.BLSKeys.KMSEnabled = cli.GetBoolFlagValue(cmd, kmsEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, kmsConfigFileFlag) {
		config.BLSKeys.KMSConfigFile = cli.GetStringFlagValue(cmd, kmsConfigFileFlag)
		fileSpecified = true
	}
	if cli.IsFlagChanged(cmd, kmsConfigSrcTypeFlag) {
		config.BLSKeys.KMSConfigSrcType = cli.GetStringFlagValue(cmd, kmsConfigSrcTypeFlag)
	} else if fileSpecified {
		config.BLSKeys.KMSConfigSrcType = blsPassTypeFile
	}
}

func applyLegacyBLSPassFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, legacyBLSPassFlag) {
		val := cli.GetStringFlagValue(cmd, legacyBLSPassFlag)
		legacyApplyBLSPassVal(val, config)
	}
	if cli.IsFlagChanged(cmd, legacyBLSPersistPassFlag) {
		config.BLSKeys.SavePassphrase = cli.GetBoolFlagValue(cmd, legacyBLSPersistPassFlag)
	}
}

func applyLegacyKMSFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, legacyKMSConfigSourceFlag) {
		val := cli.GetStringFlagValue(cmd, legacyKMSConfigSourceFlag)
		legacyApplyKMSSourceVal(val, config)
	}
}

func legacyApplyBLSPassVal(src string, config *harmonyconfig.HarmonyConfig) {
	methodArgs := strings.SplitN(src, ":", 2)
	method := methodArgs[0]

	switch method {
	case legacyBLSPassTypeDefault, legacyBLSPassTypeStdin:
		config.BLSKeys.PassSrcType = blsPassTypeAuto
	case legacyBLSPassTypeStatic:
		config.BLSKeys.PassSrcType = blsPassTypeFile
		if len(methodArgs) >= 2 {
			config.BLSKeys.PassFile = methodArgs[1]
		}
	case legacyBLSPassTypeDynamic:
		config.BLSKeys.PassSrcType = blsPassTypePrompt
	case legacyBLSPassTypePrompt:
		config.BLSKeys.PassSrcType = blsPassTypePrompt
	case legacyBLSPassTypeNone:
		config.BLSKeys.PassEnabled = false
	}
}

func legacyApplyKMSSourceVal(src string, config *harmonyconfig.HarmonyConfig) {
	methodArgs := strings.SplitN(src, ":", 2)
	method := methodArgs[0]

	switch method {
	case legacyBLSKmsTypeDefault:
		config.BLSKeys.KMSConfigSrcType = kmsConfigTypeShared
	case legacyBLSKmsTypePrompt:
		config.BLSKeys.KMSConfigSrcType = kmsConfigTypePrompt
	case legacyBLSKmsTypeFile:
		config.BLSKeys.KMSConfigSrcType = kmsConfigTypeFile
		if len(methodArgs) >= 2 {
			config.BLSKeys.KMSConfigFile = methodArgs[1]
		}
	case legacyBLSKmsTypeNone:
		config.BLSKeys.KMSEnabled = false
	}
}

// consensus flags
var (
	consensusMinPeersFlag = cli.IntFlag{
		Name:     "consensus.min-peers",
		Usage:    "minimal number of peers in shard",
		DefValue: defaultConsensusConfig.MinPeers,
		Hidden:   true,
	}
	consensusAggregateSigFlag = cli.BoolFlag{
		Name:     "consensus.aggregate-sig",
		Usage:    "(multi-key) aggregate bls signatures before sending",
		DefValue: defaultConsensusConfig.AggregateSig,
	}
	legacyDelayCommitFlag = cli.StringFlag{
		Name:       "delay_commit",
		Usage:      "how long to delay sending commit messages in consensus, ex: 500ms, 1s",
		Deprecated: "flag delay_commit is no longer effective",
	}
	legacyBlockTimeFlag = cli.IntFlag{
		Name:       "block_period",
		Usage:      "how long in second the leader waits to propose a new block",
		DefValue:   5,
		Deprecated: "flag block_period is no longer effective",
	}
	legacyConsensusMinPeersFlag = cli.IntFlag{
		Name:     "min_peers",
		Usage:    "Minimal number of Peers in shard",
		DefValue: defaultConsensusConfig.MinPeers,
		Hidden:   true,
	}
)

func applyConsensusFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if config.Consensus == nil && cli.HasFlagsChanged(cmd, consensusValidFlags) {
		cfg := getDefaultConsensusConfigCopy()
		config.Consensus = &cfg
	}

	if cli.IsFlagChanged(cmd, consensusMinPeersFlag) {
		config.Consensus.MinPeers = cli.GetIntFlagValue(cmd, consensusMinPeersFlag)
	} else if cli.IsFlagChanged(cmd, legacyConsensusMinPeersFlag) {
		config.Consensus.MinPeers = cli.GetIntFlagValue(cmd, legacyConsensusMinPeersFlag)
	}

	if cli.IsFlagChanged(cmd, consensusAggregateSigFlag) {
		config.Consensus.AggregateSig = cli.GetBoolFlagValue(cmd, consensusAggregateSigFlag)
	}
}

// transaction pool flags
var (
	tpAccountSlotsFlag = cli.IntFlag{
		Name:     "txpool.accountslots",
		Usage:    "number of executable transaction slots guaranteed per account",
		DefValue: int(defaultConfig.TxPool.AccountSlots),
	}
	tpBlacklistFileFlag = cli.StringFlag{
		Name:     "txpool.blacklist",
		Usage:    "file of blacklisted wallet addresses",
		DefValue: defaultConfig.TxPool.BlacklistFile,
	}
	rosettaFixFileFlag = cli.StringFlag{
		Name:     "txpool.rosettafixfile",
		Usage:    "file of rosetta fix file",
		DefValue: defaultConfig.TxPool.RosettaFixFile,
	}
	legacyTPBlacklistFileFlag = cli.StringFlag{
		Name:       "blacklist",
		Usage:      "Path to newline delimited file of blacklisted wallet addresses",
		DefValue:   defaultConfig.TxPool.BlacklistFile,
		Deprecated: "use --txpool.blacklist",
	}
	localAccountsFileFlag = cli.StringFlag{
		Name:     "txpool.locals",
		Usage:    "file of local wallet addresses",
		DefValue: defaultConfig.TxPool.LocalAccountsFile,
	}
	allowedTxsFileFlag = cli.StringFlag{
		Name:     "txpool.allowedtxs",
		Usage:    "file of allowed transactions",
		DefValue: defaultConfig.TxPool.AllowedTxsFile,
	}
	tpGlobalSlotsFlag = cli.IntFlag{
		Name:     "txpool.globalslots",
		Usage:    "maximum global number of non-executable transactions in the pool",
		DefValue: int(defaultConfig.TxPool.GlobalSlots),
	}
	tpAccountQueueFlag = cli.IntFlag{
		Name:     "txpool.accountqueue",
		Usage:    "capacity of queued transactions for account in the pool",
		DefValue: int(defaultConfig.TxPool.AccountQueue),
	}
	tpGlobalQueueFlag = cli.IntFlag{
		Name:     "txpool.globalqueue",
		Usage:    "global capacity for queued transactions in the pool",
		DefValue: int(defaultConfig.TxPool.GlobalQueue),
	}
	tpLifetimeFlag = cli.StringFlag{
		Name:     "txpool.lifetime",
		Usage:    "maximum lifetime of transactions in the pool as a golang duration string",
		DefValue: defaultConfig.TxPool.Lifetime.String(),
	}
	tpPriceLimitFlag = cli.IntFlag{
		Name:     "txpool.pricelimit",
		Usage:    "minimum gas price to enforce for acceptance into the pool",
		DefValue: int(defaultConfig.TxPool.PriceLimit),
	}
	tpPriceBumpFlag = cli.IntFlag{
		Name:     "txpool.pricebump",
		Usage:    "minimum price bump to replace an already existing transaction (nonce)",
		DefValue: int(defaultConfig.TxPool.PriceLimit),
	}
)

func applyTxPoolFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, rosettaFixFileFlag) {
		config.TxPool.RosettaFixFile = cli.GetStringFlagValue(cmd, rosettaFixFileFlag)
	}
	if cli.IsFlagChanged(cmd, tpAccountSlotsFlag) {
		value := cli.GetIntFlagValue(cmd, tpAccountSlotsFlag) // int, so fits in uint64 when positive
		if value <= 0 {
			panic("Must provide positive for txpool.accountslots")
		}
		config.TxPool.AccountSlots = uint64(value)
	}
	if cli.IsFlagChanged(cmd, tpGlobalSlotsFlag) {
		value := cli.GetIntFlagValue(cmd, tpGlobalSlotsFlag)
		if value <= 0 {
			panic("Must provide positive value for txpool.globalslots")
		}
		config.TxPool.GlobalSlots = uint64(value)
	}
	if cli.IsFlagChanged(cmd, tpAccountQueueFlag) {
		value := cli.GetIntFlagValue(cmd, tpAccountQueueFlag)
		if value <= 0 {
			panic("Must provide positive value for txpool.accountqueue")
		}
		config.TxPool.AccountQueue = uint64(value)
	}
	if cli.IsFlagChanged(cmd, tpGlobalQueueFlag) {
		value := cli.GetIntFlagValue(cmd, tpGlobalQueueFlag)
		if value <= 0 {
			panic("Must provide positive value for txpool.globalqueue")
		}
		config.TxPool.GlobalQueue = uint64(value)
	}
	if cli.IsFlagChanged(cmd, tpBlacklistFileFlag) {
		config.TxPool.BlacklistFile = cli.GetStringFlagValue(cmd, tpBlacklistFileFlag)
	} else if cli.IsFlagChanged(cmd, legacyTPBlacklistFileFlag) {
		config.TxPool.BlacklistFile = cli.GetStringFlagValue(cmd, legacyTPBlacklistFileFlag)
	}
	if cli.IsFlagChanged(cmd, localAccountsFileFlag) {
		config.TxPool.LocalAccountsFile = cli.GetStringFlagValue(cmd, localAccountsFileFlag)
	}
	if cli.IsFlagChanged(cmd, allowedTxsFileFlag) {
		config.TxPool.AllowedTxsFile = cli.GetStringFlagValue(cmd, allowedTxsFileFlag)
	}
	if cli.IsFlagChanged(cmd, tpLifetimeFlag) {
		value, err := time.ParseDuration(cli.GetStringFlagValue(cmd, tpLifetimeFlag))
		if err != nil {
			panic(fmt.Sprintf("Invalid value for txpool.lifetime: %v", err))
		}
		config.TxPool.Lifetime = value
	}
	if cli.IsFlagChanged(cmd, tpPriceLimitFlag) {
		value := cli.GetIntFlagValue(cmd, tpPriceLimitFlag)
		if value <= 0 {
			panic("Must provide positive value for txpool.pricelimit")
		}
		config.TxPool.PriceLimit = harmonyconfig.PriceLimit(value)
	}
	if cli.IsFlagChanged(cmd, tpPriceBumpFlag) {
		value := cli.GetIntFlagValue(cmd, tpPriceBumpFlag)
		if value <= 0 {
			panic("Must provide positive value for txpool.pricebump")
		}
		config.TxPool.PriceBump = uint64(value)
	}
}

// pprof flags
var (
	pprofEnabledFlag = cli.BoolFlag{
		Name:     "pprof",
		Usage:    "enable pprof profiling",
		DefValue: defaultConfig.Pprof.Enabled,
	}
	pprofListenAddrFlag = cli.StringFlag{
		Name:     "pprof.addr",
		Usage:    "listen address for pprof",
		DefValue: defaultConfig.Pprof.ListenAddr,
	}
	pprofFolderFlag = cli.StringFlag{
		Name:     "pprof.folder",
		Usage:    "folder to put pprof profiles",
		DefValue: defaultConfig.Pprof.Folder,
		Hidden:   true,
	}
	pprofProfileNamesFlag = cli.StringSliceFlag{
		Name:     "pprof.profile.names",
		Usage:    "a list of pprof profile names (separated by ,) e.g. cpu,heap,goroutine",
		DefValue: defaultConfig.Pprof.ProfileNames,
	}
	pprofProfileIntervalFlag = cli.IntSliceFlag{
		Name:     "pprof.profile.intervals",
		Usage:    "a list of pprof profile interval integer values (separated by ,) e.g. 30 saves all profiles every 30 seconds or 0,10 saves the first profile on shutdown and the second profile every 10 seconds",
		DefValue: defaultConfig.Pprof.ProfileIntervals,
		Hidden:   true,
	}
	pprofProfileDebugFlag = cli.IntSliceFlag{
		Name:     "pprof.profile.debug",
		Usage:    "a list of pprof profile debug integer values (separated by ,) e.g. 0 writes the gzip-compressed protocol buffer and 1 writes the legacy text format. Predefined profiles may assign meaning to other debug values: https://golang.org/pkg/runtime/pprof/",
		DefValue: defaultConfig.Pprof.ProfileDebugValues,
		Hidden:   true,
	}
)

func applyPprofFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	var pprofSet bool
	if cli.IsFlagChanged(cmd, pprofListenAddrFlag) {
		config.Pprof.ListenAddr = cli.GetStringFlagValue(cmd, pprofListenAddrFlag)
		pprofSet = true
	}
	if cli.IsFlagChanged(cmd, pprofFolderFlag) {
		config.Pprof.Folder = cli.GetStringFlagValue(cmd, pprofFolderFlag)
		pprofSet = true
	}
	if cli.IsFlagChanged(cmd, pprofProfileNamesFlag) {
		config.Pprof.ProfileNames = cli.GetStringSliceFlagValue(cmd, pprofProfileNamesFlag)
		pprofSet = true
	}
	if cli.IsFlagChanged(cmd, pprofProfileIntervalFlag) {
		config.Pprof.ProfileIntervals = cli.GetIntSliceFlagValue(cmd, pprofProfileIntervalFlag)
		pprofSet = true
	}
	if cli.IsFlagChanged(cmd, pprofProfileDebugFlag) {
		config.Pprof.ProfileDebugValues = cli.GetIntSliceFlagValue(cmd, pprofProfileDebugFlag)
		pprofSet = true
	}
	if cli.IsFlagChanged(cmd, pprofEnabledFlag) {
		config.Pprof.Enabled = cli.GetBoolFlagValue(cmd, pprofEnabledFlag)
	} else if pprofSet {
		config.Pprof.Enabled = true
	}
}

// log flags
var (
	logFolderFlag = cli.StringFlag{
		Name:     "log.dir",
		Usage:    "directory path to put rotation logs",
		DefValue: defaultConfig.Log.Folder,
	}
	logRotateSizeFlag = cli.IntFlag{
		Name:     "log.max-size",
		Usage:    "rotation log size in megabytes",
		DefValue: defaultConfig.Log.RotateSize,
	}
	logRotateCountFlag = cli.IntFlag{
		Name:     "log.rotate-count",
		Usage:    "maximum number of old log files to retain",
		DefValue: defaultConfig.Log.RotateCount,
	}
	logRotateMaxAgeFlag = cli.IntFlag{
		Name:     "log.rotate-max-age",
		Usage:    "maximum number of days to retain old log files",
		DefValue: defaultConfig.Log.RotateMaxAge,
	}
	logFileNameFlag = cli.StringFlag{
		Name:     "log.name",
		Usage:    "log file name (e.g. harmony.log)",
		DefValue: defaultConfig.Log.FileName,
	}
	logVerbosityFlag = cli.IntFlag{
		Name:      "log.verb",
		Shorthand: "v",
		Usage:     "logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail",
		DefValue:  defaultConfig.Log.Verbosity,
	}
	logVerbosePrintsFlag = cli.StringSliceFlag{
		Name:     "log.verbose-prints",
		Usage:    "debugging feature. to print verbose internal objects as JSON in log file. available internal objects: config",
		DefValue: []string{"config"},
	}
	logConsoleFlag = cli.BoolFlag{
		Name:     "log.console",
		Usage:    "output log to console only",
		DefValue: defaultConfig.Log.Console,
	}
	// TODO: remove context (this shall not be in the log)
	logContextIPFlag = cli.StringFlag{
		Name:     "log.ctx.ip",
		Usage:    "log context ip",
		DefValue: defaultLogContext.IP,
		Hidden:   true,
	}
	logContextPortFlag = cli.IntFlag{
		Name:     "log.ctx.port",
		Usage:    "log context port",
		DefValue: defaultLogContext.Port,
		Hidden:   true,
	}
	legacyLogFolderFlag = cli.StringFlag{
		Name:       "log_folder",
		Usage:      "the folder collecting the logs of this execution",
		DefValue:   defaultConfig.Log.Folder,
		Deprecated: "use --log.path",
	}
	legacyLogRotateSizeFlag = cli.IntFlag{
		Name:       "log_max_size",
		Usage:      "the max size in megabytes of the log file before it gets rotated",
		DefValue:   defaultConfig.Log.RotateSize,
		Deprecated: "use --log.max-size",
	}
	legacyVerbosityFlag = cli.IntFlag{
		Name:       "verbosity",
		Usage:      "logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail",
		DefValue:   defaultConfig.Log.Verbosity,
		Deprecated: "use --log.verbosity",
	}
)

func applyLogFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, logFolderFlag) {
		config.Log.Folder = cli.GetStringFlagValue(cmd, logFolderFlag)
	} else if cli.IsFlagChanged(cmd, legacyLogFolderFlag) {
		config.Log.Folder = cli.GetStringFlagValue(cmd, legacyLogFolderFlag)
	}

	if cli.IsFlagChanged(cmd, logRotateSizeFlag) {
		config.Log.RotateSize = cli.GetIntFlagValue(cmd, logRotateSizeFlag)
	} else if cli.IsFlagChanged(cmd, legacyLogRotateSizeFlag) {
		config.Log.RotateSize = cli.GetIntFlagValue(cmd, legacyLogRotateSizeFlag)
	}

	if cli.IsFlagChanged(cmd, logRotateCountFlag) {
		config.Log.RotateCount = cli.GetIntFlagValue(cmd, logRotateCountFlag)
	}

	if cli.IsFlagChanged(cmd, logRotateMaxAgeFlag) {
		config.Log.RotateMaxAge = cli.GetIntFlagValue(cmd, logRotateMaxAgeFlag)
	}

	if cli.IsFlagChanged(cmd, logFileNameFlag) {
		config.Log.FileName = cli.GetStringFlagValue(cmd, logFileNameFlag)
	}

	if cli.IsFlagChanged(cmd, logVerbosityFlag) {
		config.Log.Verbosity = cli.GetIntFlagValue(cmd, logVerbosityFlag)
	} else if cli.IsFlagChanged(cmd, legacyVerbosityFlag) {
		config.Log.Verbosity = cli.GetIntFlagValue(cmd, legacyVerbosityFlag)
	}

	if cli.IsFlagChanged(cmd, logVerbosePrintsFlag) {
		verbosePrintsFlagSlice := cli.GetStringSliceFlagValue(cmd, logVerbosePrintsFlag)
		config.Log.VerbosePrints = harmonyconfig.FlagSliceToLogVerbosePrints(verbosePrintsFlagSlice)
	}

	if cli.IsFlagChanged(cmd, logConsoleFlag) {
		config.Log.Console = cli.GetBoolFlagValue(cmd, logConsoleFlag)
	}

	if cli.HasFlagsChanged(cmd, []cli.Flag{logContextIPFlag, logContextPortFlag}) {
		ctx := getDefaultLogContextCopy()
		config.Log.Context = &ctx

		if cli.IsFlagChanged(cmd, logContextIPFlag) {
			config.Log.Context.IP = cli.GetStringFlagValue(cmd, logContextIPFlag)
		}
		if cli.IsFlagChanged(cmd, logContextPortFlag) {
			config.Log.Context.Port = cli.GetIntFlagValue(cmd, logContextPortFlag)
		}
	}
}

var (
	sysNtpServerFlag = cli.StringFlag{
		Name:     "sys.ntp",
		Usage:    "the ntp server",
		DefValue: defaultSysConfig.NtpServer,
		Hidden:   true,
	}
)

func applySysFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.HasFlagsChanged(cmd, sysFlags) || config.Sys == nil {
		cfg := getDefaultSysConfigCopy()
		config.Sys = &cfg
	}

	if cli.IsFlagChanged(cmd, sysNtpServerFlag) {
		config.Sys.NtpServer = cli.GetStringFlagValue(cmd, sysNtpServerFlag)
	}
}

var (
	devnetNumShardsFlag = cli.IntFlag{
		Name:     "devnet.num-shard",
		Usage:    "number of shards for devnet",
		DefValue: defaultDevnetConfig.NumShards,
		Hidden:   true,
	}
	devnetShardSizeFlag = cli.IntFlag{
		Name:     "devnet.shard-size",
		Usage:    "number of nodes per shard for devnet",
		DefValue: defaultDevnetConfig.ShardSize,
		Hidden:   true,
	}
	devnetHmyNodeSizeFlag = cli.IntFlag{
		Name:     "devnet.hmy-node-size",
		Usage:    "number of Harmony-operated nodes per shard for devnet (negative means equal to --devnet.shard-size)",
		DefValue: defaultDevnetConfig.HmyNodeSize,
		Hidden:   true,
	}
	legacyDevnetNumShardsFlag = cli.IntFlag{
		Name:       "dn_num_shards",
		Usage:      "number of shards for -network_type=devnet",
		DefValue:   defaultDevnetConfig.NumShards,
		Deprecated: "use --devnet.num-shard",
	}
	legacyDevnetShardSizeFlag = cli.IntFlag{
		Name:       "dn_shard_size",
		Usage:      "number of nodes per shard for -network_type=devnet",
		DefValue:   defaultDevnetConfig.ShardSize,
		Deprecated: "use --devnet.shard-size",
	}
	legacyDevnetHmyNodeSizeFlag = cli.IntFlag{
		Name:       "dn_hmy_size",
		Usage:      "number of Harmony-operated nodes per shard for -network_type=devnet; negative means equal to -dn_shard_size",
		DefValue:   defaultDevnetConfig.HmyNodeSize,
		Deprecated: "use --devnet.hmy-node-size",
	}
)

func applyDevnetFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.HasFlagsChanged(cmd, devnetFlags) && config.Devnet == nil {
		cfg := getDefaultDevnetConfigCopy()
		config.Devnet = &cfg
	}

	if cli.HasFlagsChanged(cmd, newDevnetFlags) {
		if cli.IsFlagChanged(cmd, devnetNumShardsFlag) {
			config.Devnet.NumShards = cli.GetIntFlagValue(cmd, devnetNumShardsFlag)
		}
		if cli.IsFlagChanged(cmd, devnetShardSizeFlag) {
			config.Devnet.ShardSize = cli.GetIntFlagValue(cmd, devnetShardSizeFlag)
		}
		if cli.IsFlagChanged(cmd, devnetHmyNodeSizeFlag) {
			config.Devnet.HmyNodeSize = cli.GetIntFlagValue(cmd, devnetHmyNodeSizeFlag)
		}
	}
	if cli.HasFlagsChanged(cmd, legacyDevnetFlags) {
		if cli.IsFlagChanged(cmd, legacyDevnetNumShardsFlag) {
			config.Devnet.NumShards = cli.GetIntFlagValue(cmd, legacyDevnetNumShardsFlag)
		}
		if cli.IsFlagChanged(cmd, legacyDevnetShardSizeFlag) {
			config.Devnet.ShardSize = cli.GetIntFlagValue(cmd, legacyDevnetShardSizeFlag)
		}
		if cli.IsFlagChanged(cmd, legacyDevnetHmyNodeSizeFlag) {
			config.Devnet.HmyNodeSize = cli.GetIntFlagValue(cmd, legacyDevnetHmyNodeSizeFlag)
		}
	}

	if config.Devnet != nil {
		if config.Devnet.HmyNodeSize < 0 || config.Devnet.HmyNodeSize > config.Devnet.ShardSize {
			config.Devnet.HmyNodeSize = config.Devnet.ShardSize
		}
	}
}

var (
	revertBeaconFlag = cli.BoolFlag{
		Name:     "revert.beacon",
		Usage:    "revert the beacon chain",
		DefValue: defaultRevertConfig.RevertBeacon,
		Hidden:   true,
	}
	revertToFlag = cli.IntFlag{
		Name:     "revert.to",
		Usage:    "rollback all blocks until and including block number revert.to",
		DefValue: defaultRevertConfig.RevertTo,
		Hidden:   true,
	}
	revertBeforeFlag = cli.IntFlag{
		Name:     "revert.do-before",
		Usage:    "if the current block is less than revert.do-before, revert all blocks until (including) revert_to block",
		DefValue: defaultRevertConfig.RevertBefore,
		Hidden:   true,
	}
	legacyRevertBeaconFlag = cli.BoolFlag{
		Name:       "revert_beacon",
		Usage:      "whether to revert beacon chain or the chain this node is assigned to",
		DefValue:   defaultRevertConfig.RevertBeacon,
		Deprecated: "use --revert.beacon",
	}
	legacyRevertBeforeFlag = cli.IntFlag{
		Name:       "do_revert_before",
		Usage:      "If the current block is less than do_revert_before, revert all blocks until (including) revert_to block",
		DefValue:   defaultRevertConfig.RevertBefore,
		Deprecated: "use --revert.do-before",
	}
	legacyRevertToFlag = cli.IntFlag{
		Name:       "revert_to",
		Usage:      "The revert will rollback all blocks until and including block number revert_to",
		DefValue:   defaultRevertConfig.RevertTo,
		Deprecated: "use --revert.to",
	}
)

func applyRevertFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.HasFlagsChanged(cmd, revertFlags) {
		cfg := getDefaultRevertConfigCopy()
		config.Revert = &cfg
	}

	if cli.HasFlagsChanged(cmd, newRevertFlags) {
		if cli.IsFlagChanged(cmd, revertBeaconFlag) {
			config.Revert.RevertBeacon = cli.GetBoolFlagValue(cmd, revertBeaconFlag)
		}
		if cli.IsFlagChanged(cmd, revertBeforeFlag) {
			config.Revert.RevertBefore = cli.GetIntFlagValue(cmd, revertBeforeFlag)
		}
		if cli.IsFlagChanged(cmd, revertToFlag) {
			config.Revert.RevertTo = cli.GetIntFlagValue(cmd, revertToFlag)
		}
	}

	if cli.HasFlagsChanged(cmd, legacyRevertFlags) {
		if cli.IsFlagChanged(cmd, legacyRevertBeaconFlag) {
			config.Revert.RevertBeacon = cli.GetBoolFlagValue(cmd, legacyRevertBeaconFlag)
		}
		if cli.IsFlagChanged(cmd, legacyRevertBeforeFlag) {
			config.Revert.RevertBefore = cli.GetIntFlagValue(cmd, legacyRevertBeforeFlag)
		}
		if cli.IsFlagChanged(cmd, legacyRevertToFlag) {
			config.Revert.RevertTo = cli.GetIntFlagValue(cmd, legacyRevertToFlag)
		}
	}
}

var (
	preimageImportFlag = cli.StringFlag{
		Name:     "preimage.import",
		Usage:    "Import pre-images from CSV file",
		Hidden:   true,
		DefValue: defaultPreimageConfig.ImportFrom,
	}
	preimageExportFlag = cli.StringFlag{
		Name:     "preimage.export",
		Usage:    "Export pre-images to CSV file",
		Hidden:   true,
		DefValue: defaultPreimageConfig.ExportTo,
	}
	preimageGenerateStartFlag = cli.Uint64Flag{
		Name:     "preimage.start",
		Usage:    "The block number from which pre-images are to be generated",
		Hidden:   true,
		DefValue: defaultPreimageConfig.GenerateStart,
	}
	preimageGenerateEndFlag = cli.Uint64Flag{
		Name:     "preimage.end",
		Usage:    "The block number upto and including which pre-images are to be generated",
		Hidden:   true,
		DefValue: defaultPreimageConfig.GenerateEnd,
	}
)

func applyPreimageFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.HasFlagsChanged(cmd, preimageFlags) {
		cfg := getDefaultPreimageConfigCopy()
		config.Preimage = &cfg
	}
	if cli.IsFlagChanged(cmd, preimageImportFlag) {
		config.Preimage.ImportFrom = cli.GetStringFlagValue(cmd, preimageImportFlag)
	}
	if cli.IsFlagChanged(cmd, preimageExportFlag) {
		config.Preimage.ExportTo = cli.GetStringFlagValue(cmd, preimageExportFlag)
	}
	if cli.IsFlagChanged(cmd, preimageGenerateStartFlag) {
		config.Preimage.GenerateStart = cli.GetUint64FlagValue(cmd, preimageGenerateStartFlag)
	}
	if cli.IsFlagChanged(cmd, preimageGenerateEndFlag) {
		config.Preimage.GenerateEnd = cli.GetUint64FlagValue(cmd, preimageGenerateEndFlag)
	}
}

var (
	legacyPortFlag = cli.IntFlag{
		Name:       "port",
		Usage:      "port of the node",
		DefValue:   defaultConfig.P2P.Port,
		Deprecated: "Use --p2p.port, --http.port instead",
	}
	legacyIPFlag = cli.StringFlag{
		Name:       "ip",
		Usage:      "ip of the node",
		DefValue:   defaultConfig.HTTP.IP,
		Deprecated: "use --http.ip",
	}
	legacyPublicRPCFlag = cli.BoolFlag{
		Name:       "public_rpc",
		Usage:      "Enable Public HTTP Access (default: false)",
		DefValue:   defaultConfig.HTTP.Enabled,
		Deprecated: "use --http.ip and --ws.ip to specify the ip address to listen. Use 127.0.0.1 to listen local requests.",
	}
	legacyWebHookConfigFlag = cli.StringFlag{
		Name:     "webhook_yaml",
		Usage:    "path for yaml config reporting double signing",
		DefValue: "",
		Hidden:   true,
	}
	legacyTPBroadcastInvalidTxFlag = cli.BoolFlag{
		Name:       "broadcast_invalid_tx",
		Usage:      "broadcast invalid transactions to sync pool state",
		DefValue:   defaultBroadcastInvalidTx,
		Deprecated: "use --txpool.broadcast-invalid-tx",
	}
)

// Note: this function need to be called before parse other flags
func applyLegacyMiscFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, legacyPortFlag) {
		legacyPort := cli.GetIntFlagValue(cmd, legacyPortFlag)
		config.P2P.Port = legacyPort
		config.HTTP.Port = nodeconfig.GetRPCHTTPPortFromBase(legacyPort)
		config.HTTP.AuthPort = nodeconfig.GetRPCAuthHTTPPortFromBase(legacyPort)
		config.HTTP.RosettaPort = nodeconfig.GetRosettaHTTPPortFromBase(legacyPort)
		config.WS.Port = nodeconfig.GetWSPortFromBase(legacyPort)
		config.WS.AuthPort = nodeconfig.GetWSAuthPortFromBase(legacyPort)

		legPortStr := strconv.Itoa(legacyPort)
		syncPort, _ := strconv.Atoi(legacysync.GetSyncingPort(legPortStr))
		config.DNSSync.Port = syncPort
		config.DNSSync.ServerPort = syncPort
	}

	if cli.IsFlagChanged(cmd, legacyIPFlag) {
		// legacy IP is not used for listening port
		config.HTTP.Enabled = true
		config.WS.Enabled = true
	}

	if cli.IsFlagChanged(cmd, legacyPublicRPCFlag) {
		if !cli.GetBoolFlagValue(cmd, legacyPublicRPCFlag) {
			config.HTTP.IP = nodeconfig.DefaultLocalListenIP
			config.WS.IP = nodeconfig.DefaultLocalListenIP
		} else {
			config.HTTP.IP = nodeconfig.DefaultPublicListenIP
			config.WS.IP = nodeconfig.DefaultPublicListenIP
		}
	}

	if cli.HasFlagsChanged(cmd, []cli.Flag{legacyPortFlag, legacyIPFlag}) {
		logIP := cli.GetStringFlagValue(cmd, legacyIPFlag)
		logPort := cli.GetIntFlagValue(cmd, legacyPortFlag)
		config.Log.FileName = fmt.Sprintf("validator-%v-%v.log", logIP, logPort)

		logCtx := &harmonyconfig.LogContext{
			IP:   logIP,
			Port: logPort,
		}
		config.Log.Context = logCtx
	}

	if cli.HasFlagsChanged(cmd, []cli.Flag{legacyWebHookConfigFlag, legacyTPBroadcastInvalidTxFlag}) {
		config.Legacy = &harmonyconfig.LegacyConfig{}
		if cli.IsFlagChanged(cmd, legacyWebHookConfigFlag) {
			val := cli.GetStringFlagValue(cmd, legacyWebHookConfigFlag)
			config.Legacy.WebHookConfig = &val
		}
		if cli.IsFlagChanged(cmd, legacyTPBroadcastInvalidTxFlag) {
			val := cli.GetBoolFlagValue(cmd, legacyTPBroadcastInvalidTxFlag)
			config.Legacy.TPBroadcastInvalidTxn = &val
		}
	}

}

var (
	prometheusEnabledFlag = cli.BoolFlag{
		Name:     "prometheus",
		Usage:    "enable HTTP / Prometheus requests",
		DefValue: defaultPrometheusConfig.Enabled,
	}
	prometheusIPFlag = cli.StringFlag{
		Name:     "prometheus.ip",
		Usage:    "ip address to listen for prometheus service",
		DefValue: defaultPrometheusConfig.IP,
	}
	prometheusPortFlag = cli.IntFlag{
		Name:     "prometheus.port",
		Usage:    "prometheus port to listen for HTTP requests",
		DefValue: defaultPrometheusConfig.Port,
	}
	prometheusGatewayFlag = cli.StringFlag{
		Name:     "prometheus.pushgateway",
		Usage:    "prometheus pushgateway URL",
		DefValue: defaultPrometheusConfig.Gateway,
	}
	prometheusEnablePushFlag = cli.BoolFlag{
		Name:     "prometheus.push",
		Usage:    "enable prometheus pushgateway",
		DefValue: defaultPrometheusConfig.EnablePush,
	}
)

func applyPrometheusFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if config.Prometheus == nil {
		cfg := getDefaultPrometheusConfigCopy()
		config.Prometheus = &cfg
		// enable pushgateway for mainnet nodes by default
		if config.Network.NetworkType == "mainnet" {
			config.Prometheus.EnablePush = true
		}
	}

	if cli.IsFlagChanged(cmd, prometheusIPFlag) {
		config.Prometheus.IP = cli.GetStringFlagValue(cmd, prometheusIPFlag)
	}

	if cli.IsFlagChanged(cmd, prometheusPortFlag) {
		config.Prometheus.Port = cli.GetIntFlagValue(cmd, prometheusPortFlag)
	}

	if cli.IsFlagChanged(cmd, prometheusEnabledFlag) {
		config.Prometheus.Enabled = cli.GetBoolFlagValue(cmd, prometheusEnabledFlag)
	}

	if cli.IsFlagChanged(cmd, prometheusGatewayFlag) {
		config.Prometheus.Gateway = cli.GetStringFlagValue(cmd, prometheusGatewayFlag)
	}
	if cli.IsFlagChanged(cmd, prometheusEnablePushFlag) {
		config.Prometheus.EnablePush = cli.GetBoolFlagValue(cmd, prometheusEnablePushFlag)
	}
}

var (
	syncStreamEnabledFlag = cli.BoolFlag{
		Name:     "sync",
		Usage:    "Enable the stream sync protocol (experimental feature)",
		DefValue: false,
	}
	// TODO: Deprecate this flag, and always set to true after stream sync is fully up.
	syncDownloaderFlag = cli.BoolFlag{
		Name:     "sync.downloader",
		Usage:    "Enable the downloader module to sync through stream sync protocol",
		Hidden:   true,
		DefValue: false,
	}
	syncStagedSyncFlag = cli.BoolFlag{
		Name:     "sync.stagedsync",
		Usage:    "Enable the staged sync",
		Hidden:   false,
		DefValue: false,
	}
	syncConcurrencyFlag = cli.IntFlag{
		Name:   "sync.concurrency",
		Usage:  "Concurrency when doing p2p sync requests",
		Hidden: true,
	}
	syncMinPeersFlag = cli.IntFlag{
		Name:   "sync.min-peers",
		Usage:  "Minimum peers check for each shard-wise sync loop",
		Hidden: true,
	}
	syncInitStreamsFlag = cli.IntFlag{
		Name:   "sync.init-peers",
		Usage:  "Initial shard-wise number of peers to start syncing",
		Hidden: true,
	}
	syncMaxAdvertiseWaitTimeFlag = cli.IntFlag{
		Name:   "sync.max-advertise-wait-time",
		Usage:  "The max time duration between two advertises for each p2p peer to tell other nodes what protocols it supports",
		Hidden: true,
	}
	syncDiscSoftLowFlag = cli.IntFlag{
		Name:   "sync.disc.soft-low-cap",
		Usage:  "Soft low cap for sync stream management",
		Hidden: true,
	}
	syncDiscHardLowFlag = cli.IntFlag{
		Name:   "sync.disc.hard-low-cap",
		Usage:  "Hard low cap for sync stream management",
		Hidden: true,
	}
	syncDiscHighFlag = cli.IntFlag{
		Name:   "sync.disc.hi-cap",
		Usage:  "High cap for sync stream management",
		Hidden: true,
	}
	syncDiscBatchFlag = cli.IntFlag{
		Name:   "sync.disc.batch",
		Usage:  "batch size of the sync discovery",
		Hidden: true,
	}
)

// applySyncFlags apply the sync flags.
func applySyncFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, syncStreamEnabledFlag) {
		config.Sync.Enabled = cli.GetBoolFlagValue(cmd, syncStreamEnabledFlag)
	}

	if cli.IsFlagChanged(cmd, syncDownloaderFlag) {
		config.Sync.Downloader = cli.GetBoolFlagValue(cmd, syncDownloaderFlag)
	}

	if cli.IsFlagChanged(cmd, syncStagedSyncFlag) {
		config.Sync.StagedSync = cli.GetBoolFlagValue(cmd, syncStagedSyncFlag)
	}

	if cli.IsFlagChanged(cmd, syncConcurrencyFlag) {
		config.Sync.Concurrency = cli.GetIntFlagValue(cmd, syncConcurrencyFlag)
	}

	if cli.IsFlagChanged(cmd, syncMinPeersFlag) {
		config.Sync.MinPeers = cli.GetIntFlagValue(cmd, syncMinPeersFlag)
	}

	if cli.IsFlagChanged(cmd, syncInitStreamsFlag) {
		config.Sync.InitStreams = cli.GetIntFlagValue(cmd, syncInitStreamsFlag)
	}

	if cli.IsFlagChanged(cmd, syncMaxAdvertiseWaitTimeFlag) {
		config.Sync.MaxAdvertiseWaitTime = cli.GetIntFlagValue(cmd, syncMaxAdvertiseWaitTimeFlag)
	}

	if cli.IsFlagChanged(cmd, syncDiscSoftLowFlag) {
		config.Sync.DiscSoftLowCap = cli.GetIntFlagValue(cmd, syncDiscSoftLowFlag)
	}

	if cli.IsFlagChanged(cmd, syncDiscHardLowFlag) {
		config.Sync.DiscHardLowCap = cli.GetIntFlagValue(cmd, syncDiscHardLowFlag)
	}

	if cli.IsFlagChanged(cmd, syncDiscHighFlag) {
		config.Sync.DiscHighCap = cli.GetIntFlagValue(cmd, syncDiscHighFlag)
	}

	if cli.IsFlagChanged(cmd, syncDiscBatchFlag) {
		config.Sync.DiscBatch = cli.GetIntFlagValue(cmd, syncDiscBatchFlag)
	}
}

// shard data flags
var (
	enableShardDataFlag = cli.BoolFlag{
		Name:     "sharddata.enable",
		Usage:    "whether use multi-database mode of levelDB",
		DefValue: defaultConfig.ShardData.EnableShardData,
	}
	diskCountFlag = cli.IntFlag{
		Name:     "sharddata.disk_count",
		Usage:    "the count of disks you want to storage block data",
		DefValue: defaultConfig.ShardData.DiskCount,
	}
	shardCountFlag = cli.IntFlag{
		Name:     "sharddata.shard_count",
		Usage:    "the count of shards you want to split in each disk",
		DefValue: defaultConfig.ShardData.ShardCount,
	}
	cacheTimeFlag = cli.IntFlag{
		Name:     "sharddata.cache_time",
		Usage:    "local cache save time (minute)",
		DefValue: defaultConfig.ShardData.CacheTime,
	}
	cacheSizeFlag = cli.IntFlag{
		Name:     "sharddata.cache_size",
		Usage:    "local cache storage size (MB)",
		DefValue: defaultConfig.ShardData.CacheSize,
	}
)

// gas price oracle flags
var (
	gpoBlocksFlag = cli.IntFlag{
		Name:     "gpo.blocks",
		Usage:    "Number of recent blocks to check for gas prices",
		DefValue: defaultConfig.GPO.Blocks,
	}
	gpoTransactionsFlag = cli.IntFlag{
		Name:     "gpo.transactions",
		Usage:    "Number of transactions to sample in a block",
		DefValue: defaultConfig.GPO.Transactions,
	}
	gpoPercentileFlag = cli.IntFlag{
		Name:     "gpo.percentile",
		Usage:    "Suggested gas price is the given percentile of a set of recent transaction gas prices",
		DefValue: defaultConfig.GPO.Percentile,
	}
	gpoDefaultPriceFlag = cli.Int64Flag{
		Name:     "gpo.defaultprice",
		Usage:    "The gas price to suggest before data is available, and the price to suggest when block utilization is low",
		DefValue: defaultConfig.GPO.DefaultPrice,
	}
	gpoMaxPriceFlag = cli.Int64Flag{
		Name:     "gpo.maxprice",
		Usage:    "Maximum gasprice to be recommended by gpo",
		DefValue: defaultConfig.GPO.MaxPrice,
	}
	gpoLowUsageThresholdFlag = cli.IntFlag{
		Name:     "gpo.low-usage-threshold",
		Usage:    "The block usage threshold below which the default gas price is suggested (0 to disable)",
		DefValue: defaultConfig.GPO.LowUsageThreshold,
	}
	gpoBlockGasLimitFlag = cli.IntFlag{
		Name:     "gpo.block-gas-limit",
		Usage:    "The gas limit, per block. If set to 0, it is pulled from the block header",
		DefValue: defaultConfig.GPO.BlockGasLimit,
	}
)

// metrics flags required for the go-eth library
// https://github.com/ethereum/go-ethereum/blob/master/metrics/metrics.go#L35-L55
var (
	metricsETHFlag = cli.BoolFlag{
		Name:     "metrics", // https://github.com/ethereum/go-ethereum/blob/master/metrics/metrics.go#L30
		Usage:    "flag required to enable the eth metrics",
		DefValue: false,
	}

	metricsExpensiveETHFlag = cli.BoolFlag{
		Name:     "metrics.expensive", // https://github.com/ethereum/go-ethereum/blob/master/metrics/metrics.go#L33
		Usage:    "flag required to enable the expensive eth metrics",
		DefValue: false,
	}
)

func applyShardDataFlags(cmd *cobra.Command, cfg *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, enableShardDataFlag) {
		cfg.ShardData.EnableShardData = cli.GetBoolFlagValue(cmd, enableShardDataFlag)
	}
	if cli.IsFlagChanged(cmd, diskCountFlag) {
		cfg.ShardData.DiskCount = cli.GetIntFlagValue(cmd, diskCountFlag)
	}
	if cli.IsFlagChanged(cmd, shardCountFlag) {
		cfg.ShardData.ShardCount = cli.GetIntFlagValue(cmd, shardCountFlag)
	}
	if cli.IsFlagChanged(cmd, cacheTimeFlag) {
		cfg.ShardData.CacheTime = cli.GetIntFlagValue(cmd, cacheTimeFlag)
	}
	if cli.IsFlagChanged(cmd, cacheSizeFlag) {
		cfg.ShardData.CacheSize = cli.GetIntFlagValue(cmd, cacheSizeFlag)
	}
}

func applyGPOFlags(cmd *cobra.Command, cfg *harmonyconfig.HarmonyConfig) {
	if cli.IsFlagChanged(cmd, gpoBlocksFlag) {
		cfg.GPO.Blocks = cli.GetIntFlagValue(cmd, gpoBlocksFlag)
	}
	if cli.IsFlagChanged(cmd, gpoTransactionsFlag) {
		cfg.GPO.Transactions = cli.GetIntFlagValue(cmd, gpoTransactionsFlag)
	}
	if cli.IsFlagChanged(cmd, gpoPercentileFlag) {
		cfg.GPO.Percentile = cli.GetIntFlagValue(cmd, gpoPercentileFlag)
	}
	if cli.IsFlagChanged(cmd, gpoDefaultPriceFlag) {
		cfg.GPO.DefaultPrice = cli.GetInt64FlagValue(cmd, gpoDefaultPriceFlag)
	}
	if cli.IsFlagChanged(cmd, gpoMaxPriceFlag) {
		cfg.GPO.MaxPrice = cli.GetInt64FlagValue(cmd, gpoMaxPriceFlag)
	}
	if cli.IsFlagChanged(cmd, gpoLowUsageThresholdFlag) {
		cfg.GPO.LowUsageThreshold = cli.GetIntFlagValue(cmd, gpoLowUsageThresholdFlag)
	}
	if cli.IsFlagChanged(cmd, gpoBlockGasLimitFlag) {
		cfg.GPO.BlockGasLimit = cli.GetIntFlagValue(cmd, gpoBlockGasLimitFlag)
	}
}
