package main

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	rosetta_common "github.com/harmony-one/harmony/rosetta/common"

	harmonyconfig "github.com/harmony-one/harmony/internal/configs/harmony"
	rpc_common "github.com/harmony-one/harmony/rpc/common"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/api/service"
	"github.com/harmony-one/harmony/api/service/pprof"
	"github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/harmony-one/harmony/api/service/synchronize"
	"github.com/harmony-one/harmony/common/fdlimit"
	"github.com/harmony-one/harmony/common/ntp"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/hmy/downloader"
	"github.com/harmony-one/harmony/internal/cli"
	"github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/webhooks"
)

// Host
var (
	myHost          p2p.Host
	initialAccounts = []*genesis.DeployAccount{}
)

var rootCmd = &cobra.Command{
	Use:   "harmony",
	Short: "harmony is the Harmony node binary file",
	Long: `harmony is the Harmony node binary file

Examples usage:

# start a validator node with default bls folder (default bls key files in ./.hmy/blskeys)
    ./harmony

# start a validator node with customized bls key folder
    ./harmony --bls.dir [bls_folder]

# start a validator node with open RPC endpoints and customized ports
    ./harmony --http.ip=0.0.0.0 --http.port=[http_port] --ws.ip=0.0.0.0 --ws.port=[ws_port]

# start an explorer node
    ./harmony --run=explorer --run.shard=[shard_id]

# start a harmony internal node on testnet
    ./harmony --run.legacy --network testnet
`,
	Run: runHarmonyNode,
}

var configFlag = cli.StringFlag{
	Name:      "config",
	Usage:     "load node config from the config toml file.",
	Shorthand: "c",
	DefValue:  "",
}

func init() {
	rand.Seed(time.Now().UnixNano())
	cli.SetParseErrorHandle(func(err error) {
		os.Exit(128) // 128 - invalid command line arguments
	})
	configCmd.AddCommand(dumpConfigCmd)
	configCmd.AddCommand(updateConfigCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(dumpConfigLegacyCmd)
	rootCmd.AddCommand(dumpDBCmd)

	if err := registerRootCmdFlags(); err != nil {
		os.Exit(2)
	}
	if err := registerDumpConfigFlags(); err != nil {
		os.Exit(2)
	}
	if err := registerDumpDBFlags(); err != nil {
		os.Exit(2)
	}
}

func main() {
	rootCmd.Execute()
}

func registerRootCmdFlags() error {
	flags := getRootFlags()

	return cli.RegisterFlags(rootCmd, flags)
}

func runHarmonyNode(cmd *cobra.Command, args []string) {
	if cli.GetBoolFlagValue(cmd, versionFlag) {
		printVersion()
		os.Exit(0)
	}

	if err := prepareRootCmd(cmd); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(128)
	}
	cfg, err := getHarmonyConfig(cmd)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(128)
	}

	setupNodeLog(cfg)
	setupNodeAndRun(cfg)
}

func prepareRootCmd(cmd *cobra.Command) error {
	// HACK Force usage of go implementation rather than the C based one. Do the right way, see the
	// notes one line 66,67 of https://golang.org/src/net/net.go that say can make the decision at
	// build time.
	os.Setenv("GODEBUG", "netdns=go")
	// Don't set higher than num of CPU. It will make go scheduler slower.
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Raise fd limits
	return raiseFdLimits()
}

func raiseFdLimits() error {
	limit, err := fdlimit.Maximum()
	if err != nil {
		return errors.Wrap(err, "Failed to retrieve file descriptor allowance")
	}
	_, err = fdlimit.Raise(uint64(limit))
	if err != nil {
		return errors.Wrap(err, "Failed to raise file descriptor allowance")
	}
	return nil
}

func getHarmonyConfig(cmd *cobra.Command) (harmonyconfig.HarmonyConfig, error) {
	var (
		config         harmonyconfig.HarmonyConfig
		err            error
		migratedFrom   string
		configFile     string
		isUsingDefault bool
	)
	if cli.IsFlagChanged(cmd, configFlag) {
		configFile = cli.GetStringFlagValue(cmd, configFlag)
		config, migratedFrom, err = loadHarmonyConfig(configFile)
	} else {
		nt := getNetworkType(cmd)
		config = getDefaultHmyConfigCopy(nt)
		isUsingDefault = true
	}
	if err != nil {
		return harmonyconfig.HarmonyConfig{}, err
	}
	if migratedFrom != defaultConfig.Version && !isUsingDefault {
		fmt.Printf("Old config version detected %s\n",
			migratedFrom)
		stat, _ := os.Stdin.Stat()
		// Ask to update if only using terminal
		if stat.Mode()&os.ModeCharDevice != 0 {
			if promptConfigUpdate() {
				err := updateConfigFile(configFile)
				if err != nil {
					fmt.Printf("Could not update config - %s", err.Error())
					fmt.Println("Update config manually with `./harmony config update [config_file]`")
				}
			}

		} else {
			fmt.Println("Update saved config with `./harmony config update [config_file]`")
		}
	}

	applyRootFlags(cmd, &config)

	if err := validateHarmonyConfig(config); err != nil {
		return harmonyconfig.HarmonyConfig{}, err
	}
	sanityFixHarmonyConfig(&config)
	return config, nil
}

func applyRootFlags(cmd *cobra.Command, config *harmonyconfig.HarmonyConfig) {
	// Misc flags shall be applied first since legacy ip / port is overwritten
	// by new ip / port flags
	applyLegacyMiscFlags(cmd, config)
	applyGeneralFlags(cmd, config)
	applyNetworkFlags(cmd, config)
	applyDNSSyncFlags(cmd, config)
	applyP2PFlags(cmd, config)
	applyHTTPFlags(cmd, config)
	applyWSFlags(cmd, config)
	applyRPCOptFlags(cmd, config)
	applyBLSFlags(cmd, config)
	applyConsensusFlags(cmd, config)
	applyTxPoolFlags(cmd, config)
	applyPprofFlags(cmd, config)
	applyLogFlags(cmd, config)
	applySysFlags(cmd, config)
	applyDevnetFlags(cmd, config)
	applyRevertFlags(cmd, config)
	applyPrometheusFlags(cmd, config)
	applySyncFlags(cmd, config)
	applyShardDataFlags(cmd, config)
}

func setupNodeLog(config harmonyconfig.HarmonyConfig) {
	logPath := filepath.Join(config.Log.Folder, config.Log.FileName)
	rotateSize := config.Log.RotateSize
	rotateCount := config.Log.RotateCount
	rotateMaxAge := config.Log.RotateMaxAge
	verbosity := config.Log.Verbosity

	utils.AddLogFile(logPath, rotateSize, rotateCount, rotateMaxAge)
	utils.SetLogVerbosity(log.Lvl(verbosity))
	if config.Log.Context != nil {
		ip := config.Log.Context.IP
		port := config.Log.Context.Port
		utils.SetLogContext(ip, strconv.Itoa(port))
	}
}

func setupNodeAndRun(hc harmonyconfig.HarmonyConfig) {
	var err error

	nodeconfigSetShardSchedule(hc)
	nodeconfig.SetShardingSchedule(shard.Schedule)
	nodeconfig.SetVersion(getHarmonyVersion())

	if hc.General.NodeType == "validator" {
		var err error
		if hc.General.NoStaking {
			err = setupLegacyNodeAccount(hc)
		} else {
			err = setupStakingNodeAccount(hc)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot set up node account: %s\n", err)
			os.Exit(1)
		}
	}
	if hc.General.NodeType == "validator" {
		fmt.Printf("%s mode; node key %s -> shard %d\n",
			map[bool]string{false: "Legacy", true: "Staking"}[!hc.General.NoStaking],
			nodeconfig.GetDefaultConfig().ConsensusPriKey.GetPublicKeys().SerializeToHexStr(),
			initialAccounts[0].ShardID)
	}
	if hc.General.NodeType != "validator" && hc.General.ShardID >= 0 {
		for _, initialAccount := range initialAccounts {
			utils.Logger().Info().
				Uint32("original", initialAccount.ShardID).
				Int("override", hc.General.ShardID).
				Msg("ShardID Override")
			initialAccount.ShardID = uint32(hc.General.ShardID)
		}
	}

	nodeConfig, err := createGlobalConfig(hc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR cannot configure node: %s\n", err)
		os.Exit(1)
	}

	// Update ethereum compatible chain ids
	params.UpdateEthChainIDByShard(nodeConfig.ShardID)

	currentNode := setupConsensusAndNode(hc, nodeConfig)
	nodeconfig.GetDefaultConfig().ShardID = nodeConfig.ShardID
	nodeconfig.GetDefaultConfig().IsOffline = nodeConfig.IsOffline
	nodeconfig.GetDefaultConfig().Downloader = nodeConfig.Downloader

	// Check NTP configuration
	accurate, err := ntp.CheckLocalTimeAccurate(nodeConfig.NtpServer)
	if !accurate {
		if os.IsTimeout(err) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "NTP query timed out. Continuing.\n")
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "Error: local timeclock is not accurate. Please config NTP properly.\n")
		}
	}
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("Check Local Time Accuracy Error")
	}

	// Parse RPC config
	nodeConfig.RPCServer = nodeconfig.RPCServerConfig{
		HTTPEnabled:        hc.HTTP.Enabled,
		HTTPIp:             hc.HTTP.IP,
		HTTPPort:           hc.HTTP.Port,
		HTTPAuthPort:       hc.HTTP.AuthPort,
		WSEnabled:          hc.WS.Enabled,
		WSIp:               hc.WS.IP,
		WSPort:             hc.WS.Port,
		WSAuthPort:         hc.WS.AuthPort,
		DebugEnabled:       hc.RPCOpt.DebugEnabled,
		RateLimiterEnabled: hc.RPCOpt.RateLimterEnabled,
		RequestsPerSecond:  hc.RPCOpt.RequestsPerSecond,
	}

	// Parse rosetta config
	nodeConfig.RosettaServer = nodeconfig.RosettaServerConfig{
		HTTPEnabled: hc.HTTP.RosettaEnabled,
		HTTPIp:      hc.HTTP.IP,
		HTTPPort:    hc.HTTP.RosettaPort,
	}

	if hc.Revert != nil && hc.Revert.RevertBefore != 0 && hc.Revert.RevertTo != 0 {
		chain := currentNode.Blockchain()
		if hc.Revert.RevertBeacon {
			chain = currentNode.Beaconchain()
		}
		curNum := chain.CurrentBlock().NumberU64()
		if curNum < uint64(hc.Revert.RevertBefore) && curNum >= uint64(hc.Revert.RevertTo) {
			// Remove invalid blocks
			for chain.CurrentBlock().NumberU64() >= uint64(hc.Revert.RevertTo) {
				curBlock := chain.CurrentBlock()
				rollbacks := []ethCommon.Hash{curBlock.Hash()}
				if err := chain.Rollback(rollbacks); err != nil {
					fmt.Printf("Revert failed: %v\n", err)
					os.Exit(1)
				}
				lastSig := curBlock.Header().LastCommitSignature()
				sigAndBitMap := append(lastSig[:], curBlock.Header().LastCommitBitmap()...)
				chain.WriteCommitSig(curBlock.NumberU64()-1, sigAndBitMap)
			}
			fmt.Printf("Revert finished. Current block: %v\n", chain.CurrentBlock().NumberU64())
			utils.Logger().Warn().
				Uint64("Current Block", chain.CurrentBlock().NumberU64()).
				Msg("Revert finished.")
			os.Exit(1)
		}
	}

	startMsg := "==== New Harmony Node ===="
	if hc.General.NodeType == nodeTypeExplorer {
		startMsg = "==== New Explorer Node ===="
	}

	utils.Logger().Info().
		Str("BLSPubKey", nodeConfig.ConsensusPriKey.GetPublicKeys().SerializeToHexStr()).
		Uint32("ShardID", nodeConfig.ShardID).
		Str("ShardGroupID", nodeConfig.GetShardGroupID().String()).
		Str("BeaconGroupID", nodeConfig.GetBeaconGroupID().String()).
		Str("ClientGroupID", nodeConfig.GetClientGroupID().String()).
		Str("Role", currentNode.NodeConfig.Role().String()).
		Str("Version", getHarmonyVersion()).
		Str("multiaddress",
			fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", hc.P2P.IP, hc.P2P.Port, myHost.GetID().Pretty()),
		).
		Msg(startMsg)

	nodeconfig.SetPeerID(myHost.GetID())

	if hc.Log.VerbosePrints.Config {
		utils.Logger().Info().Interface("config", rpc_common.Config{
			HarmonyConfig: hc,
			NodeConfig:    *nodeConfig,
			ChainConfig:   *currentNode.Blockchain().Config(),
		}).Msg("verbose prints config")
	}

	// Setup services
	if hc.Sync.Enabled {
		setupSyncService(currentNode, myHost, hc)
	}
	if currentNode.NodeConfig.Role() == nodeconfig.Validator {
		currentNode.RegisterValidatorServices()
	} else if currentNode.NodeConfig.Role() == nodeconfig.ExplorerNode {
		currentNode.RegisterExplorerServices()
	}
	if hc.Pprof.Enabled {
		setupPprofService(currentNode, hc)
	}
	if hc.Prometheus.Enabled {
		setupPrometheusService(currentNode, hc, nodeConfig.ShardID)
	}

	if hc.DNSSync.Server && !hc.General.IsOffline {
		utils.Logger().Info().Msg("support gRPC sync server")
		currentNode.SupportGRPCSyncServer(hc.DNSSync.ServerPort)
	}
	if hc.DNSSync.Client && !hc.General.IsOffline {
		utils.Logger().Info().Msg("go with gRPC sync client")
		currentNode.StartGRPCSyncClient()
	}

	if err := currentNode.StartServices(); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(-1)
	}

	if err := currentNode.StartRPC(); err != nil {
		utils.Logger().Warn().
			Err(err).
			Msg("StartRPC failed")
	}

	if err := currentNode.StartRosetta(); err != nil {
		utils.Logger().Warn().
			Err(err).
			Msg("Start Rosetta failed")
	}

	go listenOSSigAndShutDown(currentNode)

	if !hc.General.IsOffline {
		if err := myHost.Start(); err != nil {
			utils.Logger().Fatal().
				Err(err).
				Msg("Start p2p host failed")
		}

		if err := currentNode.BootstrapConsensus(); err != nil {
			fmt.Fprint(os.Stderr, "could not bootstrap consensus", err.Error())
			if !currentNode.NodeConfig.IsOffline {
				os.Exit(-1)
			}
		}

		if err := currentNode.StartPubSub(); err != nil {
			fmt.Fprint(os.Stderr, "could not begin network message handling for node", err.Error())
			os.Exit(-1)
		}
	}

	select {}
}

func nodeconfigSetShardSchedule(config harmonyconfig.HarmonyConfig) {
	switch config.Network.NetworkType {
	case nodeconfig.Mainnet:
		shard.Schedule = shardingconfig.MainnetSchedule
	case nodeconfig.Testnet:
		shard.Schedule = shardingconfig.TestnetSchedule
	case nodeconfig.Pangaea:
		shard.Schedule = shardingconfig.PangaeaSchedule
	case nodeconfig.Localnet:
		shard.Schedule = shardingconfig.LocalnetSchedule
	case nodeconfig.Partner:
		shard.Schedule = shardingconfig.PartnerSchedule
	case nodeconfig.Stressnet:
		shard.Schedule = shardingconfig.StressNetSchedule
	case nodeconfig.Devnet:
		var dnConfig harmonyconfig.DevnetConfig
		if config.Devnet != nil {
			dnConfig = *config.Devnet
		} else {
			dnConfig = getDefaultDevnetConfigCopy()
		}

		devnetConfig, err := shardingconfig.NewInstance(
			uint32(dnConfig.NumShards), dnConfig.ShardSize, dnConfig.HmyNodeSize, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccounts, nil, shardingconfig.VLBPE)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR invalid devnet sharding config: %s",
				err)
			os.Exit(1)
		}
		shard.Schedule = shardingconfig.NewFixedSchedule(devnetConfig)
	}
}

func findAccountsByPubKeys(config shardingconfig.Instance, pubKeys multibls.PublicKeys) {
	for _, key := range pubKeys {
		keyStr := key.Bytes.Hex()
		_, account := config.FindAccount(keyStr)
		if account != nil {
			initialAccounts = append(initialAccounts, account)
		}
	}
}

func setupLegacyNodeAccount(hc harmonyconfig.HarmonyConfig) error {
	genesisShardingConfig := shard.Schedule.InstanceForEpoch(big.NewInt(core.GenesisEpoch))
	multiBLSPubKey := setupConsensusKeys(hc, nodeconfig.GetDefaultConfig())

	reshardingEpoch := genesisShardingConfig.ReshardingEpoch()
	if len(reshardingEpoch) > 0 {
		for _, epoch := range reshardingEpoch {
			config := shard.Schedule.InstanceForEpoch(epoch)
			findAccountsByPubKeys(config, multiBLSPubKey)
			if len(initialAccounts) != 0 {
				break
			}
		}
	} else {
		findAccountsByPubKeys(genesisShardingConfig, multiBLSPubKey)
	}

	if len(initialAccounts) == 0 {
		fmt.Fprintf(
			os.Stderr,
			"ERROR cannot find your BLS key in the genesis/FN tables: %s\n",
			multiBLSPubKey.SerializeToHexStr(),
		)
		os.Exit(100)
	}

	for _, account := range initialAccounts {
		fmt.Printf("My Genesis Account: %v\n", *account)
	}
	return nil
}

func setupStakingNodeAccount(hc harmonyconfig.HarmonyConfig) error {
	pubKeys := setupConsensusKeys(hc, nodeconfig.GetDefaultConfig())
	shardID, err := nodeconfig.GetDefaultConfig().ShardIDFromConsensusKey()
	if err != nil {
		return errors.Wrap(err, "cannot determine shard to join")
	}
	if err := nodeconfig.GetDefaultConfig().ValidateConsensusKeysForSameShard(
		pubKeys, shardID,
	); err != nil {
		return err
	}
	for _, blsKey := range pubKeys {
		initialAccount := &genesis.DeployAccount{}
		initialAccount.ShardID = shardID
		initialAccount.BLSPublicKey = blsKey.Bytes.Hex()
		initialAccount.Address = ""
		initialAccounts = append(initialAccounts, initialAccount)
	}
	return nil
}

func createGlobalConfig(hc harmonyconfig.HarmonyConfig) (*nodeconfig.ConfigType, error) {
	var err error

	if len(initialAccounts) == 0 {
		initialAccounts = append(initialAccounts, &genesis.DeployAccount{ShardID: uint32(hc.General.ShardID)})
	}
	nodeConfig := nodeconfig.GetShardConfig(initialAccounts[0].ShardID)
	if hc.General.NodeType == nodeTypeValidator {
		// Set up consensus keys.
		setupConsensusKeys(hc, nodeConfig)
	} else {
		// set dummy bls key for consensus object
		nodeConfig.ConsensusPriKey = multibls.GetPrivateKeys(&bls.SecretKey{})
	}

	// Set network type
	netType := nodeconfig.NetworkType(hc.Network.NetworkType)
	nodeconfig.SetNetworkType(netType)                // sets for both global and shard configs
	nodeConfig.SetShardID(initialAccounts[0].ShardID) // sets shard ID
	nodeConfig.SetArchival(hc.General.IsBeaconArchival, hc.General.IsArchival)
	nodeConfig.IsOffline = hc.General.IsOffline
	nodeConfig.Downloader = hc.Sync.Downloader

	// P2P private key is used for secure message transfer between p2p nodes.
	nodeConfig.P2PPriKey, _, err = utils.LoadKeyFromFile(hc.P2P.KeyFile)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load or create P2P key at %#v",
			hc.P2P.KeyFile)
	}

	selfPeer := p2p.Peer{
		IP:              hc.P2P.IP,
		Port:            strconv.Itoa(hc.P2P.Port),
		ConsensusPubKey: nodeConfig.ConsensusPriKey[0].Pub.Object,
	}
	myHost, err = p2p.NewHost(p2p.HostConfig{
		Self:            &selfPeer,
		BLSKey:          nodeConfig.P2PPriKey,
		BootNodes:       hc.Network.BootNodes,
		DataStoreFile:   hc.P2P.DHTDataStore,
		DiscConcurrency: hc.P2P.DiscConcurrency,
		MaxConnPerIP:    hc.P2P.MaxConnsPerIP,
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create P2P network host")
	}

	nodeConfig.DBDir = hc.General.DataDir

	if hc.Legacy != nil && hc.Legacy.WebHookConfig != nil && len(*hc.Legacy.WebHookConfig) != 0 {
		p := *hc.Legacy.WebHookConfig
		config, err := webhooks.NewWebHooksFromPath(p)
		if err != nil {
			fmt.Fprintf(
				os.Stderr, "yaml path is bad: %s", p,
			)
			os.Exit(1)
		}
		nodeConfig.WebHooks.Hooks = config
	}

	nodeConfig.NtpServer = hc.Sys.NtpServer

	return nodeConfig, nil
}

func setupConsensusAndNode(hc harmonyconfig.HarmonyConfig, nodeConfig *nodeconfig.ConfigType) *node.Node {
	// Consensus object.
	// TODO: consensus object shouldn't start here
	decider := quorum.NewDecider(quorum.SuperMajorityVote, uint32(hc.General.ShardID))

	currentConsensus, err := consensus.New(
		myHost, nodeConfig.ShardID, p2p.Peer{}, nodeConfig.ConsensusPriKey, decider,
	)
	currentConsensus.Decider.SetMyPublicKeyProvider(func() (multibls.PublicKeys, error) {
		return currentConsensus.GetPublicKeys(), nil
	})

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error :%v \n", err)
		os.Exit(1)
	}

	// Parse minPeers from harmonyconfig.HarmonyConfig
	var minPeers int
	var aggregateSig bool
	if hc.Consensus != nil {
		minPeers = hc.Consensus.MinPeers
		aggregateSig = hc.Consensus.AggregateSig
	} else {
		minPeers = defaultConsensusConfig.MinPeers
		aggregateSig = defaultConsensusConfig.AggregateSig
	}
	currentConsensus.MinPeers = minPeers
	currentConsensus.AggregateSig = aggregateSig

	blacklist, err := setupBlacklist(hc)
	if err != nil {
		utils.Logger().Warn().Msgf("Blacklist setup error: %s", err.Error())
	}

	// Current node.
	var chainDBFactory shardchain.DBFactory
	if hc.ShardData.EnableShardData {
		chainDBFactory = &shardchain.LDBShardFactory{
			RootDir:    nodeConfig.DBDir,
			DiskCount:  hc.ShardData.DiskCount,
			ShardCount: hc.ShardData.ShardCount,
			CacheTime:  hc.ShardData.CacheTime,
			CacheSize:  hc.ShardData.CacheSize,
		}
	} else {
		chainDBFactory = &shardchain.LDBFactory{RootDir: nodeConfig.DBDir}
	}

	currentNode := node.New(myHost, currentConsensus, chainDBFactory, blacklist, nodeConfig.ArchiveModes(), &hc)

	if hc.Legacy != nil && hc.Legacy.TPBroadcastInvalidTxn != nil {
		currentNode.BroadcastInvalidTx = *hc.Legacy.TPBroadcastInvalidTxn
	} else {
		currentNode.BroadcastInvalidTx = defaultBroadcastInvalidTx
	}

	// Syncing provider is provided by following rules:
	//   1. If starting with a localnet or offline, use local sync peers.
	//   2. If specified with --dns=false, use legacy syncing which is syncing through self-
	//      discover peers.
	//   3. Else, use the dns for syncing.
	if hc.Network.NetworkType == nodeconfig.Localnet || hc.General.IsOffline {
		epochConfig := shard.Schedule.InstanceForEpoch(ethCommon.Big0)
		selfPort := hc.P2P.Port
		currentNode.SyncingPeerProvider = node.NewLocalSyncingPeerProvider(
			6000, uint16(selfPort), epochConfig.NumShards(), uint32(epochConfig.NumNodesPerShard()))
	} else if hc.DNSSync.LegacySyncing {
		currentNode.SyncingPeerProvider = node.NewLegacySyncingPeerProvider(currentNode)
	} else {
		currentNode.SyncingPeerProvider = node.NewDNSSyncingPeerProvider(hc.DNSSync.Zone, strconv.Itoa(hc.DNSSync.Port))
	}

	// TODO: refactor the creation of blockchain out of node.New()
	currentConsensus.Blockchain = currentNode.Blockchain()
	currentNode.NodeConfig.DNSZone = hc.DNSSync.Zone

	currentNode.NodeConfig.SetBeaconGroupID(
		nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID),
	)

	nodeconfig.GetDefaultConfig().DBDir = nodeConfig.DBDir
	processNodeType(hc, currentNode, currentConsensus)
	currentNode.NodeConfig.SetShardGroupID(nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(nodeConfig.ShardID)))
	currentNode.NodeConfig.SetClientGroupID(nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID))
	currentNode.NodeConfig.ConsensusPriKey = nodeConfig.ConsensusPriKey

	// This needs to be executed after consensus setup
	if err := currentNode.InitConsensusWithValidators(); err != nil {
		utils.Logger().Warn().
			Int("shardID", hc.General.ShardID).
			Err(err).
			Msg("InitConsensusWithMembers failed")
	}

	// Set the consensus ID to be the current block number
	viewID := currentNode.Blockchain().CurrentBlock().Header().ViewID().Uint64()
	currentConsensus.SetViewIDs(viewID + 1)
	utils.Logger().Info().
		Uint64("viewID", viewID).
		Msg("Init Blockchain")

	// Assign closure functions to the consensus object
	currentConsensus.SetBlockVerifier(currentNode.VerifyNewBlock)
	currentConsensus.PostConsensusJob = currentNode.PostConsensusProcessing
	// update consensus information based on the blockchain
	currentConsensus.SetMode(currentConsensus.UpdateConsensusInformation())
	currentConsensus.NextBlockDue = time.Now()
	return currentNode
}

func processNodeType(hc harmonyconfig.HarmonyConfig, currentNode *node.Node, currentConsensus *consensus.Consensus) {
	switch hc.General.NodeType {
	case nodeTypeExplorer:
		nodeconfig.SetDefaultRole(nodeconfig.ExplorerNode)
		currentNode.NodeConfig.SetRole(nodeconfig.ExplorerNode)

	case nodeTypeValidator:
		nodeconfig.SetDefaultRole(nodeconfig.Validator)
		currentNode.NodeConfig.SetRole(nodeconfig.Validator)

		if hc.General.IsBackup {
			currentConsensus.SetIsBackup(true)
		}
	}
}

func setupPprofService(node *node.Node, hc harmonyconfig.HarmonyConfig) {
	pprofConfig := pprof.Config{
		Enabled:            hc.Pprof.Enabled,
		ListenAddr:         hc.Pprof.ListenAddr,
		Folder:             hc.Pprof.Folder,
		ProfileNames:       hc.Pprof.ProfileNames,
		ProfileIntervals:   hc.Pprof.ProfileIntervals,
		ProfileDebugValues: hc.Pprof.ProfileDebugValues,
	}
	s := pprof.NewService(pprofConfig)
	node.RegisterService(service.Pprof, s)
}

func setupPrometheusService(node *node.Node, hc harmonyconfig.HarmonyConfig, sid uint32) {
	prometheusConfig := prometheus.Config{
		Enabled:    hc.Prometheus.Enabled,
		IP:         hc.Prometheus.IP,
		Port:       hc.Prometheus.Port,
		EnablePush: hc.Prometheus.EnablePush,
		Gateway:    hc.Prometheus.Gateway,
		Network:    hc.Network.NetworkType,
		Legacy:     hc.General.NoStaking,
		NodeType:   hc.General.NodeType,
		Shard:      sid,
		Instance:   myHost.GetID().Pretty(),
	}
	p := prometheus.NewService(prometheusConfig)
	node.RegisterService(service.Prometheus, p)
}

func setupSyncService(node *node.Node, host p2p.Host, hc harmonyconfig.HarmonyConfig) {
	blockchains := []*core.BlockChain{node.Blockchain()}
	if !node.IsRunningBeaconChain() {
		blockchains = append(blockchains, node.Beaconchain())
	}

	dConfig := downloader.Config{
		ServerOnly:   !hc.Sync.Downloader,
		Network:      nodeconfig.NetworkType(hc.Network.NetworkType),
		Concurrency:  hc.Sync.Concurrency,
		MinStreams:   hc.Sync.MinPeers,
		InitStreams:  hc.Sync.InitStreams,
		SmSoftLowCap: hc.Sync.DiscSoftLowCap,
		SmHardLowCap: hc.Sync.DiscHardLowCap,
		SmHiCap:      hc.Sync.DiscHighCap,
		SmDiscBatch:  hc.Sync.DiscBatch,
	}
	// If we are running side chain, we will need to do some extra works for beacon
	// sync
	if !node.IsRunningBeaconChain() {
		dConfig.BHConfig = &downloader.BeaconHelperConfig{
			BlockC:     node.BeaconBlockChannel,
			InsertHook: node.BeaconSyncHook,
		}
	}
	s := synchronize.NewService(host, blockchains, dConfig)

	node.RegisterService(service.Synchronize, s)

	d := s.Downloaders.GetShardDownloader(node.Blockchain().ShardID())
	if hc.Sync.Downloader {
		node.Consensus.SetDownloader(d) // Set downloader when stream client is active
	}
}

func setupBlacklist(hc harmonyconfig.HarmonyConfig) (map[ethCommon.Address]struct{}, error) {
	rosetta_common.InitRosettaFile(hc.TxPool.RosettaFixFile)

	utils.Logger().Debug().Msgf("Using blacklist file at `%s`", hc.TxPool.BlacklistFile)
	dat, err := ioutil.ReadFile(hc.TxPool.BlacklistFile)
	if err != nil {
		return nil, err
	}
	addrMap := make(map[ethCommon.Address]struct{})
	for _, line := range strings.Split(string(dat), "\n") {
		if len(line) != 0 { // blacklist file may have trailing empty string line
			b32 := strings.TrimSpace(strings.Split(string(line), "#")[0])
			addr, err := common.Bech32ToAddress(b32)
			if err != nil {
				return nil, err
			}
			addrMap[addr] = struct{}{}
		}
	}
	return addrMap, nil
}

func listenOSSigAndShutDown(node *node.Node) {
	// Prepare for graceful shutdown from os signals
	osSignal := make(chan os.Signal)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)
	sig := <-osSignal
	utils.Logger().Warn().Str("signal", sig.String()).Msg("Gracefully shutting down...")
	const msg = "Got %s signal. Gracefully shutting down...\n"
	fmt.Fprintf(os.Stderr, msg, sig)

	go node.ShutDown()

	for i := 10; i > 0; i-- {
		<-osSignal
		if i > 1 {
			fmt.Printf("Already shutting down, interrupt more to force quit: (times=%v)\n", i-1)
		}
	}
	fmt.Println("Forced QUIT.")
	os.Exit(-1)
}
