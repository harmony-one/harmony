package main

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/cli"
	"github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/webhooks"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Host
var (
	myHost          p2p.Host
	initialAccounts = []*genesis.DeployAccount{}
)

var rootCmd = &cobra.Command{
	// TODO: elaborate the usage
	Use:   "harmony",
	Short: "run harmony node",
	Long:  "run harmony node",
	Run:   runHarmonyNode,
}

var configFlag = cli.StringFlag{
	Name:      "config",
	Usage:     "load node config from the config toml file.",
	Shorthand: "c",
	DefValue:  "",
}

func init() {
	cli.SetParseErrorHandle(func(err error) {
		os.Exit(128) // 128 - invalid command line arguments
	})
	rootCmd.AddCommand(dumpConfigCmd)
	rootCmd.AddCommand(versionCmd)

	if err := registerRootCmdFlags(); err != nil {
		os.Exit(2)
	}
	if err := registerDumpConfigFlags(); err != nil {
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
	prepareRootCmd(cmd)

	cfg, err := getHarmonyConfig(cmd)
	if err != nil {
		fmt.Println(err)
		cmd.Help()
		os.Exit(128)
	}

	setupNodeLog(cfg)
	setupPprof(cfg)
	setupNodeAndRun(cfg)
}

func prepareRootCmd(cmd *cobra.Command) {
	// HACK Force usage of go implementation rather than the C based one. Do the right way, see the
	// notes one line 66,67 of https://golang.org/src/net/net.go that say can make the decision at
	// build time.
	os.Setenv("GODEBUG", "netdns=go")
	// Don't set higher than num of CPU. It will make go scheduler slower.
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Set up randomization seed.
	rand.Seed(int64(time.Now().Nanosecond()))
}

func getHarmonyConfig(cmd *cobra.Command) (harmonyConfig, error) {
	var (
		config harmonyConfig
		err    error
	)
	if cli.IsFlagChanged(cmd, configFlag) {
		configFile := cli.GetStringFlagValue(cmd, configFlag)
		config, err = loadHarmonyConfig(configFile)
	} else {
		nt := getNetworkType(cmd)
		config = getDefaultHmyConfigCopy(nt)
	}
	if err != nil {
		return harmonyConfig{}, err
	}

	applyRootFlags(cmd, &config)

	if err := validateHarmonyConfig(config); err != nil {
		return harmonyConfig{}, err
	}

	return config, nil
}

func applyRootFlags(cmd *cobra.Command, config *harmonyConfig) {
	// Misc flags shall be applied first since legacy ip / port is overwritten
	// by new ip / port flags
	applyLegacyMiscFlags(cmd, config)
	applyGeneralFlags(cmd, config)
	applyNetworkFlags(cmd, config)
	applyP2PFlags(cmd, config)
	applyRPCFlags(cmd, config)
	applyWSFlags(cmd, config)
	applyBLSFlags(cmd, config)
	applyConsensusFlags(cmd, config)
	applyTxPoolFlags(cmd, config)
	applyPprofFlags(cmd, config)
	applyLogFlags(cmd, config)
	applyDevnetFlags(cmd, config)
	applyRevertFlags(cmd, config)
}

func setupNodeLog(config harmonyConfig) {
	logPath := filepath.Join(config.Log.Folder, config.Log.FileName)
	rotateSize := config.Log.RotateSize
	verbosity := config.Log.Verbosity

	utils.AddLogFile(logPath, rotateSize)
	utils.SetLogVerbosity(log.Lvl(verbosity))
	if config.Log.Context != nil {
		ip := config.Log.Context.IP
		port := config.Log.Context.Port
		utils.SetLogContext(ip, strconv.Itoa(port))
	}
}

func setupPprof(config harmonyConfig) {
	enabled := config.Pprof.Enabled
	addr := config.Pprof.ListenAddr

	if enabled {
		go func() {
			http.ListenAndServe(addr, nil)
		}()
	}
}

func setupNodeAndRun(hc harmonyConfig) {
	var err error
	bootNodes := hc.Network.BootNodes
	p2p.BootNodes, err = p2p.StringsToAddrs(bootNodes)
	if err != nil {
		utils.FatalErrMsg(err, "cannot parse bootnode list %#v",
			bootNodes)
	}
	// TODO: seperate use of port rpc / p2p
	nodeconfig.GetDefaultConfig().Port = strconv.Itoa(hc.RPC.Port)
	nodeconfig.GetDefaultConfig().IP = hc.RPC.IP

	nodeconfig.SetShardingSchedule(shard.Schedule)
	nodeconfig.SetPublicRPC(true)
	nodeconfig.SetVersion(getHarmonyVersion())
	nodeconfigSetShardSchedule(hc)

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
	currentNode := setupConsensusAndNode(hc, nodeConfig)
	nodeconfig.GetDefaultConfig().ShardID = nodeConfig.ShardID

	// Prepare for graceful shutdown from os signals
	osSignal := make(chan os.Signal)
	signal.Notify(osSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range osSignal {
			if sig == syscall.SIGTERM || sig == os.Interrupt {
				const msg = "Got %s signal. Gracefully shutting down...\n"
				utils.Logger().Printf(msg, sig)
				fmt.Printf(msg, sig)
				currentNode.ShutDown()
			}
		}
	}()

	if nodeConfig.ShardID != shard.BeaconChainShardID {
		utils.Logger().Info().
			Uint32("shardID", currentNode.Blockchain().ShardID()).
			Uint32("shardID", nodeConfig.ShardID).Msg("SupportBeaconSyncing")
		currentNode.SupportBeaconSyncing()
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
				chain.Rollback(rollbacks)
				lastSig := curBlock.Header().LastCommitSignature()
				sigAndBitMap := append(lastSig[:], curBlock.Header().LastCommitBitmap()...)
				chain.WriteCommitSig(curBlock.NumberU64()-1, sigAndBitMap)
			}
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
		Str("multiaddress",
			fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", hc.RPC.IP, hc.P2P.Port, myHost.GetID().Pretty()),
		).
		Msg(startMsg)

	nodeconfig.SetPeerID(myHost.GetID())

	currentNode.SupportSyncing()
	currentNode.ServiceManagerSetup()
	currentNode.RunServices()

	if err := currentNode.StartRPC(strconv.Itoa(hc.RPC.Port)); err != nil {
		utils.Logger().Warn().
			Err(err).
			Msg("StartRPC failed")
	}

	if err := currentNode.BootstrapConsensus(); err != nil {
		fmt.Println("could not bootstrap consensus", err.Error())
		os.Exit(-1)
	}

	if err := currentNode.Start(); err != nil {
		fmt.Println("could not begin network message handling for node", err.Error())
		os.Exit(-1)
	}
}

func nodeconfigSetShardSchedule(config harmonyConfig) {
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
		var dnConfig devnetConfig
		if config.Devnet != nil {
			dnConfig = *config.Devnet
		} else {
			dnConfig = getDefaultDevnetConfigCopy()
		}

		// TODO (leo): use a passing list of accounts here
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

func setupLegacyNodeAccount(hc harmonyConfig) error {
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

func setupStakingNodeAccount(hc harmonyConfig) error {
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

func createGlobalConfig(hc harmonyConfig) (*nodeconfig.ConfigType, error) {
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
	nodeconfig.SetNetworkType(netType) // sets for both global and shard configs
	nodeConfig.SetArchival(hc.General.IsArchival)

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

	myHost, err = p2p.NewHost(&selfPeer, nodeConfig.P2PPriKey)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create P2P network host")
	}

	nodeConfig.DBDir = hc.General.DataDir

	if hc.Legacy != nil && len(hc.Legacy.WebHookConfig) != 0 {
		p := hc.Legacy.WebHookConfig
		config, err := webhooks.NewWebHooksFromPath(p)
		if err != nil {
			fmt.Fprintf(
				os.Stderr, "yaml path is bad: %s", p,
			)
			os.Exit(1)
		}
		nodeConfig.WebHooks.Hooks = config
	}

	return nodeConfig, nil
}

func setupConsensusAndNode(hc harmonyConfig, nodeConfig *nodeconfig.ConfigType) *node.Node {
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
	commitDelay, err := time.ParseDuration(hc.Consensus.DelayCommit)
	if err != nil || commitDelay < 0 {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR invalid commit delay %#v", hc.Consensus.DelayCommit)
		os.Exit(1)
	}
	currentConsensus.SetCommitDelay(commitDelay)
	currentConsensus.MinPeers = hc.Consensus.MinPeers

	blacklist, err := setupBlacklist(hc)
	if err != nil {
		utils.Logger().Warn().Msgf("Blacklist setup error: %s", err.Error())
	}

	// Current node.
	chainDBFactory := &shardchain.LDBFactory{RootDir: nodeConfig.DBDir}

	currentNode := node.New(myHost, currentConsensus, chainDBFactory, blacklist, hc.General.IsArchival)
	currentNode.BroadcastInvalidTx = hc.TxPool.BroadcastInvalidTx

	if hc.Network.LegacySyncing {
		currentNode.SyncingPeerProvider = node.NewLegacySyncingPeerProvider(currentNode)
	} else {
		if hc.Network.NetworkType == nodeconfig.Localnet {
			epochConfig := shard.Schedule.InstanceForEpoch(ethCommon.Big0)
			selfPort := hc.Network.DNSPort
			currentNode.SyncingPeerProvider = node.NewLocalSyncingPeerProvider(
				6000, uint16(selfPort), epochConfig.NumShards(), uint32(epochConfig.NumNodesPerShard()))
		} else {
			currentNode.SyncingPeerProvider = node.NewDNSSyncingPeerProvider(hc.Network.DNSZone, syncing.GetSyncingPort(strconv.Itoa(hc.Network.DNSPort)))
		}
	}

	// TODO: refactor the creation of blockchain out of node.New()
	currentConsensus.ChainReader = currentNode.Blockchain()
	currentNode.NodeConfig.DNSZone = hc.Network.DNSZone

	currentNode.NodeConfig.SetBeaconGroupID(
		nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID),
	)

	nodeconfig.GetDefaultConfig().DBDir = nodeConfig.DBDir
	switch hc.General.NodeType {
	case nodeTypeExplorer:
		nodeconfig.SetDefaultRole(nodeconfig.ExplorerNode)
		currentNode.NodeConfig.SetRole(nodeconfig.ExplorerNode)

	case nodeTypeValidator:
		nodeconfig.SetDefaultRole(nodeconfig.Validator)
		currentNode.NodeConfig.SetRole(nodeconfig.Validator)
	}
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
	currentConsensus.SetViewID(viewID + 1)
	utils.Logger().Info().
		Uint64("viewID", viewID).
		Msg("Init Blockchain")

	// Assign closure functions to the consensus object
	currentConsensus.BlockVerifier = currentNode.VerifyNewBlock
	currentConsensus.OnConsensusDone = currentNode.PostConsensusProcessing
	// update consensus information based on the blockchain
	currentConsensus.SetMode(currentConsensus.UpdateConsensusInformation())
	// Setup block period and block due time.
	// TODO: move the error check to validation
	currentConsensus.BlockPeriod, err = time.ParseDuration(hc.Consensus.BlockTime)
	if err != nil {
		fmt.Printf("Unknown block period: %v\n", hc.Consensus.BlockTime)
		os.Exit(128)
	}
	currentConsensus.NextBlockDue = time.Now()
	return currentNode
}

func setupBlacklist(hc harmonyConfig) (map[ethCommon.Address]struct{}, error) {
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
