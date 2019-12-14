package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/blsgen"
	"github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/genesis"
	hmykey "github.com/harmony-one/harmony/internal/keystore"
	"github.com/harmony-one/harmony/internal/memprofiling"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/libp2pctl"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	p2putils "github.com/harmony-one/harmony/p2p/utils"
	"github.com/harmony-one/harmony/shard"
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

func printVersion() {
	_, _ = fmt.Fprintln(os.Stderr, nodeconfig.GetVersion())
	os.Exit(0)
}

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
	// staking indicates whether the node is operating in staking mode.
	stakingFlag = flag.Bool("staking", false, "whether the node should operate in staking mode")
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
	logConn        = flag.Bool("log_conn", false, "log incoming/outgoing connections")
	keystoreDir    = flag.String("keystore", hmykey.DefaultKeyStoreDir, "The default keystore directory")
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
	publicRPC       = flag.Bool("public_rpc", false, "Enable Public RPC Access (default: false)")
	// Bad block revert
	doRevertBefore = flag.Int("do_revert_before", 0, "If the current block is less than do_revert_before, revert all blocks until (including) revert_to block")
	revertTo       = flag.Int("revert_to", 0, "The revert will rollback all blocks until and including block number revert_to")
	revertBeacon   = flag.Bool("revert_beacon", false, "Whether to revert beacon chain or the chain this node is assigned to")
	// libp2p control interface
	libp2pctlFlag         = flag.Bool("libp2pctl", false, "Enable libp2p control interface")
	libp2pctlPortFlag     = flag.String("libp2pctl_port", "-4000", "libp2pctl port number; if prefixed with +/-, treat as offset from the -port value (default: -5000)")
	libp2pctlCertFlag     = flag.String("libp2pctl_cert", "harmony-libp2pctl-cert.pem", "libp2pctl HTTPS certificate filename (default: harmony-libp2pctl.crt)")
	libp2pctlKeyFlag      = flag.String("libp2pctl_key", "harmony-libp2pctl-key.pem", "libp2pctl HTTPS private key filename (default: harmony-libp2pctl.key)")
	libp2pctlClientCAFlag = flag.String("libp2pct_client_ca", "harmony-libp2pctl-client-ca.pem", "libp2pctl HTTPS client CA (default: harmony-libp2pctl-ca.pem)")
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

	// Set sharding schedule
	nodeconfig.SetShardingSchedule(shard.Schedule)

	// Setup mem profiling.
	memprofiling.GetMemProfiling().Config()

	// Set default keystore Dir
	hmykey.DefaultKeyStoreDir = *keystoreDir

	// Set up randomization seed.
	rand.Seed(int64(time.Now().Nanosecond()))

	if len(p2putils.BootNodes) == 0 {
		bootNodeAddrs, err := p2putils.StringsToAddrs(p2putils.DefaultBootNodeAddrStrings)
		if err != nil {
			utils.FatalErrMsg(err, "cannot parse default bootnode list %#v",
				p2putils.DefaultBootNodeAddrStrings)
		}
		p2putils.BootNodes = bootNodeAddrs
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
		_, _ = fmt.Fprintf(os.Stderr, "ERROR when reading passphrase file: %v\n", err)
		os.Exit(100)
	}
	blsPassphrase = passphrase
}

func setupLegacyNodeAccount() error {
	genesisShardingConfig := shard.Schedule.InstanceForEpoch(big.NewInt(core.GenesisEpoch))
	pubKey := setupConsensusKey(nodeconfig.GetDefaultConfig())

	reshardingEpoch := genesisShardingConfig.ReshardingEpoch()
	if reshardingEpoch != nil && len(reshardingEpoch) > 0 {
		for _, epoch := range reshardingEpoch {
			config := shard.Schedule.InstanceForEpoch(epoch)
			_, initialAccount = config.FindAccount(pubKey.SerializeToHexStr())
			if initialAccount != nil {
				break
			}
		}
	} else {
		_, initialAccount = genesisShardingConfig.FindAccount(pubKey.SerializeToHexStr())
	}

	if initialAccount == nil {
		return errors.Errorf("cannot find key %s in table", pubKey.SerializeToHexStr())
	}
	fmt.Printf("My Genesis Account: %v\n", *initialAccount)
	return nil
}

func setupStakingNodeAccount() error {
	pubKey := setupConsensusKey(nodeconfig.GetDefaultConfig())
	shardID, err := nodeconfig.GetDefaultConfig().ShardIDFromConsensusKey()
	if err != nil {
		return errors.Wrap(err, "cannot determine shard to join")
	}
	initialAccount = &genesis.DeployAccount{}
	initialAccount.ShardID = shardID
	initialAccount.BlsPublicKey = pubKey.SerializeToHexStr()
	initialAccount.Address = ""
	return nil
}

func setupConsensusKey(nodeConfig *nodeconfig.ConfigType) *bls.PublicKey {
	consensusPriKey, err := blsgen.LoadBlsKeyWithPassPhrase(*blsKeyFile, blsPassphrase)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR when loading bls key, err :%v\n", err)
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

func createGlobalConfig() (*nodeconfig.ConfigType, error) {
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
	nodeconfig.SetNetworkType(netType) // sets for both global and shard configs
	nodeConfig.SetPushgatewayIP(*pushgatewayIP)
	nodeConfig.SetPushgatewayPort(*pushgatewayPort)
	nodeConfig.SetMetricsFlag(*metricsFlag)

	// P2p private key is used for secure message transfer between p2p nodes.
	nodeConfig.P2pPriKey, _, err = utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load or create P2P key at %#v",
			*keyFile)
	}

	selfPeer := p2p.Peer{IP: *ip, Port: *port, ConsensusPubKey: nodeConfig.ConsensusPubKey}

	myHost, err = p2pimpl.NewHost(&selfPeer, nodeConfig.P2pPriKey)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create P2P network host")
	}
	if *logConn && nodeConfig.GetNetworkType() != nodeconfig.Mainnet {
		myHost.GetP2PHost().Network().Notify(utils.NewConnLogger(utils.GetLogger()))
	}

	nodeConfig.DBDir = *dbDir

	return nodeConfig, nil
}

func setupConsensusAndNode(nodeConfig *nodeconfig.ConfigType) *node.Node {
	// Consensus object.
	// TODO: consensus object shouldn't start here
	// TODO(minhdoan): During refactoring, found out that the peers list is actually empty. Need to clean up the logic of consensus later.
	decider := quorum.NewDecider(quorum.SuperMajorityVote)

	currentConsensus, err := consensus.New(
		myHost, nodeConfig.ShardID, p2p.Peer{}, nodeConfig.ConsensusPriKey, decider,
	)
	currentConsensus.Decider.SetShardIDProvider(func() (uint32, error) {
		return currentConsensus.ShardID, nil
	})
	currentConsensus.Decider.SetMyPublicKeyProvider(func() (*bls.PublicKey, error) {
		return currentConsensus.PubKey, nil
	})

	if initialAccount.Address != "" { // staking validator doesn't have to specify ECDSA address
		currentConsensus.SelfAddress = common.ParseAddr(initialAccount.Address)
	}

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

	if *disableViewChange {
		currentConsensus.DisableViewChangeForTestingOnly()
	}

	// Current node.
	chainDBFactory := &shardchain.LDBFactory{RootDir: nodeConfig.DBDir}
	currentNode := node.New(myHost, currentConsensus, chainDBFactory, *isArchival)

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
		if nodeConfig.ShardID == shard.BeaconChainShardID {
			currentNode.NodeConfig.SetShardGroupID(nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID))
			currentNode.NodeConfig.SetClientGroupID(nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID))
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
	// HACK Force usage of go implementation rather than the C based one. Do the right way, see the
	// notes one line 66,67 of https://golang.org/src/net/net.go that say can make the decision at
	// build time.
	os.Setenv("GODEBUG", "netdns=go")

	flag.Var(&p2putils.BootNodes, "bootnodes", "a list of bootnode multiaddress (delimited by ,)")
	flag.Parse()

	switch *nodeType {
	case "validator":
	case "explorer":
		break
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown node type: %s\n", *nodeType)
		os.Exit(1)
	}

	nodeconfig.SetPublicRPC(*publicRPC)
	nodeconfig.SetVersion(fmt.Sprintf("Harmony (C) 2019. %v, version %v-%v (%v %v)", path.Base(os.Args[0]), version, commit, builtBy, builtAt))
	if *versionFlag {
		printVersion()
	}

	switch *networkType {
	case nodeconfig.Mainnet:
		shard.Schedule = shardingconfig.MainnetSchedule
	case nodeconfig.Testnet:
		shard.Schedule = shardingconfig.TestnetSchedule
	case nodeconfig.Pangaea:
		shard.Schedule = shardingconfig.PangaeaSchedule
	case nodeconfig.Localnet:
		shard.Schedule = shardingconfig.LocalnetSchedule
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
		shard.Schedule = shardingconfig.NewFixedSchedule(devnetConfig)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "invalid network type: %#v\n", *networkType)
		os.Exit(2)
	}

	initSetup()

	// Set up manual call for garbage collection.
	if *enableGC {
		memprofiling.MaybeCallGCPeriodically()
	}

	if *nodeType == "validator" {
		var err error
		if *stakingFlag {
			err = setupStakingNodeAccount()
		} else {
			err = setupLegacyNodeAccount()
		}
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "cannot set up node account: %s\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("%s mode; node key %s -> shard %d\n",
		map[bool]string{false: "Legacy", true: "Staking"}[*stakingFlag],
		nodeconfig.GetDefaultConfig().ConsensusPubKey.SerializeToHexStr(),
		initialAccount.ShardID)

	if *nodeType != "validator" && *shardID >= 0 {
		utils.Logger().Info().
			Uint32("original", initialAccount.ShardID).
			Int("override", *shardID).
			Msg("ShardID Override")
		initialAccount.ShardID = uint32(*shardID)
	}

	nodeConfig, err := createGlobalConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR cannot configure node: %s\n", err)
		os.Exit(1)
	}
	currentNode := setupConsensusAndNode(nodeConfig)
	//setup state syncing and beacon syncing frequency
	currentNode.SetSyncFreq(*syncFreq)
	currentNode.SetBeaconSyncFreq(*beaconSyncFreq)

	if nodeConfig.ShardID != shard.BeaconChainShardID && currentNode.NodeConfig.Role() != nodeconfig.ExplorerNode {
		utils.Logger().Info().Uint32("shardID", currentNode.Blockchain().ShardID()).Uint32("shardID", nodeConfig.ShardID).Msg("SupportBeaconSyncing")
		go currentNode.SupportBeaconSyncing()
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
				chain.WriteLastCommits(sigAndBitMap)
			}
		}
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
		utils.Logger().Warn().
			Err(err).
			Msg("StartRPC failed")
	}

	if *libp2pctlFlag {
		port, err := getPort(*libp2pctlPortFlag)
		if err != nil {
			utils.FatalErrMsg(err, "cannot parse -libp2pctl_port %#v",
				*libp2pctlPortFlag)
		}
		cert, err := tls.LoadX509KeyPair(*libp2pctlCertFlag, *libp2pctlKeyFlag)
		if err != nil {
			utils.FatalErrMsg(err, "cannot load libp2pctl TLS cert/key")
		}
		clientCAPEM, err := ioutil.ReadFile(*libp2pctlClientCAFlag)
		if err != nil {
			utils.FatalErrMsg(err, "cannot load libp2pctl client CA %s",
				*libp2pctlClientCAFlag)
		}
		clientCAs := x509.NewCertPool()
		if !clientCAs.AppendCertsFromPEM(clientCAPEM) {
			utils.Fatal("no libp2pctl client CA in %s", *libp2pctlClientCAFlag)
		}
		tlsConfig := tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  clientCAs,
		}
		listenAddr := &net.TCPAddr{Port: port}
		listener, err := net.ListenTCP("tcp", listenAddr)
		if err != nil {
			utils.FatalErrMsg(err, "cannot listen on libp2pctl port %d",
				port)
		}
		tlsListener := tls.NewListener(listener, &tlsConfig)
		ctl := libp2pctl.New(myHost.GetP2PHost())
		go func() { _ = http.Serve(tlsListener, ctl.Handler()) }()
		utils.Logger().Info().Interface("listenAddr", listenAddr).
			Msg("started libp2pctl")
	}

	// Run additional node collectors
	// Collect node metrics if metrics flag is set
	if currentNode.NodeConfig.GetMetricsFlag() {
		go currentNode.CollectMetrics()
	}

	currentNode.StartServer()
}

func getPort(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty port")
	}
	if s[0] != '+' && s[0] != '-' {
		return net.LookupPort("tcp", s)
	}
	n, err := strconv.ParseUint(s[1:], 0, 16)
	if err != nil {
		return 0, err
	}
	num, err := net.LookupPort("tcp", *port)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot parse -port value %#v", *port)
	}
	if s[0] == '+' {
		num += int(n)
	} else {
		num -= int(n)
	}
	if num < 1 || num > 65535 {
		return 0, errors.Wrapf(err, "offset result %v out of range", num)
	}
	return num, nil
}
