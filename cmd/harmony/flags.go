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
		p2pIPFlag,
		p2pPortFlag,
		p2pKeyFileFlag,

		legacyKeyFileFlag,
	}

	rpcFlags = []cli.Flag{
		rpcEnabledFlag,
		rpcIPFlag,
		rpcPortFlag,

		legacyPublicRPCFlag,
	}

	wsFlags = []cli.Flag{
		wsEnabledFlag,
		wsIPFlag,
		wsPortFlag,
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

	consensusFlags = []cli.Flag{
		consensusDelayCommitFlag,
		consensusBlockTimeFlag,
		consensusMinPeersFlag,

		legacyDelayCommitFlag,
		legacyBlockTimeFlag,
		legacyConsensusMinPeersFlag,
	}

	txPoolFlags = []cli.Flag{
		tpBlacklistFileFlag,
		tpBroadcastInvalidTxFlag,

		legacyTPBlacklistFileFlag,
		legacyTPBroadcastInvalidTxFlag,
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
		legacyWebHookConfigFlag,
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
		Usage:    "run node in archive mode",
		DefValue: defaultConfig.General.IsArchival,
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

	flags = append(flags, configFlag)
	flags = append(flags, generalFlags...)
	flags = append(flags, networkFlags...)
	flags = append(flags, p2pFlags...)
	flags = append(flags, rpcFlags...)
	flags = append(flags, wsFlags...)
	flags = append(flags, blsFlags...)
	flags = append(flags, consensusFlags...)
	flags = append(flags, txPoolFlags...)
	flags = append(flags, pprofFlags...)
	flags = append(flags, logFlags...)
	flags = append(flags, devnetFlags...)
	flags = append(flags, revertFlags...)
	flags = append(flags, legacyMiscFlags...)

	return flags
}

func applyGeneralFlags(cmd *cobra.Command, config *harmonyConfig) {
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

	if cli.IsFlagChanged(cmd, dataDirFlag) {
		config.General.DataDir = cli.GetStringFlagValue(cmd, dataDirFlag)
	} else if cli.IsFlagChanged(cmd, legacyDataDirFlag) {
		config.General.DataDir = cli.GetStringFlagValue(cmd, legacyDataDirFlag)
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
	p2pIPFlag = cli.StringFlag{
		Name:     "p2p.ip",
		Usage:    "ip to listen for p2p protocols",
		DefValue: defaultConfig.P2P.IP,
	}
	p2pPortFlag = cli.IntFlag{
		Name:     "p2p.port",
		Usage:    "port to listen for p2p protocols",
		DefValue: defaultConfig.P2P.Port,
	}
	p2pKeyFileFlag = cli.StringFlag{
		Name:     "p2p.keyfile",
		Usage:    "the p2p key file of the harmony node",
		DefValue: defaultConfig.P2P.KeyFile,
	}
	legacyKeyFileFlag = cli.StringFlag{
		Name:       "key",
		Usage:      "the p2p key file of the harmony node",
		DefValue:   defaultConfig.P2P.KeyFile,
		Deprecated: "use --p2p.keyfile",
	}
)

func applyP2PFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, p2pIPFlag) {
		config.P2P.IP = cli.GetStringFlagValue(cmd, p2pIPFlag)
	}

	if cli.IsFlagChanged(cmd, p2pPortFlag) {
		config.P2P.Port = cli.GetIntFlagValue(cmd, p2pPortFlag)
	}

	if cli.IsFlagChanged(cmd, p2pKeyFileFlag) {
		config.P2P.KeyFile = cli.GetStringFlagValue(cmd, p2pKeyFileFlag)
	} else if cli.IsFlagChanged(cmd, legacyKeyFileFlag) {
		config.P2P.KeyFile = cli.GetStringFlagValue(cmd, legacyKeyFileFlag)
	}
}

// rpc flags
var (
	rpcEnabledFlag = cli.BoolFlag{
		Name:     "http",
		Usage:    "enable HTTP / RPC requests",
		DefValue: defaultConfig.HTTP.Enabled,
	}
	rpcIPFlag = cli.StringFlag{
		Name:     "http.ip",
		Usage:    "ip address to listen for RPC calls",
		DefValue: defaultConfig.HTTP.IP,
	}
	rpcPortFlag = cli.IntFlag{
		Name:     "http.port",
		Usage:    "rpc port to listen for HTTP requests",
		DefValue: defaultConfig.HTTP.Port,
	}
	legacyPublicRPCFlag = cli.BoolFlag{
		Name:       "public_rpc",
		Usage:      "Enable Public HTTP Access (default: false)",
		DefValue:   defaultConfig.HTTP.Enabled,
		Deprecated: "use --http.ip and --ws.ip to specify the ip address to listen. Use 127.0.0.1 to listen local requests.",
	}
)

func applyRPCFlags(cmd *cobra.Command, config *harmonyConfig) {
	var isRPCSpecified bool

	if cli.IsFlagChanged(cmd, rpcIPFlag) {
		config.HTTP.IP = cli.GetStringFlagValue(cmd, rpcIPFlag)
		isRPCSpecified = true
	}

	if cli.IsFlagChanged(cmd, rpcPortFlag) {
		config.HTTP.Port = cli.GetIntFlagValue(cmd, rpcPortFlag)
		isRPCSpecified = true
	}

	if cli.IsFlagChanged(cmd, rpcEnabledFlag) {
		config.HTTP.Enabled = cli.GetBoolFlagValue(cmd, rpcEnabledFlag)
	} else if isRPCSpecified {
		config.HTTP.Enabled = true
	}

	if cli.IsFlagChanged(cmd, legacyPublicRPCFlag) {
		if !cli.GetBoolFlagValue(cmd, legacyPublicRPCFlag) {
			config.HTTP.IP = localEndpoint
			config.WS.IP = localEndpoint
		}
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
		Usage:    "ip endpoint for websocket",
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
	// TODO: hard code value?
	consensusDelayCommitFlag = cli.StringFlag{
		Name:     "consensus.delay-commit",
		Usage:    "how long to delay sending commit messages in consensus, e.g: 500ms, 1s",
		DefValue: defaultConfig.Consensus.DelayCommit,
		Hidden:   true,
	}
	// TODO: hard code value?
	consensusBlockTimeFlag = cli.StringFlag{
		Name:     "consensus.block-time",
		Usage:    "block interval time, e.g: 8s",
		DefValue: defaultConfig.Consensus.BlockTime,
		Hidden:   true,
	}
	// TODO: hard code value?
	consensusMinPeersFlag = cli.IntFlag{
		Name:     "consensus.min-peers",
		Usage:    "minimal number of peers in shard",
		DefValue: defaultConfig.Consensus.MinPeers,
		Hidden:   true,
	}
	legacyDelayCommitFlag = cli.StringFlag{
		Name:       "delay_commit",
		Usage:      "how long to delay sending commit messages in consensus, ex: 500ms, 1s",
		DefValue:   defaultConfig.Consensus.DelayCommit,
		Deprecated: "use --consensus.delay-commit",
	}
	legacyBlockTimeFlag = cli.IntFlag{
		Name:       "block_period",
		Usage:      "how long in second the leader waits to propose a new block",
		DefValue:   8,
		Deprecated: "use --consensus.block-time",
	}
	legacyConsensusMinPeersFlag = cli.IntFlag{
		Name:     "min_peers",
		Usage:    "Minimal number of Peers in shard",
		DefValue: defaultConfig.Consensus.MinPeers,
		Hidden:   true,
	}
)

func applyConsensusFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, consensusDelayCommitFlag) {
		config.Consensus.DelayCommit = cli.GetStringFlagValue(cmd, consensusDelayCommitFlag)
	} else if cli.IsFlagChanged(cmd, legacyDelayCommitFlag) {
		config.Consensus.DelayCommit = cli.GetStringFlagValue(cmd, legacyDelayCommitFlag)
	}

	if cli.IsFlagChanged(cmd, consensusBlockTimeFlag) {
		config.Consensus.BlockTime = cli.GetStringFlagValue(cmd, consensusBlockTimeFlag)
	} else if cli.IsFlagChanged(cmd, legacyBlockTimeFlag) {
		sec := cli.GetIntFlagValue(cmd, legacyBlockTimeFlag)
		config.Consensus.BlockTime = fmt.Sprintf("%ds", sec)
	}

	if cli.IsFlagChanged(cmd, consensusMinPeersFlag) {
		config.Consensus.MinPeers = cli.GetIntFlagValue(cmd, consensusMinPeersFlag)
	} else if cli.IsFlagChanged(cmd, legacyConsensusMinPeersFlag) {
		config.Consensus.MinPeers = cli.GetIntFlagValue(cmd, legacyConsensusMinPeersFlag)
	}
}

// transaction pool flags
var (
	tpBlacklistFileFlag = cli.StringFlag{
		Name:     "txpool.blacklist",
		Usage:    "file of blacklisted wallet addresses",
		DefValue: defaultConfig.TxPool.BlacklistFile,
	}
	// TODO: mark hard code?
	tpBroadcastInvalidTxFlag = cli.BoolFlag{
		Name:     "txpool.broadcast-invalid-tx",
		Usage:    "whether to broadcast invalid transactions",
		DefValue: defaultConfig.TxPool.BroadcastInvalidTx,
		Hidden:   true,
	}
	legacyTPBlacklistFileFlag = cli.StringFlag{
		Name:       "blacklist",
		Usage:      "Path to newline delimited file of blacklisted wallet addresses",
		DefValue:   defaultConfig.TxPool.BlacklistFile,
		Deprecated: "use --txpool.blacklist",
	}
	legacyTPBroadcastInvalidTxFlag = cli.BoolFlag{
		Name:       "broadcast_invalid_tx",
		Usage:      "broadcast invalid transactions to sync pool state",
		DefValue:   defaultConfig.TxPool.BroadcastInvalidTx,
		Deprecated: "use --txpool.broadcast-invalid-tx",
	}
)

func applyTxPoolFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, tpBlacklistFileFlag) {
		config.TxPool.BlacklistFile = cli.GetStringFlagValue(cmd, tpBlacklistFileFlag)
	} else if cli.IsFlagChanged(cmd, legacyTPBlacklistFileFlag) {
		config.TxPool.BlacklistFile = cli.GetStringFlagValue(cmd, legacyTPBlacklistFileFlag)
	}

	if cli.IsFlagChanged(cmd, tpBroadcastInvalidTxFlag) {
		config.TxPool.BroadcastInvalidTx = cli.GetBoolFlagValue(cmd, tpBroadcastInvalidTxFlag)
	} else if cli.IsFlagChanged(cmd, legacyTPBroadcastInvalidTxFlag) {
		config.TxPool.BroadcastInvalidTx = cli.GetBoolFlagValue(cmd, legacyTPBroadcastInvalidTxFlag)
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
	devnetNumShardsFlag = cli.IntFlag{
		Name:     "devnet.num-shard",
		Usage:    "number of shards for devnet",
		DefValue: defaultDevnetConfig.NumShards,
	}
	devnetShardSizeFlag = cli.IntFlag{
		Name:     "devnet.shard-size",
		Usage:    "number of nodes per shard for devnet",
		DefValue: defaultDevnetConfig.ShardSize,
	}
	devnetHmyNodeSizeFlag = cli.IntFlag{
		Name:     "devnet.hmy-node-size",
		Usage:    "number of Harmony-operated nodes per shard for devnet (negative means equal to --devnet.shard-size)",
		DefValue: defaultDevnetConfig.HmyNodeSize,
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
	legacyWebHookConfigFlag = cli.StringFlag{
		Name:     "webhook_yaml",
		Usage:    "path for yaml config reporting double signing",
		DefValue: "",
		Hidden:   true,
	}
)

// Note: this function need to be called before parse other flags
func applyLegacyMiscFlags(cmd *cobra.Command, config *harmonyConfig) {
	if cli.IsFlagChanged(cmd, legacyPortFlag) {
		legacyPort := cli.GetIntFlagValue(cmd, legacyPortFlag)
		config.P2P.Port = legacyPort
		config.HTTP.Port = nodeconfig.GetHTTPPortFromBase(legacyPort)
		config.WS.Port = nodeconfig.GetWSPortFromBase(legacyPort)
	}

	if cli.IsFlagChanged(cmd, legacyIPFlag) {
		legacyIP := cli.GetStringFlagValue(cmd, legacyIPFlag)
		config.HTTP.IP = legacyIP
		config.HTTP.Enabled = true
		config.P2P.IP = legacyIP
		config.WS.IP = legacyIP
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

	if cli.IsFlagChanged(cmd, legacyWebHookConfigFlag) {
		config.Legacy = &legacyConfig{
			WebHookConfig: cli.GetStringFlagValue(cmd, legacyWebHookConfigFlag),
		}
	}
}
