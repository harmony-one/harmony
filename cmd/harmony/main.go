package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path"
	"runtime"
	"strconv"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/blsgen"
	"github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/genesis"
	hmykey "github.com/harmony-one/harmony/internal/keystore"
	"github.com/harmony-one/harmony/internal/memprofiling"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

// Version string variables
var (
	version string
	builtBy string
	builtAt string
	commit  string
)

// Host
var (
	myHost p2p.Host
)

// InitLDBDatabase initializes a LDBDatabase. isGenesis=true will return the beacon chain database for normal shard nodes
func InitLDBDatabase(ip string, port string, freshDB bool, isBeacon bool) (*ethdb.LDBDatabase, error) {
	var dbFileName string
	if isBeacon {
		dbFileName = fmt.Sprintf("./db/harmony_beacon_%s_%s", ip, port)
	} else {
		dbFileName = fmt.Sprintf("./db/harmony_%s_%s", ip, port)
	}
	if freshDB {
		var err = os.RemoveAll(dbFileName)
		if err != nil {
			fmt.Println(err.Error())
		}
	}
	return ethdb.NewLDBDatabase(dbFileName, 0, 0)
}

func printVersion() {
	fmt.Fprintln(os.Stderr, nodeconfig.GetVersion())
	os.Exit(0)
}

// Flags
var (
	ip               = flag.String("ip", "127.0.0.1", "ip of the node")
	port             = flag.String("port", "9000", "port of the node.")
	logFolder        = flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	logMaxSize       = flag.Int("log_max_size", 100, "the max size in megabytes of the log file before it gets rotated")
	freshDB          = flag.Bool("fresh_db", false, "true means the existing disk based db will be removed")
	profile          = flag.Bool("profile", false, "Turn on profiling (CPU, Memory).")
	metricsReportURL = flag.String("metrics_report_url", "", "If set, reports metrics to this URL.")
	versionFlag      = flag.Bool("version", false, "Output version info")
	onlyLogTps       = flag.Bool("only_log_tps", false, "Only log TPS if true")
	dnsZone          = flag.String("dns_zone", "", "if given and not empty, use peers from the zone (default: use libp2p peer discovery instead)")
	dnsFlag          = flag.Bool("dns", true, "[deprecated] equivalent to -dns_zone t.hmny.io")
	//Leader needs to have a minimal number of peers to start consensus
	minPeers = flag.Int("min_peers", 32, "Minimal number of Peers in shard")
	// Key file to store the private key
	keyFile = flag.String("key", "./.hmykey", "the p2p key file of the harmony node")
	// isGenesis indicates this node is a genesis node
	isGenesis = flag.Bool("is_genesis", true, "true means this node is a genesis node")
	// isArchival indicates this node is an archival node that will save and archive current blockchain
	isArchival = flag.Bool("is_archival", true, "false makes node faster by turning caching off")
	// delayCommit is the commit-delay timer, used by Harmony nodes
	delayCommit = flag.String("delay_commit", "0ms", "how long to delay sending commit messages in consensus, ex: 500ms, 1s")
	// nodeType indicates the type of the node: validator, explorer
	nodeType = flag.String("node_type", "validator", "node type: validator, explorer")
	// networkType indicates the type of the network
	networkType = flag.String("network_type", "mainnet", "type of the network: mainnet, testnet, devnet, localnet")
	// syncFreq indicates sync frequency
	syncFreq = flag.Int("sync_freq", 60, "unit in seconds")
	// beaconSyncFreq indicates beaconchain sync frequency
	beaconSyncFreq = flag.Int("beacon_sync_freq", 60, "unit in seconds")

	// blockPeriod indicates the how long the leader waits to propose a new block.
	blockPeriod    = flag.Int("block_period", 8, "how long in second the leader waits to propose a new block.")
	leaderOverride = flag.Bool("leader_override", false, "true means override the default leader role and acts as validator")
	// shardID indicates the shard ID of this node
	shardID            = flag.Int("shard_id", -1, "the shard ID of this node")
	enableMemProfiling = flag.Bool("enableMemProfiling", false, "Enable memsize logging.")
	enableGC           = flag.Bool("enableGC", true, "Enable calling garbage collector manually .")
	blsKeyFile         = flag.String("blskey_file", "", "The encrypted file of bls serialized private key by passphrase.")
	blsPass            = flag.String("blspass", "", "The file containing passphrase to decrypt the encrypted bls file.")
	blsPassphrase      string

	// Sharding configuration parameters for devnet
	devnetNumShards   = flag.Uint("dn_num_shards", 2, "number of shards for -network_type=devnet (default: 2)")
	devnetShardSize   = flag.Int("dn_shard_size", 10, "number of nodes per shard for -network_type=devnet (default 10)")
	devnetHarmonySize = flag.Int("dn_hmy_size", -1, "number of Harmony-operated nodes per shard for -network_type=devnet; negative (default) means equal to -dn_shard_size")

	// logConn logs incoming/outgoing connections
	logConn = flag.Bool("log_conn", false, "log incoming/outgoing connections")

	keystoreDir = flag.String("keystore", hmykey.DefaultKeyStoreDir, "The default keystore directory")

	initialAccount = &genesis.DeployAccount{}

	// logging verbosity
	verbosity = flag.Int("verbosity", 5, "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 5)")

	// dbDir is the database directory.
	dbDir = flag.String("db_dir", "", "blockchain database directory")

	// Disable view change.
	disableViewChange = flag.Bool("disable_view_change", false, "Do not propose view change (testing only)")

	// metrics flag to collct meetrics or not, pushgateway ip and port for metrics
	metricsFlag     = flag.Bool("metrics", false, "Collect and upload node metrics")
	pushgatewayIP   = flag.String("pushgateway_ip", "grafana.harmony.one", "Metrics view ip")
	pushgatewayPort = flag.String("pushgateway_port", "9091", "Metrics view port")

	publicRPC = flag.Bool("public_rpc", false, "Enable Public RPC Access (default: false)")
)

func initSetup() {

	// maybe request passphrase for bls key.
	passphraseForBls()

	// Configure log parameters
	utils.SetLogContext(*port, *ip)
	utils.SetLogVerbosity(log.Lvl(*verbosity))
	utils.AddLogFile(fmt.Sprintf("%v/validator-%v-%v.log", *logFolder, *ip, *port), *logMaxSize)

	if *onlyLogTps {
		matchFilterHandler := log.MatchFilterHandler("msg", "TPS Report", utils.GetLogInstance().GetHandler())
		utils.GetLogInstance().SetHandler(matchFilterHandler)
	}

	// Add GOMAXPROCS to achieve max performance.
	runtime.GOMAXPROCS(runtime.NumCPU() * 4)

	// Set port and ip to global config.
	nodeconfig.GetDefaultConfig().Port = *port
	nodeconfig.GetDefaultConfig().IP = *ip

	// Setup mem profiling.
	memprofiling.GetMemProfiling().Config()

	// Set default keystore Dir
	hmykey.DefaultKeyStoreDir = *keystoreDir

	// Set up randomization seed.
	rand.Seed(int64(time.Now().Nanosecond()))

	if len(utils.BootNodes) == 0 {
		bootNodeAddrs, err := utils.StringsToAddrs(utils.DefaultBootNodeAddrStrings)
		if err != nil {
			panic(err)
		}
		utils.BootNodes = bootNodeAddrs
	}
}

func passphraseForBls() {
	// If FN node running, they should either specify blsPrivateKey or the file with passphrase
	// However, explorer or non-validator nodes need no blskey
	if *nodeType != "validator" {
		return
	}

	if *blsKeyFile == "" || *blsPass == "" {
		fmt.Println("Internal nodes need to have pass to decrypt blskey")
		os.Exit(101)
	}
	passphrase, err := utils.GetPassphraseFromSource(*blsPass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR when reading passphrase file: %v\n", err)
		os.Exit(100)
	}
	blsPassphrase = passphrase
}

func setupInitialAccount() (isLeader bool) {
	genesisShardingConfig := core.ShardingSchedule.InstanceForEpoch(big.NewInt(core.GenesisEpoch))
	pubKey := setupConsensusKey(nodeconfig.GetDefaultConfig())

	reshardingEpoch := genesisShardingConfig.ReshardingEpoch()
	if reshardingEpoch != nil && len(reshardingEpoch) > 0 {
		for _, epoch := range reshardingEpoch {
			config := core.ShardingSchedule.InstanceForEpoch(epoch)
			isLeader, initialAccount = config.FindAccount(pubKey.SerializeToHexStr())
			if initialAccount != nil {
				break
			}
		}
	} else {
		isLeader, initialAccount = genesisShardingConfig.FindAccount(pubKey.SerializeToHexStr())
	}

	if initialAccount == nil {
		fmt.Fprintf(os.Stderr, "ERROR cannot find your BLS key in the genesis/FN tables: %s\n", pubKey.SerializeToHexStr())
		os.Exit(100)
	}

	fmt.Printf("My Genesis Account: %v\n", *initialAccount)

	return isLeader
}

func setupConsensusKey(nodeConfig *nodeconfig.ConfigType) *bls.PublicKey {
	consensusPriKey, err := blsgen.LoadBlsKeyWithPassPhrase(*blsKeyFile, blsPassphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR when loading bls key, err :%v\n", err)
		os.Exit(100)
	}
	pubKey := consensusPriKey.GetPublicKey()

	// Consensus keys are the BLS12-381 keys used to sign consensus messages
	nodeConfig.ConsensusPriKey, nodeConfig.ConsensusPubKey = consensusPriKey, consensusPriKey.GetPublicKey()
	if nodeConfig.ConsensusPriKey == nil || nodeConfig.ConsensusPubKey == nil {
		fmt.Println("error to get consensus keys.")
		os.Exit(100)
	}
	return pubKey
}

func createGlobalConfig() *nodeconfig.ConfigType {
	var err error

	nodeConfig := nodeconfig.GetShardConfig(initialAccount.ShardID)
	if *nodeType == "validator" {
		// Set up consensus keys.
		setupConsensusKey(nodeConfig)
	} else {
		nodeConfig.ConsensusPriKey = &bls.SecretKey{} // set dummy bls key for consensus object
	}

	// Set network type
	netType := nodeconfig.NetworkType(*networkType)
	switch netType {
	case nodeconfig.Mainnet, nodeconfig.Testnet, nodeconfig.Pangaea, nodeconfig.Localnet, nodeconfig.Devnet:
		nodeconfig.SetNetworkType(netType)
	default:
		panic(fmt.Sprintf("invalid network type: %s", *networkType))
	}

	nodeConfig.SetPushgatewayIP(*pushgatewayIP)
	nodeConfig.SetPushgatewayPort(*pushgatewayPort)
	nodeConfig.SetMetricsFlag(*metricsFlag)

	// P2p private key is used for secure message transfer between p2p nodes.
	nodeConfig.P2pPriKey, _, err = utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	selfPeer := p2p.Peer{IP: *ip, Port: *port, ConsensusPubKey: nodeConfig.ConsensusPubKey}

	myHost, err = p2pimpl.NewHost(&selfPeer, nodeConfig.P2pPriKey)
	if *logConn && nodeConfig.GetNetworkType() != nodeconfig.Mainnet {
		myHost.GetP2PHost().Network().Notify(utils.NewConnLogger(utils.GetLogInstance()))
	}
	if err != nil {
		panic("unable to new host in harmony")
	}

	nodeConfig.DBDir = *dbDir

	return nodeConfig
}

func setupConsensusAndNode(nodeConfig *nodeconfig.ConfigType) *node.Node {
	// Consensus object.
	// TODO: consensus object shouldn't start here
	// TODO(minhdoan): During refactoring, found out that the peers list is actually empty. Need to clean up the logic of consensus later.
	decider := quorum.NewDecider(quorum.SuperMajorityVote)
	currentConsensus, err := consensus.New(
		myHost, nodeConfig.ShardID, p2p.Peer{}, nodeConfig.ConsensusPriKey, decider,
	)
	currentConsensus.SelfAddress = common.ParseAddr(initialAccount.Address)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error :%v \n", err)
		os.Exit(1)
	}
	commitDelay, err := time.ParseDuration(*delayCommit)
	if err != nil || commitDelay < 0 {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR invalid commit delay %#v", *delayCommit)
		os.Exit(1)
	}
	currentConsensus.SetCommitDelay(commitDelay)
	currentConsensus.MinPeers = *minPeers

	if *disableViewChange {
		currentConsensus.DisableViewChangeForTestingOnly()
	}

	// Current node.
	chainDBFactory := &shardchain.LDBFactory{RootDir: nodeConfig.DBDir}
	currentNode := node.New(myHost, currentConsensus, chainDBFactory, *isArchival)

	switch {
	case *networkType == nodeconfig.Localnet:
		epochConfig := core.ShardingSchedule.InstanceForEpoch(ethCommon.Big0)
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
		currentNode.SyncingPeerProvider = node.NewDNSSyncingPeerProvider(*dnsZone, syncing.GetSyncingPort(*port))
	case *dnsFlag:
		currentNode.SyncingPeerProvider = node.NewDNSSyncingPeerProvider("t.hmny.io", syncing.GetSyncingPort(*port))
	default:
		currentNode.SyncingPeerProvider = node.NewLegacySyncingPeerProvider(currentNode)

	}

	// TODO: refactor the creation of blockchain out of node.New()
	currentConsensus.ChainReader = currentNode.Blockchain()

	// Set up prometheus pushgateway for metrics monitoring serivce.
	currentNode.NodeConfig.SetPushgatewayIP(nodeConfig.PushgatewayIP)
	currentNode.NodeConfig.SetPushgatewayPort(nodeConfig.PushgatewayPort)
	currentNode.NodeConfig.SetMetricsFlag(nodeConfig.MetricsFlag)

	currentNode.NodeConfig.SetBeaconGroupID(nodeconfig.NewGroupIDByShardID(0))

	switch *nodeType {
	case "explorer":
		currentNode.NodeConfig.SetRole(nodeconfig.ExplorerNode)
		currentNode.NodeConfig.SetShardGroupID(nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(*shardID)))
		currentNode.NodeConfig.SetClientGroupID(nodeconfig.NewClientGroupIDByShardID(nodeconfig.ShardID(*shardID)))
	case "validator":
		currentNode.NodeConfig.SetRole(nodeconfig.Validator)
		if nodeConfig.ShardID == 0 {
			currentNode.NodeConfig.SetShardGroupID(nodeconfig.NewGroupIDByShardID(0))
			currentNode.NodeConfig.SetClientGroupID(nodeconfig.NewClientGroupIDByShardID(0))
		} else {
			currentNode.NodeConfig.SetShardGroupID(nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(nodeConfig.ShardID)))
			currentNode.NodeConfig.SetClientGroupID(nodeconfig.NewClientGroupIDByShardID(nodeconfig.ShardID(nodeConfig.ShardID)))
		}
	}
	currentNode.NodeConfig.ConsensusPubKey = nodeConfig.ConsensusPubKey
	currentNode.NodeConfig.ConsensusPriKey = nodeConfig.ConsensusPriKey

	// Setup block period for currentNode.
	currentNode.BlockPeriod = time.Duration(*blockPeriod) * time.Second

	// TODO: Disable drand. Currently drand isn't functioning but we want to compeletely turn it off for full protection.
	// Enable it back after mainnet.
	// dRand := drand.New(nodeConfig.Host, nodeConfig.ShardID, []p2p.Peer{}, nodeConfig.Leader, currentNode.ConfirmedBlockChannel, nodeConfig.ConsensusPriKey)
	// currentNode.Consensus.RegisterPRndChannel(dRand.PRndChannel)
	// currentNode.Consensus.RegisterRndChannel(dRand.RndChannel)
	// currentNode.DRand = dRand

	// This needs to be executed after consensus and drand are setup
	if err := currentNode.CalculateInitShardState(); err != nil {
		ctxerror.Crit(utils.GetLogger(), err, "CalculateInitShardState failed",
			"shardID", *shardID)
	}

	// Set the consensus ID to be the current block number
	viewID := currentNode.Blockchain().CurrentBlock().Header().ViewID().Uint64()
	currentConsensus.SetViewID(viewID)
	utils.Logger().Info().
		Uint64("viewID", viewID).
		Msg("Init Blockchain")

	// Assign closure functions to the consensus object
	currentConsensus.BlockVerifier = currentNode.VerifyNewBlock
	currentConsensus.OnConsensusDone = currentNode.PostConsensusProcessing
	currentNode.State = node.NodeWaitToJoin

	// update consensus information based on the blockchain
	mode := currentConsensus.UpdateConsensusInformation()
	currentConsensus.SetMode(mode)

	// Watching currentNode and currentConsensus.
	memprofiling.GetMemProfiling().Add("currentNode", currentNode)
	memprofiling.GetMemProfiling().Add("currentConsensus", currentConsensus)
	return currentNode
}

func main() {
	flag.Var(&utils.BootNodes, "bootnodes", "a list of bootnode multiaddress (delimited by ,)")
	flag.Parse()

	switch *nodeType {
	case "validator":
	case "explorer":
		break
	default:
		fmt.Fprintf(os.Stderr, "Unknown node type: %s\n", *nodeType)
		os.Exit(1)
	}

	nodeconfig.SetPublicRPC(*publicRPC)
	nodeconfig.SetVersion(fmt.Sprintf("Harmony (C) 2019. %v, version %v-%v (%v %v)", path.Base(os.Args[0]), version, commit, builtBy, builtAt))
	if *versionFlag {
		printVersion()
	}

	switch *networkType {
	case nodeconfig.Mainnet:
		core.ShardingSchedule = shardingconfig.MainnetSchedule
	case nodeconfig.Testnet:
		core.ShardingSchedule = shardingconfig.TestnetSchedule
	case nodeconfig.Pangaea:
		core.ShardingSchedule = shardingconfig.PangaeaSchedule
	case nodeconfig.Localnet:
		core.ShardingSchedule = shardingconfig.LocalnetSchedule
	case nodeconfig.Devnet:
		if *devnetHarmonySize < 0 {
			*devnetHarmonySize = *devnetShardSize
		}
		// TODO (leo): use a passing list of accounts here
		devnetConfig, err := shardingconfig.NewInstance(
			uint32(*devnetNumShards), *devnetShardSize, *devnetHarmonySize, genesis.HarmonyAccounts, genesis.FoundationalNodeAccounts, nil)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR invalid devnet sharding config: %s",
				err)
			os.Exit(1)
		}
		core.ShardingSchedule = shardingconfig.NewFixedSchedule(devnetConfig)
	}

	initSetup()

	// Set up manual call for garbage collection.
	if *enableGC {
		memprofiling.MaybeCallGCPeriodically()
	}

	if *nodeType == "validator" {
		setupInitialAccount()
	}

	if *shardID >= 0 {
		utils.Logger().Info().
			Uint32("original", initialAccount.ShardID).
			Int("override", *shardID).
			Msg("ShardID Override")
		initialAccount.ShardID = uint32(*shardID)
	}

	nodeConfig := createGlobalConfig()
	currentNode := setupConsensusAndNode(nodeConfig)
	//setup state syncing and beacon syncing frequency
	currentNode.SetSyncFreq(*syncFreq)
	currentNode.SetBeaconSyncFreq(*beaconSyncFreq)

	if nodeConfig.ShardID != 0 && currentNode.NodeConfig.Role() != nodeconfig.ExplorerNode {
		utils.GetLogInstance().Info("SupportBeaconSyncing", "shardID", currentNode.Blockchain().ShardID(), "shardID", nodeConfig.ShardID)
		go currentNode.SupportBeaconSyncing()
	}

	startMsg := "==== New Harmony Node ===="
	if *nodeType == "explorer" {
		startMsg = "==== New Explorer Node ===="
	}

	utils.Logger().Info().
		Str("BlsPubKey", hex.EncodeToString(nodeConfig.ConsensusPubKey.Serialize())).
		Uint32("ShardID", nodeConfig.ShardID).
		Str("ShardGroupID", nodeConfig.GetShardGroupID().String()).
		Str("BeaconGroupID", nodeConfig.GetBeaconGroupID().String()).
		Str("ClientGroupID", nodeConfig.GetClientGroupID().String()).
		Str("Role", currentNode.NodeConfig.Role().String()).
		Str("multiaddress", fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, myHost.GetID().Pretty())).
		Msg(startMsg)

	if *enableMemProfiling {
		memprofiling.GetMemProfiling().Start()
	}
	go currentNode.SupportSyncing()
	currentNode.ServiceManagerSetup()

	currentNode.RunServices()
	// RPC for SDK not supported for mainnet.
	if err := currentNode.StartRPC(*port); err != nil {
		ctxerror.Warn(utils.GetLogger(), err, "StartRPC failed")
	}

	// Run additional node collectors
	// Collect node metrics if metrics flag is set
	if currentNode.NodeConfig.GetMetricsFlag() {
		go currentNode.CollectMetrics()
	}
	// Commit committtee if node role is explorer
	if currentNode.NodeConfig.Role() == nodeconfig.ExplorerNode {
		go currentNode.CommitCommittee()
	}

	currentNode.StartServer()
}
