package main

import (
	"fmt"
	"strings"

	"github.com/harmony-one/harmony/internal/cli"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/spf13/cobra"
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
	}

	networkFlags = []cli.Flag{
		networkTypeFlag,
		bootNodeFlag,
		dnsZoneFlag,
		dnsPortFlag,

		legacyDNSZoneFlag,
		legacyDNSPortFlag,
		legacyDNSFlag,
		legacyNetworkTypeFlag,
	}

	p2pFlags = []cli.Flag{
		p2pPortFlag,
		p2pIPFlag,
		p2pKeyFileFlag,
		p2pDHTDataStoreFlag,

		legacyKeyFileFlag,
	}

	httpFlags = []cli.Flag{
		httpEnabledFlag,
		httpRosettaEnabledFlag,
		httpIPFlag,
		httpPortFlag,
		httpRosettaPortFlag,
	}

	wsFlags = []cli.Flag{
		wsEnabledFlag,
		wsIPFlag,
		wsPortFlag,
	}

	rpcOptFlags = []cli.Flag{
		rpcDebugEnabledFlag,
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
		tpBlacklistFileFlag,
		legacyTPBlacklistFileFlag,
	}

	pprofFlags = []cli.Flag{
		pprofEnabledFlag,
		pprofListenAddrFlag,
	}

	logFlags = []cli.Flag{
		logFolderFlag,
		logRotateSizeFlag,
		logFileNameFlag,
		logContextIPFlag,
		logContextPortFlag,
		logVerbosityFlag,
		legacyVerbosityFlag,

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
		syncLegacyClientFlag,
		syncLegacyServerFlag,
		syncConcurrencyFlag,
		syncMinPeersFlag,
		syncInitStreamsFlag,
		syncDiscSoftLowFlag,
		syncDiscHardLowFlag,
		syncDiscHighFlag,
		syncDiscBatchFlag,
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
)

func getRootFlags() []cli.Flag {
	var flags []cli.Flag

	flags = append(flags, versionFlag)
	flags = append(flags, configFlag)
	flags = append(flags, generalFlags...)
	flags = append(flags, networkFlags...)
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
	flags = append(flags, legacyMiscFlags...)
	flags = append(flags, prometheusFlags...)
	flags = append(flags, syncFlags...)

	return flags
}

func applyGeneralFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, nodeTypeFlag) {
		config.General.NodeType = cli.GetStringFlagValue(cmd, nodeTypeFlag)
	} else if cli.IsFlagChanged(cmd, legacyNodeTypeFlag) {
		config.General.NodeType = cli.GetStringFlagValue(cmd, legacyNodeTypeFlag)
	}

	if config.General.NodeType == nodeTypeExplorer {
		config.General.IsArchival = true
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
	dnsZoneFlag = cli.StringFlag{
		Name:  "dns.zone",
		Usage: "use customized peers from the zone for state syncing",
	}
	dnsPortFlag = cli.IntFlag{
		Name:     "dns.port",
		DefValue: nodeconfig.DefaultDNSPort,
		Usage:    "port of customized dns node",
	}
	legacyDNSZoneFlag = cli.StringFlag{
		Name:       "dns_zone",
		Usage:      "use peers from the zone for state syncing",
		Deprecated: "use --dns.zone",
	}
	legacyDNSPortFlag = cli.IntFlag{
		Name:       "dns_port",
		Usage:      "port of dns node",
		Deprecated: "use --dns.zone",
	}
	legacyDNSFlag = cli.BoolFlag{
		Name:       "dns",
		DefValue:   true,
		Usage:      "use dns for syncing",
		Deprecated: "only set to false to use self discovered peers for syncing",
	}
	legacyNetworkTypeFlag = cli.StringFlag{
		Name:       "network_type",
		Usage:      "network to join (mainnet, testnet, pangaea, localnet, partner, stressnet, devnet)",
		Deprecated: "use --network",
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

func applyNetworkFlags(cmd *cobra.Command, cfg *harmonyConfig) {
	if cli.IsFlagChanged(cmd, bootNodeFlag) {
		cfg.Network.BootNodes = cli.GetStringSliceFlagValue(cmd, bootNodeFlag)
	}

	if cli.IsFlagChanged(cmd, dnsZoneFlag) {
		cfg.Network.DNSZone = cli.GetStringFlagValue(cmd, dnsZoneFlag)
	} else if cli.IsFlagChanged(cmd, legacyDNSZoneFlag) {
		cfg.Network.DNSZone = cli.GetStringFlagValue(cmd, legacyDNSZoneFlag)
	} else if cli.IsFlagChanged(cmd, legacyDNSFlag) {
		val := cli.GetBoolFlagValue(cmd, legacyDNSFlag)
		if !val {
			cfg.Network.LegacySyncing = true
		}
	}

	if cli.IsFlagChanged(cmd, dnsPortFlag) {
		cfg.Network.DNSPort = cli.GetIntFlagValue(cmd, dnsPortFlag)
	} else if cli.IsFlagChanged(cmd, legacyDNSPortFlag) {
		cfg.Network.DNSPort = cli.GetIntFlagValue(cmd, legacyDNSPortFlag)
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
)

func applyP2PFlags(cmd *cobra.Command, config *harmonyConfig) {
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
)

func applyHTTPFlags(cmd *cobra.Command, config *harmonyConfig) {
	var isRPCSpecified, isRosettaSpecified bool

	if cli.IsFlagChanged(cmd, httpIPFlag) {
		config.HTTP.IP = cli.GetStringFlagValue(cmd, httpIPFlag)
		isRPCSpecified = true
	}

	if cli.IsFlagChanged(cmd, httpPortFlag) {
		config.HTTP.Port = cli.GetIntFlagValue(cmd, httpPortFlag)
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
)

func applyWSFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, wsEnabledFlag) {
		config.WS.Enabled = cli.GetBoolFlagValue(cmd, wsEnabledFlag)
	}
	if cli.IsFlagChanged(cmd, wsIPFlag) {
		config.WS.IP = cli.GetStringFlagValue(cmd, wsIPFlag)
	}
	if cli.IsFlagChanged(cmd, wsPortFlag) {
		config.WS.Port = cli.GetIntFlagValue(cmd, wsPortFlag)
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
)

func applyRPCOptFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, rpcDebugEnabledFlag) {
		config.RPCOpt.DebugEnabled = cli.GetBoolFlagValue(cmd, rpcDebugEnabledFlag)
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

func applyBLSFlags(cmd *cobra.Command, config *harmonyConfig) {
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

func applyBLSPassFlags(cmd *cobra.Command, config *harmonyConfig) {
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

func applyKMSFlags(cmd *cobra.Command, config *harmonyConfig) {
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

func applyLegacyBLSPassFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, legacyBLSPassFlag) {
		val := cli.GetStringFlagValue(cmd, legacyBLSPassFlag)
		legacyApplyBLSPassVal(val, config)
	}
	if cli.IsFlagChanged(cmd, legacyBLSPersistPassFlag) {
		config.BLSKeys.SavePassphrase = cli.GetBoolFlagValue(cmd, legacyBLSPersistPassFlag)
	}
}

func applyLegacyKMSFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, legacyKMSConfigSourceFlag) {
		val := cli.GetStringFlagValue(cmd, legacyKMSConfigSourceFlag)
		legacyApplyKMSSourceVal(val, config)
	}
}

func legacyApplyBLSPassVal(src string, config *harmonyConfig) {
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

func legacyApplyKMSSourceVal(src string, config *harmonyConfig) {
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

func applyConsensusFlags(cmd *cobra.Command, config *harmonyConfig) {
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
	tpBlacklistFileFlag = cli.StringFlag{
		Name:     "txpool.blacklist",
		Usage:    "file of blacklisted wallet addresses",
		DefValue: defaultConfig.TxPool.BlacklistFile,
	}
	legacyTPBlacklistFileFlag = cli.StringFlag{
		Name:       "blacklist",
		Usage:      "Path to newline delimited file of blacklisted wallet addresses",
		DefValue:   defaultConfig.TxPool.BlacklistFile,
		Deprecated: "use --txpool.blacklist",
	}
)

func applyTxPoolFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, tpBlacklistFileFlag) {
		config.TxPool.BlacklistFile = cli.GetStringFlagValue(cmd, tpBlacklistFileFlag)
	} else if cli.IsFlagChanged(cmd, legacyTPBlacklistFileFlag) {
		config.TxPool.BlacklistFile = cli.GetStringFlagValue(cmd, legacyTPBlacklistFileFlag)
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
)

func applyPprofFlags(cmd *cobra.Command, config *harmonyConfig) {
	var pprofSet bool
	if cli.IsFlagChanged(cmd, pprofListenAddrFlag) {
		config.Pprof.ListenAddr = cli.GetStringFlagValue(cmd, pprofListenAddrFlag)
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

func applyLogFlags(cmd *cobra.Command, config *harmonyConfig) {
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

	if cli.IsFlagChanged(cmd, logFileNameFlag) {
		config.Log.FileName = cli.GetStringFlagValue(cmd, logFileNameFlag)
	}

	if cli.IsFlagChanged(cmd, logVerbosityFlag) {
		config.Log.Verbosity = cli.GetIntFlagValue(cmd, logVerbosityFlag)
	} else if cli.IsFlagChanged(cmd, legacyVerbosityFlag) {
		config.Log.Verbosity = cli.GetIntFlagValue(cmd, legacyVerbosityFlag)
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

func applySysFlags(cmd *cobra.Command, config *harmonyConfig) {
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

func applyDevnetFlags(cmd *cobra.Command, config *harmonyConfig) {
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

func applyRevertFlags(cmd *cobra.Command, config *harmonyConfig) {
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
func applyLegacyMiscFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, legacyPortFlag) {
		legacyPort := cli.GetIntFlagValue(cmd, legacyPortFlag)
		config.P2P.Port = legacyPort
		config.HTTP.Port = nodeconfig.GetRPCHTTPPortFromBase(legacyPort)
		config.HTTP.RosettaPort = nodeconfig.GetRosettaHTTPPortFromBase(legacyPort)
		config.WS.Port = nodeconfig.GetWSPortFromBase(legacyPort)
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

		logCtx := &logContext{
			IP:   logIP,
			Port: logPort,
		}
		config.Log.Context = logCtx
	}

	if cli.HasFlagsChanged(cmd, []cli.Flag{legacyWebHookConfigFlag, legacyTPBroadcastInvalidTxFlag}) {
		config.Legacy = &legacyConfig{}
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

func applyPrometheusFlags(cmd *cobra.Command, config *harmonyConfig) {
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
	syncLegacyServerFlag = cli.BoolFlag{
		Name:     "sync.legacy.server",
		Usage:    "Enable the gRPC sync server for backward compatibility",
		Hidden:   true,
		DefValue: true,
	}
	syncLegacyClientFlag = cli.BoolFlag{
		Name:     "sync.legacy.client",
		Usage:    "Enable the legacy centralized sync service for block synchronization",
		Hidden:   true,
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
func applySyncFlags(cmd *cobra.Command, config *harmonyConfig) {
	if config.Sync == (syncConfig{}) {
		nt := nodeconfig.NetworkType(config.Network.NetworkType)
		config.Sync = getDefaultSyncConfig(nt)
	}

	if cli.IsFlagChanged(cmd, syncLegacyServerFlag) {
		config.Sync.LegacyServer = cli.GetBoolFlagValue(cmd, syncLegacyServerFlag)
	}

	if cli.IsFlagChanged(cmd, syncLegacyClientFlag) {
		config.Sync.LegacyClient = cli.GetBoolFlagValue(cmd, syncLegacyClientFlag)
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
