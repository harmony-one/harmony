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
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

	golog "github.com/ipfs/go-log"
	gologging "github.com/whyrusleeping/go-logging"
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

// InitLDBDatabase initializes a LDBDatabase. will return the beacon chain database for normal shard nodes
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
	pprof            = flag.String("pprof", "", "what address and port the pprof profiling server should listen on")
	versionFlag      = flag.Bool("version", false, "Output version info")
	onlyLogTps       = flag.Bool("only_log_tps", false, "Only log TPS if true")
	dnsZone          = flag.String("dns_zone", "", "if given and not empty, use peers from the zone (default: use libp2p peer discovery instead)")
	dnsFlag          = flag.Bool("dns", true, "[deprecated] equivalent to -dns_zone t.hmny.io")
	//Leader needs to have a minimal number of peers to start consensus
	minPeers = flag.Int("min_peers", 32, "Minimal number of Peers in shard")
	// Key file to store the private key
	keyFile = flag.String("key", "./.hmykey", "the p2p key file of the harmony node")
	// isArchival indicates this node is an archival node that will save and archive current blockchain
	isArchival = flag.Bool("is_archival", false, "false will enable cached state pruning")
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
	cmkEncryptedBLSKey = flag.String("aws_blskey", "", "The aws CMK encrypted bls private key file.")
	enableGC           = flag.Bool("enableGC", true, "Enable calling garbage collector manually .")
	blsKeyFile         = flag.String("blskey_file", "", "The encrypted file of bls serialized private key by passphrase.")
	blsFolder          = flag.String("blsfolder", ".hmy/blskeys", "The folder that stores the bls keys; same blspass is used to decrypt all bls keys; all bls keys mapped to same shard")

	blsPass       = flag.String("blspass", "", "The file containing passphrase to decrypt the encrypted bls file.")
	blsPassphrase string

	// Sharding configuration parameters for devnet
	devnetNumShards   = flag.Uint("dn_num_shards", 2, "number of shards for -network_type=devnet (default: 2)")
	devnetShardSize   = flag.Int("dn_shard_size", 10, "number of nodes per shard for -network_type=devnet (default 10)")
	devnetHarmonySize = flag.Int("dn_hmy_size", -1, "number of Harmony-operated nodes per shard for -network_type=devnet; negative (default) means equal to -dn_shard_size")

	// logConn logs incoming/outgoing connections
	logConn = flag.Bool("log_conn", false, "log incoming/outgoing connections")
	// Use a separate log file to log libp2p traces
	logP2P = flag.Bool("log_p2p", false, "log libp2p debug info")

	keystoreDir = flag.String("keystore", hmykey.DefaultKeyStoreDir, "The default keystore directory")

	initialAccounts = make([]*genesis.DeployAccount, 0)

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
	// Bad block revert
	doRevertBefore = flag.Int("do_revert_before", -1, "If the current block is less than do_revert_before, revert all blocks until (including) revert_to block")
	revertTo       = flag.Int("revert_to", -1, "The revert will rollback all blocks until and including block number revert_to")
	// Blacklist of addresses
	blacklistPath = flag.String("blacklist", "./.hmy/blacklist.txt", "Path to newline delimited file of blacklisted wallet addresses")

	// aws credentials
	awsSettingString = ""
)

func initSetup() {

	// Setup pprof
	if addr := *pprof; addr != "" {
		go func() { http.ListenAndServe(addr, nil) }()
	}

	// maybe request passphrase for bls key.
	if *cmkEncryptedBLSKey == "" {
		passphraseForBls()
	} else {
		// Get aws credentials from stdin prompt
		awsSettingString, _ = blsgen.Readln(1 * time.Second)
	}

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

	if *blsKeyFile == "" && *blsFolder == "" {
		fmt.Println("blskey_file or blsfolder option must be provided")
		os.Exit(101)
	}
	if *blsPass == "" {
		fmt.Println("Internal nodes need to have blspass to decrypt blskey")
		os.Exit(101)
	}
	passphrase, err := utils.GetPassphraseFromSource(*blsPass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR when reading passphrase file: %v\n", err)
		os.Exit(100)
	}
	blsPassphrase = passphrase
}

func findAccountsByPubKeys(config shardingconfig.Instance, pubKeys []*bls.PublicKey) {
	for _, key := range pubKeys {
		keyStr := key.SerializeToHexStr()
		_, account := config.FindAccount(keyStr)
		if account != nil {
			initialAccounts = append(initialAccounts, account)
		}
	}
}

func setupInitialAccounts() {
	genesisShardingConfig := core.ShardingSchedule.InstanceForEpoch(big.NewInt(core.GenesisEpoch))
	multiBlsPubKey := setupConsensusKey(nodeconfig.GetDefaultConfig())

	reshardingEpoch := genesisShardingConfig.ReshardingEpoch()
	if reshardingEpoch != nil && len(reshardingEpoch) > 0 {
		for _, epoch := range reshardingEpoch {
			config := core.ShardingSchedule.InstanceForEpoch(epoch)
			findAccountsByPubKeys(config, multiBlsPubKey.PublicKey)
			if len(initialAccounts) != 0 {
				break
			}
		}
	} else {
		findAccountsByPubKeys(genesisShardingConfig, multiBlsPubKey.PublicKey)
	}

	if len(initialAccounts) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR cannot find your BLS key in the genesis/FN tables: %s\n", multiBlsPubKey.SerializeToHexStr())
		os.Exit(100)
	}

	for _, account := range initialAccounts {
		fmt.Printf("My Genesis Account: %v\n", *account)
	}
}

func readMultiBlsKeys(consensusMultiBlsPriKey *nodeconfig.MultiBlsPrivateKey, consensusMultiBlsPubKey *nodeconfig.MultiBlsPublicKey) error {
	multiBlsKeyDir := blsFolder
	blsKeyFiles, err := ioutil.ReadDir(*multiBlsKeyDir)
	if err != nil {
		return err
	}

	for _, blsKeyFile := range blsKeyFiles {
		var consensusPriKey *bls.SecretKey
		var err error

		blsKeyFilePath := path.Join(*multiBlsKeyDir, blsKeyFile.Name())
		switch filepath.Ext(blsKeyFile.Name()) {
		case ".key":
			// uses the same bls passphrase for multiple bls keys
			consensusPriKey, err = blsgen.LoadBlsKeyWithPassPhrase(blsKeyFilePath, blsPassphrase)
		case ".bls":
			consensusPriKey, err = blsgen.LoadAwsCMKEncryptedBLSKey(blsKeyFilePath, awsSettingString)
		case ".pass":
			utils.Logger().Info().
				Str("file", blsKeyFile.Name()).
				Msg("pass file found in blskey directory")
		default:
			utils.Logger().Warn().
				Str("file", blsKeyFile.Name()).
				Msg("irrelevant file found in blskey directory")
		}
		if err != nil {
			return err
		}
		nodeconfig.AppendPriKey(consensusMultiBlsPriKey, consensusPriKey)
		nodeconfig.AppendPubKey(consensusMultiBlsPubKey, consensusPriKey.GetPublicKey())
	}

	return nil
}

func setupConsensusKey(nodeConfig *nodeconfig.ConfigType) nodeconfig.MultiBlsPublicKey {
	var consensusMultiPriKey nodeconfig.MultiBlsPrivateKey
	var consensusMultiPubKey nodeconfig.MultiBlsPublicKey

	if *blsKeyFile != "" {
		consensusPriKey, err := blsgen.LoadBlsKeyWithPassPhrase(*blsKeyFile, blsPassphrase)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR when loading bls key, err :%v\n", err)
			os.Exit(100)
		}
		nodeconfig.AppendPriKey(&consensusMultiPriKey, consensusPriKey)
		nodeconfig.AppendPubKey(&consensusMultiPubKey, consensusPriKey.GetPublicKey())
	} else if *cmkEncryptedBLSKey != "" {
		consensusPriKey, err := blsgen.LoadAwsCMKEncryptedBLSKey(*cmkEncryptedBLSKey, awsSettingString)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR when loading aws CMK encrypted bls key, err :%v\n", err)
			os.Exit(100)
		}

		nodeconfig.AppendPriKey(&consensusMultiPriKey, consensusPriKey)
		nodeconfig.AppendPubKey(&consensusMultiPubKey, consensusPriKey.GetPublicKey())
	} else {
		err := readMultiBlsKeys(&consensusMultiPriKey, &consensusMultiPubKey)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR when loading bls keys, err :%v\n", err)
			os.Exit(100)
		}
	}

	// Consensus keys are the BLS12-381 keys used to sign consensus messages
	nodeConfig.ConsensusPriKey = &consensusMultiPriKey
	nodeConfig.ConsensusPubKey = &consensusMultiPubKey

	return consensusMultiPubKey
}

func createGlobalConfig() *nodeconfig.ConfigType {
	var err error

	if len(initialAccounts) == 0 {
		initialAccounts = append(initialAccounts, &genesis.DeployAccount{ShardID: uint32(*shardID)})
	}
	nodeConfig := nodeconfig.GetShardConfig(initialAccounts[0].ShardID) // assuming all accounts are on same shard
	if *nodeType == "validator" {
		// Set up consensus keys.
		setupConsensusKey(nodeConfig)
	} else {
		// set dummy bls key for consensus object
		nodeConfig.ConsensusPriKey = nodeconfig.GetMultiBlsPrivateKey(&bls.SecretKey{})
		nodeConfig.ConsensusPubKey = nodeconfig.GetMultiBlsPublicKey(&bls.PublicKey{})
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
	nodeConfig.SetArchival(*isArchival)

	// P2p private key is used for secure message transfer between p2p nodes.
	nodeConfig.P2pPriKey, _, err = utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}
	// TODO: should self peer have multi-bls key?
	selfPeer := p2p.Peer{IP: *ip, Port: *port, ConsensusPubKey: nodeConfig.ConsensusPubKey.PublicKey[0]}

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

	currentConsensus.SelfAddress = make(map[string]ethCommon.Address)
	for _, initialAccount := range initialAccounts {
		currentConsensus.SelfAddress[initialAccount.BlsPublicKey] = common.ParseAddr(initialAccount.Address)
	}

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

	blacklist, err := setupBlacklist()
	if err != nil {
		utils.Logger().Warn().Msgf("Blacklist setup error: %s", err.Error())
	}

	// Current node.
	chainDBFactory := &shardchain.LDBFactory{RootDir: nodeConfig.DBDir}
	currentNode := node.New(myHost, currentConsensus, chainDBFactory, blacklist, *isArchival)

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
	// TODO: add staking support
	// currentNode.StakingAccount = myAccount
	utils.Logger().Info().
		Str("address", common.MustAddressToBech32(currentNode.StakingAccount.Address)).
		Msg("node account set")

	// TODO: refactor the creation of blockchain out of node.New()
	currentConsensus.ChainReader = currentNode.Blockchain()

	// Set up prometheus pushgateway for metrics monitoring serivce.
	currentNode.NodeConfig.SetPushgatewayIP(nodeConfig.PushgatewayIP)
	currentNode.NodeConfig.SetPushgatewayPort(nodeConfig.PushgatewayPort)
	currentNode.NodeConfig.SetMetricsFlag(nodeConfig.MetricsFlag)

	currentNode.NodeConfig.SetBeaconGroupID(nodeconfig.NewGroupIDByShardID(0))

	nodeconfig.GetDefaultConfig().DBDir = nodeConfig.DBDir
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
	currentConsensus.SetViewID(viewID + 1)
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

func setupBlacklist() (*map[ethCommon.Address]struct{}, error) {
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
	return &addrMap, nil
}

func main() {
	// HACK Force usage of go implementation rather than the C based one. Do the right way, see the
	// notes one line 66,67 of https://golang.org/src/net/net.go that say can make the decision at
	// build time.
	os.Setenv("GODEBUG", "netdns=go")

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
		setupInitialAccounts()
	}

	for _, initialAccount := range initialAccounts {
		if *shardID >= 0 {
			utils.Logger().Info().
				Uint32("original", initialAccount.ShardID).
				Int("override", *shardID).
				Msg("ShardID Override")
			initialAccount.ShardID = uint32(*shardID)
		}
	}

	nodeConfig := createGlobalConfig()
	currentNode := setupConsensusAndNode(nodeConfig)

	// Prepare for graceful shutdown from os signals
	osSignal := make(chan os.Signal)
	signal.Notify(osSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			select {
			case sig := <-osSignal:
				if sig == syscall.SIGTERM || sig == os.Interrupt {
					msg := "Got %s signal. Gracefully shutting down...\n"
					utils.Logger().Printf(msg, sig)
					fmt.Printf(msg, sig)
					currentNode.ShutDown()
				}
			}
		}
	}()

	//setup state syncing and beacon syncing frequency
	currentNode.SetSyncFreq(*syncFreq)
	currentNode.SetBeaconSyncFreq(*beaconSyncFreq)

	if nodeConfig.ShardID != 0 && currentNode.NodeConfig.Role() != nodeconfig.ExplorerNode {
		utils.GetLogInstance().Info("SupportBeaconSyncing", "shardID", currentNode.Blockchain().ShardID(), "shardID", nodeConfig.ShardID)
		go currentNode.SupportBeaconSyncing()
	}

	////// Code only used for one-off rollback /////////
	chain := currentNode.Blockchain()
	curNum := chain.CurrentBlock().NumberU64()
	if curNum < uint64(*doRevertBefore) && curNum >= uint64(*revertTo) {
		// Remove invalid blocks
		for chain.CurrentBlock().NumberU64() >= uint64(*revertTo) {
			curBlock := chain.CurrentBlock()
			rollbacks := []ethCommon.Hash{curBlock.Hash()}
			utils.Logger().Info().Msgf("Rolling back block %d", chain.CurrentBlock().NumberU64())
			chain.Rollback(rollbacks)
			lastSig := curBlock.Header().LastCommitSignature()
			sigAndBitMap := append(lastSig[:], curBlock.Header().LastCommitBitmap()...)
			chain.WriteLastCommits(sigAndBitMap)
		}
	}
	///////////////////////////////////////////////

	startMsg := "==== New Harmony Node ===="
	if *nodeType == "explorer" {
		startMsg = "==== New Explorer Node ===="
	}

	utils.Logger().Info().
		Str("BlsPubKey", nodeConfig.ConsensusPubKey.SerializeToHexStr()).
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

	if *logP2P {
		f, err := os.OpenFile(path.Join(*logFolder, "libp2p.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open libp2p.log. %v\n", err)
		} else {
			defer f.Close()
			backend1 := gologging.NewLogBackend(f, "", 0)
			gologging.SetBackend(backend1)
			golog.SetAllLoggers(gologging.DEBUG) // Change to DEBUG for extra info
		}
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

	currentNode.StartServer()
}
