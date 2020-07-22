package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/harmony-one/harmony/internal/cli"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	viperconfig "github.com/harmony-one/harmony/internal/configs/viper"
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
)

// Host
var (
	myHost          p2p.Host
	initialAccounts = []*genesis.DeployAccount{}
)

func printVersion() {
	fmt.Fprintln(os.Stdout, nodeconfig.GetVersion())
	os.Exit(0)
}

//ip         = flag.String("ip", "127.0.0.1", "ip of the node")
//port       = flag.String("port", "9000", "port of the node.")
//logFolder  = flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
//logMaxSize = flag.Int("log_max_size", 100, "the max size in megabytes of the log file before it gets rotated")
//// Remove this flag
freshDB     = flag.Bool("fresh_db", false, "true means the existing disk based db will be removed")
//pprof       = flag.String("pprof", "", "what address and port the pprof profiling server should listen on")
//versionFlag = flag.Bool("version", false, "Output version info")
//dnsZone     = flag.String("dns_zone", "", "if given and not empty, use peers from the zone (default: use libp2p peer discovery instead)")
//dnsFlag     = flag.Bool("dns", true, "[deprecated] equivalent to -dns_zone t.hmny.io")
//dnsPort     = flag.String("dns_port", "9000", "port of dns node")
////Leader needs to have a minimal number of peers to start consensus
//minPeers = flag.Int("min_peers", 32, "Minimal number of Peers in shard")
//// Key file to store the private key
//keyFile = flag.String("key", "./.hmykey", "the p2p key file of the harmony node")
//// isArchival indicates this node is an archival node that will save and archive current blockchain
//isArchival = flag.Bool("is_archival", false, "false will enable cached state pruning")
//// delayCommit is the commit-delay timer, used by Harmony nodes
//// TODO: check whether this field can be removed
//delayCommit = flag.String("delay_commit", "0ms", "how long to delay sending commit messages in consensus, ex: 500ms, 1s")
//// nodeType indicates the type of the node: validator, explorer
//nodeType = flag.String("node_type", "validator", "node type: validator, explorer")
//// networkType indicates the type of the network
//networkType = flag.String("network_type", "mainnet", "type of the network: mainnet, testnet, pangaea, partner, stressnet, devnet, localnet")
//// blockPeriod indicates the how long the leader waits to propose a new block.
//blockPeriod = flag.Int("block_period", 8, "how long in second the leader waits to propose a new block")
//// staking indicates whether the node is operating in staking mode.
//stakingFlag = flag.Bool("staking", false, "whether the node should operate in staking mode")
//// shardID indicates the shard ID of this node
//shardID = flag.Int("shard_id", -1, "the shard ID of this node")
//// Sharding configuration parameters for devnet
//devnetNumShards   = flag.Uint("dn_num_shards", 2, "number of shards for -network_type=devnet (default: 2)")
//devnetShardSize   = flag.Int("dn_shard_size", 10, "number of nodes per shard for -network_type=devnet (default 10)")
//devnetHarmonySize = flag.Int("dn_hmy_size", -1, "number of Harmony-operated nodes per shard for -network_type=devnet; negative (default) means equal to -dn_shard_size")
//// logging verbosity
//verbosity = flag.Int("verbosity", 5, "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 5)")
//// dbDir is the database directory.
//dbDir     = flag.String("db_dir", "", "blockchain database directory")
//publicRPC = flag.Bool("public_rpc", false, "Enable Public RPC Access (default: false)")
//// Bad block revert
//// TODO: Seperate revert to a different command
//doRevertBefore = flag.Int("do_revert_before", 0, "If the current block is less than do_revert_before, revert all blocks until (including) revert_to block")
//revertTo       = flag.Int("revert_to", 0, "The revert will rollback all blocks until and including block number revert_to")
//revertBeacon   = flag.Bool("revert_beacon", false, "Whether to revert beacon chain or the chain this node is assigned to")
//// Blacklist of addresses
//blacklistPath      = flag.String("blacklist", "./.hmy/blacklist.txt", "Path to newline delimited file of blacklisted wallet addresses")
//broadcastInvalidTx = flag.Bool("broadcast_invalid_tx", false, "Broadcast invalid transactions to sync pool state (default: false)")
//webHookYamlPath    = flag.String(
//	"webhook_yaml", "", "path for yaml config reporting double signing",
//)

func init() {
	if err := registerRootCmdFlags(); err != nil {
		// Only happens when code is wrong
		panic(err)
	}
	cli.SetParseErrorHandle(func(err error) {
		os.Exit(128) // 128 - invalid command line arguments
	})

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

func setupLegacyNodeAccount() error {
	genesisShardingConfig := shard.Schedule.InstanceForEpoch(big.NewInt(core.GenesisEpoch))
	multiBLSPubKey := setupConsensusKeys(nodeconfig.GetDefaultConfig())

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

func setupStakingNodeAccount() error {
	pubKeys := setupConsensusKeys(nodeconfig.GetDefaultConfig())
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

// setupConsensusKeys load bls keys and set the keys to nodeConfig. Return the loaded public keys.
func setupConsensusKeys(config *nodeconfig.ConfigType) multibls.PublicKeys {
	onceLoadBLSKey.Do(func() {
		var err error
		multiBLSPriKey, err = loadBLSKeys()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR when loading bls key: %v\n", err)
			os.Exit(100)
		}
		fmt.Printf("Successfully loaded %v BLS keys\n", len(multiBLSPriKey))
	})
	config.ConsensusPriKey = multiBLSPriKey
	return multiBLSPriKey.GetPublicKeys()
}

func createGlobalConfig() (*nodeconfig.ConfigType, error) {
	var err error

	if len(initialAccounts) == 0 {
		initialAccounts = append(initialAccounts, &genesis.DeployAccount{ShardID: uint32(*shardID)})
	}
	nodeConfig := nodeconfig.GetShardConfig(initialAccounts[0].ShardID)
	if *nodeType == "validator" {
		// Set up consensus keys.
		setupConsensusKeys(nodeConfig)
	} else {
		// set dummy bls key for consensus object
		nodeConfig.ConsensusPriKey = multibls.GetPrivateKeys(&bls.SecretKey{})
	}

	// Set network type
	netType := nodeconfig.NetworkType(*networkType)
	nodeconfig.SetNetworkType(netType) // sets for both global and shard configs
	nodeConfig.SetArchival(*isArchival)

	// P2P private key is used for secure message transfer between p2p nodes.
	nodeConfig.P2PPriKey, _, err = utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load or create P2P key at %#v",
			*keyFile)
	}

	selfPeer := p2p.Peer{
		IP:              *ip,
		Port:            *port,
		ConsensusPubKey: nodeConfig.ConsensusPriKey[0].Pub.Object,
	}

	myHost, err = p2p.NewHost(&selfPeer, nodeConfig.P2PPriKey)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create P2P network host")
	}

	nodeConfig.DBDir = *dbDir

	if p := *webHookYamlPath; p != "" {
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

func setupConsensusAndNode(nodeConfig *nodeconfig.ConfigType) *node.Node {
	// Consensus object.
	// TODO: consensus object shouldn't start here
	decider := quorum.NewDecider(quorum.SuperMajorityVote, uint32(*shardID))

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
	commitDelay, err := time.ParseDuration(*delayCommit)
	if err != nil || commitDelay < 0 {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR invalid commit delay %#v", *delayCommit)
		os.Exit(1)
	}
	currentConsensus.SetCommitDelay(commitDelay)
	currentConsensus.MinPeers = *minPeers

	blacklist, err := setupBlacklist()
	if err != nil {
		utils.Logger().Warn().Msgf("Blacklist setup error: %s", err.Error())
	}

	// Current node.
	chainDBFactory := &shardchain.LDBFactory{RootDir: nodeConfig.DBDir}

	currentNode := node.New(myHost, currentConsensus, chainDBFactory, blacklist, *isArchival)
	currentNode.BroadcastInvalidTx = *broadcastInvalidTx

	switch {
	case *networkType == nodeconfig.Localnet:
		epochConfig := shard.Schedule.InstanceForEpoch(ethCommon.Big0)
		selfPort, err := strconv.ParseUint(*port, 10, 16)
		if err != nil {
			utils.Logger().Fatal().
				Err(err).
				Str("self_port_string", *port).
				Msg("cannot convert self port string into port number")
		}
		currentNode.SyncingPeerProvider = node.NewLocalSyncingPeerProvider(
			6000, uint16(selfPort), epochConfig.NumShards(), uint32(epochConfig.NumNodesPerShard()))
	case *dnsZone != "":
		currentNode.SyncingPeerProvider = node.NewDNSSyncingPeerProvider(*dnsZone, syncing.GetSyncingPort(*dnsPort))
	case *dnsFlag:
		currentNode.SyncingPeerProvider = node.NewDNSSyncingPeerProvider("t.hmny.io", syncing.GetSyncingPort(*dnsPort))
	default:
		currentNode.SyncingPeerProvider = node.NewLegacySyncingPeerProvider(currentNode)

	}

	// TODO: refactor the creation of blockchain out of node.New()
	currentConsensus.ChainReader = currentNode.Blockchain()
	currentNode.NodeConfig.DNSZone = *dnsZone

	currentNode.NodeConfig.SetBeaconGroupID(
		nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID),
	)

	nodeconfig.GetDefaultConfig().DBDir = nodeConfig.DBDir
	switch *nodeType {
	case "explorer":
		nodeconfig.SetDefaultRole(nodeconfig.ExplorerNode)
		currentNode.NodeConfig.SetRole(nodeconfig.ExplorerNode)
	case "validator":
		nodeconfig.SetDefaultRole(nodeconfig.Validator)
		currentNode.NodeConfig.SetRole(nodeconfig.Validator)
	}
	currentNode.NodeConfig.SetShardGroupID(nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(nodeConfig.ShardID)))
	currentNode.NodeConfig.SetClientGroupID(nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID))
	currentNode.NodeConfig.ConsensusPriKey = nodeConfig.ConsensusPriKey

	// This needs to be executed after consensus setup
	if err := currentNode.InitConsensusWithValidators(); err != nil {
		utils.Logger().Warn().
			Int("shardID", *shardID).
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
	currentConsensus.BlockPeriod = time.Duration(*blockPeriod) * time.Second
	currentConsensus.NextBlockDue = time.Now()
	return currentNode
}

func setupBlacklist() (map[ethCommon.Address]struct{}, error) {
	utils.Logger().Debug().Msgf("Using blacklist file at `%s`", *blacklistPath)
	dat, err := ioutil.ReadFile(*blacklistPath)
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

func main() {
	if *nodeType == "validator" {
		var err error
		if *stakingFlag {
			err = setupStakingNodeAccount()
		} else {
			err = setupLegacyNodeAccount()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot set up node account: %s\n", err)
			os.Exit(1)
		}
	}
	if *nodeType == "validator" {
		fmt.Printf("%s mode; node key %s -> shard %d\n",
			map[bool]string{false: "Legacy", true: "Staking"}[*stakingFlag],
			nodeconfig.GetDefaultConfig().ConsensusPriKey.GetPublicKeys().SerializeToHexStr(),
			initialAccounts[0].ShardID)
	}
	if *nodeType != "validator" && *shardID >= 0 {
		for _, initialAccount := range initialAccounts {
			utils.Logger().Info().
				Uint32("original", initialAccount.ShardID).
				Int("override", *shardID).
				Msg("ShardID Override")
			initialAccount.ShardID = uint32(*shardID)
		}
	}

	nodeConfig, err := createGlobalConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR cannot configure node: %s\n", err)
		os.Exit(1)
	}
	currentNode := setupConsensusAndNode(nodeConfig)
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

	if uint64(*doRevertBefore) != 0 && uint64(*revertTo) != 0 {
		chain := currentNode.Blockchain()
		if *revertBeacon {
			chain = currentNode.Beaconchain()
		}
		curNum := chain.CurrentBlock().NumberU64()
		if curNum < uint64(*doRevertBefore) && curNum >= uint64(*revertTo) {
			// Remove invalid blocks
			for chain.CurrentBlock().NumberU64() >= uint64(*revertTo) {
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
	if *nodeType == "explorer" {
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
			fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, myHost.GetID().Pretty()),
		).
		Msg(startMsg)

	nodeconfig.SetPeerID(myHost.GetID())

	currentNode.SupportSyncing()
	currentNode.ServiceManagerSetup()
	currentNode.RunServices()

	if err := currentNode.StartRPC(*port); err != nil {
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
