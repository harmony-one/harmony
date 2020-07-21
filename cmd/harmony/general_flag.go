package main

import (
	"fmt"

	"github.com/harmony-one/harmony/internal/cli"
	"github.com/spf13/cobra"
)

var generalFlags = []cli.Flag{
	nodeTypeFlag,
	isStakingFlag,
	shardIDFlag,
	isArchiveFlag,
	dataDirFlag,

	legacyNodeTypeFlag,
	legacyIsStakingFlag,
	legacyShardIDFlag,
	legacyIsArchiveFlag,
	legacyDataDirFlag,
}

var (
	nodeTypeFlag = cli.StringFlag{
		Name:     "run",
		Usage:    "run node type (validator, explorer)",
		DefValue: defaultConfig.General.NodeType,
	}
	isStakingFlag = cli.BoolFlag{
		Name:     "run.staking",
		Usage:    "whether to run node in staking mode",
		DefValue: defaultConfig.General.IsStaking,
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
		DefValue:   defaultConfig.General.IsStaking,
		Deprecated: "use --run.staking",
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

func applyGeneralFlags(cmd *cobra.Command, config *hmyConfig) {
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

	if cli.IsFlagChanged(cmd, isStakingFlag) {
		config.General.IsStaking = cli.GetBoolFlagValue(cmd, isStakingFlag)
	} else if cli.IsFlagChanged(cmd, legacyIsStakingFlag) {
		config.General.IsStaking = cli.GetBoolFlagValue(cmd, legacyIsStakingFlag)
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

var consensusFlags = []cli.Flag{
	consensusDelayCommitFlag,
	consensusBlockTimeFlag,

	legacyDelayCommitFlag,
	legacyBlockTimeFlag,
}

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
)

func applyConsensusFlags(cmd *cobra.Command, config *hmyConfig) {
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
}

var txPoolFlags = []cli.Flag{
	tpBlacklistFileFlag,
	tpBroadcastInvalidTxFlag,

	legacyTPBlacklistFileFlag,
	legacyTPBroadcastInvalidTxFlag,
}

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

func applyTxPoolFlags(cmd *cobra.Command, config *hmyConfig) {
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

var pprofFlags = []cli.Flag{
	pprofEnabledFlag,
	pprofListenAddrFlag,
}

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

func applyPprofFlags(cmd *cobra.Command, config *hmyConfig) {
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

var logFlags = []cli.Flag{
	logFolderFlag,
	logRotateSizeFlag,

	legacyLogFolderFlag,
	legacyLogRotateSizeFlag,
}

var (
	logFolderFlag = cli.StringFlag{
		Name:     "log.path",
		Usage:    "directory path to put rotation logs",
		DefValue: defaultConfig.Log.LogFolder,
	}
	logRotateSizeFlag = cli.IntFlag{
		Name:     "log.max-size",
		Usage:    "rotation log size in megabytes",
		DefValue: defaultConfig.Log.LogRotateSize,
	}
	legacyLogFolderFlag = cli.StringFlag{
		Name:       "log_folder",
		Usage:      "the folder collecting the logs of this execution",
		DefValue:   defaultConfig.Log.LogFolder,
		Deprecated: "use --log.path",
	}
	legacyLogRotateSizeFlag = cli.IntFlag{
		Name:       "log_max_size",
		Usage:      "the max size in megabytes of the log file before it gets rotated",
		DefValue:   defaultConfig.Log.LogRotateSize,
		Deprecated: "use --log.max-size",
	}
)

func applyLogFlags(cmd *cobra.Command, config *hmyConfig) {
	if cli.IsFlagChanged(cmd, logFolderFlag) {
		config.Log.LogFolder = cli.GetStringFlagValue(cmd, logFolderFlag)
	} else if cli.IsFlagChanged(cmd, legacyLogFolderFlag) {
		config.Log.LogFolder = cli.GetStringFlagValue(cmd, legacyLogFolderFlag)
	}

	if cli.IsFlagChanged(cmd, logRotateSizeFlag) {
		config.Log.LogRotateSize = cli.GetIntFlagValue(cmd, logRotateSizeFlag)
	} else if cli.IsFlagChanged(cmd, legacyLogRotateSizeFlag) {
		config.Log.LogRotateSize = cli.GetIntFlagValue(cmd, legacyLogRotateSizeFlag)
	}
}

var newDevnetFlags = []cli.Flag{
	devnetNumShardsFlag,
	devnetShardSizeFlag,
	devnetHmyNodeSizeFlag,
}

var legacyDevnetFlags = []cli.Flag{
	legacyDevnetNumShardsFlag,
	legacyDevnetShardSizeFlag,
	legacyDevnetHmyNodeSizeFlag,
}

var devnetFlags = append(newDevnetFlags, legacyDevnetFlags...)

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

func applyDevnetFlags(cmd *cobra.Command, config *hmyConfig) {
	if cli.HasFlagsChanged(cmd, devnetFlags) && config.Devnet != nil {
		devnet := getDefaultDevnetConfigCopy()
		config.Devnet = &devnet
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
}

var revertFlags = append(newRevertFlags, legacyRevertFlags...)

var newRevertFlags = []cli.Flag{
	revertBeaconFlag,
	revertToFlag,
	revertBeforeFlag,
}

var legacyRevertFlags = []cli.Flag{
	legacyRevertBeaconFlag,
	legacyRevertBeforeFlag,
	legacyRevertToFlag,
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
		Usage:    "rollback all blocks until and including block number revert_to",
		DefValue: defaultRevertConfig.RevertTo,
		Hidden:   true,
	}
	revertBeforeFlag = cli.IntFlag{
		Name:     "revert.do-before",
		Usage:    "if the current block is less than do_revert_before, revert all blocks until (including) revert_to block",
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

func applyRevertFlags(cmd *cobra.Command, config *hmyConfig) {
	if cli.HasFlagsChanged(cmd, revertFlags) {
		revert := getDefaultRevertConfigCopy()
		config.Revert = &revert
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

// legacyMiscFlags are legacy flags that cannot be categorized to a single category.
var legacyMiscFlags = []cli.Flag{
	legacyPortFlag,
	legacyFreshDB,
}

var (
	legacyPortFlag = cli.IntFlag{
		Name:       "port",
		Usage:      "port of the node",
		DefValue:   defaultConfig.P2P.Port,
		Deprecated: "Use --p2p.port, --http.port instead",
	}
	legacyFreshDB = cli.BoolFlag{
		Name:       "fresh_db",
		Usage:      "true means the existing disk based db will be removed",
		DefValue:   false,
		Deprecated: "will be removed in future version",
	}
)

// Note: this function need to be called before parse other flags
func applyMiscFlags(cmd *cobra.Command, config *hmyConfig) {
	// TODO: move all port manipulation +500 -3000 logic here
	if cli.IsFlagChanged(cmd, legacyPortFlag) {
		legacyPort := cli.GetIntFlagValue(cmd, legacyPortFlag)
		config.P2P.Port = legacyPort
		config.RPC.Port = legacyPort
	}
}
