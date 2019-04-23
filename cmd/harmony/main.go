package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/harmony-one/harmony/core"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/drand"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/profiler"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/utils/contract"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
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

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2018. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

func loggingInit(logFolder, role, ip, port string, onlyLogTps bool) {
	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/%s-%v-%v.log", logFolder, role, ip, port)
	h := log.MultiHandler(
		log.StreamHandler(os.Stdout, log.TerminalFormat(false)),
		log.Must.FileHandler(logFileName, log.JSONFormat()), // Log to file
	)
	if onlyLogTps {
		h = log.MatchFilterHandler("msg", "TPS Report", h)
	}
	log.Root().SetHandler(h)
}

var (
	ip               = flag.String("ip", "127.0.0.1", "IP of the node")
	port             = flag.String("port", "9000", "port of the node.")
	logFolder        = flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	freshDB          = flag.Bool("fresh_db", false, "true means the existing disk based db will be removed")
	profile          = flag.Bool("profile", false, "Turn on profiling (CPU, Memory).")
	metricsReportURL = flag.String("metrics_report_url", "", "If set, reports metrics to this URL.")
	versionFlag      = flag.Bool("version", false, "Output version info")
	onlyLogTps       = flag.Bool("only_log_tps", false, "Only log TPS if true")
	//Leader needs to have a minimal number of peers to start consensus
	minPeers = flag.Int("min_peers", 100, "Minimal number of Peers in shard")
	// Key file to store the private key of staking account.
	stakingKeyFile = flag.String("staking_key", "./.stakingkey", "the private key file of the harmony node")
	// Key file to store the private key
	keyFile = flag.String("key", "./.hmykey", "the private key file of the harmony node")
	// isGenesis indicates this node is a genesis node
	isGenesis = flag.Bool("is_genesis", false, "true means this node is a genesis node")
	// isArchival indicates this node is an archival node that will save and archive current blockchain
	isArchival = flag.Bool("is_archival", false, "true means this node is a archival node")
	//isNewNode indicates this node is a new node
	isNewNode    = flag.Bool("is_newnode", false, "true means this node is a new node")
	accountIndex = flag.Int("account_index", 0, "the index of the staking account to use")
	shardID      = flag.Int("shard_id", -1, "the shard ID of this node")
	// logConn logs incoming/outgoing connections
	logConn = flag.Bool("log_conn", false, "log incoming/outgoing connections")
)

func initSetup() {
	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Logging setup
	utils.SetPortAndIP(*port, *ip)

	// Add GOMAXPROCS to achieve max performance.
	runtime.GOMAXPROCS(1024)

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

func createGlobalConfig() *nodeconfig.ConfigType {
	var err error

	nodeConfig := nodeconfig.GetDefaultConfig()

	myShardID := uint32(*accountIndex / core.GenesisShardSize)

	// Specified Shard ID override calculated Shard ID
	if *shardID >= 0 {
		utils.GetLogInstance().Info("ShardID Override", "original", myShardID, "override", *shardID)
		myShardID = uint32(*shardID)
	}

	if !*isNewNode {
		nodeConfig = nodeconfig.GetShardConfig(myShardID)
	} else {
		myShardID = 0 // This should be default value as new node doesn't belong to any shard.
		if *shardID >= 0 {
			utils.GetLogInstance().Info("ShardID Override", "original", myShardID, "override", *shardID)
			myShardID = uint32(*shardID)
			nodeConfig = nodeconfig.GetShardConfig(myShardID)
		}
	}

	// The initial genesis nodes are sequentially put into genesis shards based on their accountIndex
	nodeConfig.ShardID = myShardID

	// Key Setup ================= [Start]
	// Staking private key is the ecdsa key used for token related transaction signing (especially the staking txs).
	stakingPriKey := ""
	consensusPriKey := &bls.SecretKey{}
	if *isGenesis {
		stakingPriKey = contract.GenesisAccounts[*accountIndex].Private
		err := consensusPriKey.SetHexString(contract.GenesisBLSAccounts[*accountIndex].Private)
		if err != nil {
			panic(fmt.Errorf("generate key error"))
		}
	} else {
		// TODO: let user specify the ECDSA key
		stakingPriKey = contract.NewNodeAccounts[*accountIndex].Private
		// TODO: use user supplied key
		err := consensusPriKey.SetHexString(contract.GenesisBLSAccounts[200+*accountIndex].Private) // TODO: use separate bls accounts for this.
		if err != nil {
			panic(fmt.Errorf("generate key error"))
		}
	}
	nodeConfig.StakingPriKey = node.StoreStakingKeyFromFile(*stakingKeyFile, stakingPriKey)

	// P2p private key is used for secure message transfer between p2p nodes.
	nodeConfig.P2pPriKey, _, err = utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	// Consensus keys are the BLS12-381 keys used to sign consensus messages
	nodeConfig.ConsensusPriKey, nodeConfig.ConsensusPubKey = consensusPriKey, consensusPriKey.GetPublicKey()
	if nodeConfig.ConsensusPriKey == nil || nodeConfig.ConsensusPubKey == nil {
		panic(fmt.Errorf("generate key error"))
	}
	// Key Setup ================= [End]

	// Initialize leveldb for main blockchain and beacon.
	if nodeConfig.MainDB, err = InitLDBDatabase(*ip, *port, *freshDB, false); err != nil {
		panic(err)
	}
	if myShardID != 0 {
		if nodeConfig.BeaconDB, err = InitLDBDatabase(*ip, *port, *freshDB, true); err != nil {
			panic(err)
		}
	}

	nodeConfig.SelfPeer = p2p.Peer{IP: *ip, Port: *port, ConsensusPubKey: nodeConfig.ConsensusPubKey}

	if *accountIndex%core.GenesisShardSize == 0 { // The first node in a shard is the leader at genesis
		nodeConfig.StringRole = "leader"
		nodeConfig.Leader = nodeConfig.SelfPeer
	} else {
		nodeConfig.StringRole = "validator"
	}

	nodeConfig.Host, err = p2pimpl.NewHost(&nodeConfig.SelfPeer, nodeConfig.P2pPriKey)
	if *logConn {
		nodeConfig.Host.GetP2PHost().Network().Notify(utils.ConnLogger)
	}
	if err != nil {
		panic("unable to new host in harmony")
	}

	nodeConfig.Host.AddPeer(&nodeConfig.Leader)

	return nodeConfig
}

func setUpConsensusAndNode(nodeConfig *nodeconfig.ConfigType) (*consensus.Consensus, *node.Node) {
	// Consensus object.
	// TODO: consensus object shouldn't start here
	// TODO(minhdoan): During refactoring, found out that the peers list is actually empty. Need to clean up the logic of consensus later.
	consensus, err := consensus.New(nodeConfig.Host, nodeConfig.ShardID, nodeConfig.Leader, nodeConfig.ConsensusPriKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error :%v \n", err)
		os.Exit(1)
	}
	consensus.MinPeers = *minPeers

	// Current node.
	currentNode := node.New(nodeConfig.Host, consensus, nodeConfig.MainDB, *isArchival)
	currentNode.NodeConfig.SetRole(nodeconfig.NewNode)
	currentNode.AccountKey = nodeConfig.StakingPriKey

	// TODO: refactor the creation of blockchain out of node.New()
	consensus.ChainReader = currentNode.Blockchain()

	// TODO: the setup should only based on shard state
	if *isGenesis {
		// TODO: need change config file and use switch instead of complicated "if else" condition
		if nodeConfig.ShardID == 0 { // Beacon chain
			nodeConfig.SetIsBeacon(true)
			if nodeConfig.StringRole == "leader" {
				currentNode.NodeConfig.SetRole(nodeconfig.BeaconLeader)
				currentNode.NodeConfig.SetIsLeader(true)
			} else {
				currentNode.NodeConfig.SetRole(nodeconfig.BeaconValidator)
				currentNode.NodeConfig.SetIsLeader(false)
			}
			currentNode.NodeConfig.SetShardGroupID(p2p.GroupIDBeacon)
			currentNode.NodeConfig.SetClientGroupID(p2p.GroupIDBeaconClient)
		} else {
			if nodeConfig.StringRole == "leader" {
				currentNode.NodeConfig.SetRole(nodeconfig.ShardLeader)
				currentNode.NodeConfig.SetIsLeader(true)
			} else {
				currentNode.NodeConfig.SetRole(nodeconfig.ShardValidator)
				currentNode.NodeConfig.SetIsLeader(false)
			}
			currentNode.NodeConfig.SetShardGroupID(p2p.NewGroupIDByShardID(p2p.ShardID(nodeConfig.ShardID)))
			currentNode.NodeConfig.SetClientGroupID(p2p.NewClientGroupIDByShardID(p2p.ShardID(nodeConfig.ShardID)))
		}
	} else {
		if *isNewNode {
			currentNode.NodeConfig.SetRole(nodeconfig.NewNode)
			currentNode.NodeConfig.SetClientGroupID(p2p.GroupIDBeaconClient)
			currentNode.NodeConfig.SetBeaconGroupID(p2p.GroupIDBeacon)
			if *shardID > -1 {
				// I will be a validator (single leader is fixed for now)
				currentNode.NodeConfig.SetRole(nodeconfig.ShardValidator)
				currentNode.NodeConfig.SetIsLeader(false)
				currentNode.NodeConfig.SetShardGroupID(p2p.NewGroupIDByShardID(p2p.ShardID(nodeConfig.ShardID)))
				currentNode.NodeConfig.SetClientGroupID(p2p.NewClientGroupIDByShardID(p2p.ShardID(nodeConfig.ShardID)))
			}
		} else if nodeConfig.StringRole == "leader" {
			currentNode.NodeConfig.SetRole(nodeconfig.ShardLeader)
			currentNode.NodeConfig.SetIsLeader(true)
			currentNode.NodeConfig.SetShardGroupID(p2p.GroupIDUnknown)
		} else {
			currentNode.NodeConfig.SetRole(nodeconfig.ShardValidator)
			currentNode.NodeConfig.SetIsLeader(false)
			currentNode.NodeConfig.SetShardGroupID(p2p.GroupIDUnknown)
		}
	}
	currentNode.NodeConfig.ConsensusPubKey = nodeConfig.ConsensusPubKey
	currentNode.NodeConfig.ConsensusPriKey = nodeConfig.ConsensusPriKey

	// Add randomness protocol
	// TODO: enable drand only for beacon chain
	// TODO: put this in a better place other than main.
	// TODO(minhdoan): During refactoring, found out that the peers list is actually empty. Need to clean up the logic of drand later.
	dRand := drand.New(nodeConfig.Host, nodeConfig.ShardID, []p2p.Peer{}, nodeConfig.Leader, currentNode.ConfirmedBlockChannel, nodeConfig.ConsensusPriKey)
	currentNode.Consensus.RegisterPRndChannel(dRand.PRndChannel)
	currentNode.Consensus.RegisterRndChannel(dRand.RndChannel)
	currentNode.DRand = dRand

	// TODO: enable beacon chain sync
	// TODO: fix bug that beacon chain override the shard chain in DB.
	if consensus.ShardID != 0 {
		currentNode.AddBeaconChainDatabase(nodeConfig.BeaconDB)
	}

	// This needs to be executed after consensus and drand are setup
	if !*isNewNode || *shardID > -1 { // initial staking new node doesn't need to initialize shard state
		currentNode.InitShardState(*shardID == -1 && !*isNewNode) // TODO: Have a better why to distinguish non-genesis node
	}

	// Set the consensus ID to be the current block number
	height := currentNode.Blockchain().CurrentBlock().NumberU64()

	consensus.SetConsensusID(uint32(height))
	utils.GetLogInstance().Info("Init Blockchain", "height", height)

	// Assign closure functions to the consensus object
	consensus.BlockVerifier = currentNode.VerifyNewBlock
	consensus.OnConsensusDone = currentNode.PostConsensusProcessing
	currentNode.State = node.NodeWaitToJoin
	return consensus, currentNode
}

func main() {
	flag.Var(&utils.BootNodes, "bootnodes", "a list of bootnode multiaddress (delimited by ,)")
	flag.Parse()

	initSetup()
	var currentNode *node.Node
	var consensus *consensus.Consensus
	nodeConfig := createGlobalConfig()

	// Init logging.
	loggingInit(*logFolder, nodeConfig.StringRole, *ip, *port, *onlyLogTps)

	// Start Profiler for leader if profile argument is on
	if nodeConfig.StringRole == "leader" && (*profile || *metricsReportURL != "") {
		prof := profiler.GetProfiler()
		prof.Config(nodeConfig.ShardID, *metricsReportURL)
		if *profile {
			prof.Start()
		}
	}
	consensus, currentNode = setUpConsensusAndNode(nodeConfig)
	// TODO: put this inside discovery service
	if consensus.IsLeader {
		go currentNode.SendPongMessage()
	}
	//if consensus.ShardID != 0 {
	//	go currentNode.SupportBeaconSyncing()
	//}

	utils.GetLogInstance().Info("==== New Harmony Node ====", "BlsPubKey", hex.EncodeToString(nodeConfig.ConsensusPubKey.Serialize()), "ShardID", nodeConfig.ShardID, "ShardGroupID", nodeConfig.GetShardGroupID(), "BeaconGroupID", nodeConfig.GetBeaconGroupID(), "ClientGroupID", nodeConfig.GetClientGroupID(), "Role", currentNode.NodeConfig.Role(), "multiaddress", fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, nodeConfig.Host.GetID().Pretty()))

	go currentNode.SupportSyncing()
	currentNode.ServiceManagerSetup()
	currentNode.RunServices()
	currentNode.StartServer()
}
