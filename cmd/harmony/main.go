package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"runtime"
	"time"

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

// InitLDBDatabase initializes a LDBDatabase. isBeacon=true will return the beacon chain database for normal shard nodes
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
	// isBeacon indicates this node is a beacon chain node
	isBeacon = flag.Bool("is_beacon", false, "true means this node is a beacon chain node")
	// isArchival indicates this node is an archival node that will save and archive current blockchain
	isArchival = flag.Bool("is_archival", false, "true means this node is a archival node")
	//isNewNode indicates this node is a new node
	isNewNode    = flag.Bool("is_newnode", false, "true means this node is a new node")
	accountIndex = flag.Int("account_index", 0, "the index of the staking account to use")
	// isLeader indicates this node is a beacon chain leader node during the bootstrap process
	isLeader = flag.Bool("is_leader", false, "true means this node is a beacon chain leader node")
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
	nodeConfig := nodeconfig.GetGlobalConfig()

	// Currently we hardcode only one shard.
	nodeConfig.ShardID = 0

	// Key Setup ================= [Start]
	// Staking private key is the ecdsa key used for token related transaction signing (especially the staking txs).
	stakingPriKey := ""
	consensusPriKey := &bls.SecretKey{}
	if *isBeacon {
		stakingPriKey = contract.InitialBeaconChainAccounts[*accountIndex].Private
		err := consensusPriKey.SetHexString(contract.InitialBeaconChainBLSAccounts[*accountIndex].Private)
		if err != nil {
			panic(fmt.Errorf("generate key error"))
		}
	} else {
		// TODO: let user specify the ECDSA key
		stakingPriKey = contract.NewNodeAccounts[*accountIndex].Private
		// TODO: use user supplied key
		consensusPriKey.SetByCSPRNG()
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
	if !*isBeacon {
		if nodeConfig.BeaconDB, err = InitLDBDatabase(*ip, *port, *freshDB, true); err != nil {
			panic(err)
		}
	}

	nodeConfig.SelfPeer = p2p.Peer{IP: *ip, Port: *port, ConsensusPubKey: nodeConfig.ConsensusPubKey}
	if *isLeader {
		nodeConfig.StringRole = "leader"
		nodeConfig.Leader = nodeConfig.SelfPeer
	} else if *isArchival {
		nodeConfig.StringRole = "archival"
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

func setupArchivalNode(nodeConfig *nodeconfig.ConfigType) (*node.Node, *nodeconfig.ConfigType) {
	currentNode := node.New(nodeConfig.Host, &consensus.Consensus{ShardID: uint32(0)}, nil)
	currentNode.NodeConfig.SetRole(nodeconfig.ArchivalNode)
	currentNode.AddBeaconChainDatabase(nodeConfig.BeaconDB)
	return currentNode, nodeConfig
}

func setUpConsensusAndNode(nodeConfig *nodeconfig.ConfigType) (*consensus.Consensus, *node.Node) {
	// Consensus object.
	// TODO: consensus object shouldn't start here
	// TODO(minhdoan): During refactoring, found out that the peers list is actually empty. Need to clean up the logic of consensus later.
	consensus := consensus.New(nodeConfig.Host, nodeConfig.ShardID, []p2p.Peer{}, nodeConfig.Leader, nodeConfig.ConsensusPriKey)
	consensus.MinPeers = *minPeers

	// Current node.
	currentNode := node.New(nodeConfig.Host, consensus, nodeConfig.MainDB)
	currentNode.Consensus.OfflinePeers = currentNode.OfflinePeers
	currentNode.NodeConfig.SetRole(nodeconfig.NewNode)
	currentNode.AccountKey = nodeConfig.StakingPriKey

	// TODO: refactor the creation of blockchain out of node.New()
	consensus.ChainReader = currentNode.Blockchain()

	if *isBeacon {
		if nodeConfig.StringRole == "leader" {
			currentNode.NodeConfig.SetRole(nodeconfig.BeaconLeader)
			currentNode.NodeConfig.SetIsLeader(true)
		} else {
			currentNode.NodeConfig.SetRole(nodeconfig.BeaconValidator)
			currentNode.NodeConfig.SetIsLeader(false)
		}
		currentNode.NodeConfig.SetShardGroupID(p2p.GroupIDBeacon)
		currentNode.NodeConfig.SetIsBeacon(true)
	} else {
		currentNode.AddBeaconChainDatabase(nodeConfig.BeaconDB)

		if *isNewNode {
			currentNode.NodeConfig.SetRole(nodeconfig.NewNode)
		} else if nodeConfig.StringRole == "leader" {
			currentNode.NodeConfig.SetRole(nodeconfig.ShardLeader)
			currentNode.NodeConfig.SetIsLeader(true)
		} else {
			currentNode.NodeConfig.SetRole(nodeconfig.ShardValidator)
			currentNode.NodeConfig.SetIsLeader(false)
		}
		currentNode.NodeConfig.SetShardGroupID(p2p.GroupIDUnknown)
		currentNode.NodeConfig.SetIsBeacon(false)
	}

	// Add randomness protocol
	// TODO: enable drand only for beacon chain
	// TODO: put this in a better place other than main.
	// TODO(minhdoan): During refactoring, found out that the peers list is actually empty. Need to clean up the logic of drand later.
	dRand := drand.New(nodeConfig.Host, nodeConfig.ShardID, []p2p.Peer{}, nodeConfig.Leader, currentNode.ConfirmedBlockChannel, *isLeader, nodeConfig.ConsensusPriKey)
	currentNode.Consensus.RegisterPRndChannel(dRand.PRndChannel)
	currentNode.Consensus.RegisterRndChannel(dRand.RndChannel)
	currentNode.DRand = dRand

	// Assign closure functions to the consensus object
	consensus.BlockVerifier = currentNode.VerifyNewBlock
	consensus.OnConsensusDone = currentNode.PostConsensusProcessing
	currentNode.State = node.NodeWaitToJoin
	return consensus, currentNode
}

func main() {
	flag.Var(&utils.BootNodes, "bootnodes", "a list of bootnode multiaddress")
	flag.Parse()

	initSetup()
	var currentNode *node.Node
	var consensus *consensus.Consensus
	nodeConfig := createGlobalConfig()
	if *isArchival {
		currentNode, nodeConfig = setupArchivalNode(nodeConfig)
		loggingInit(*logFolder, nodeConfig.StringRole, *ip, *port, *onlyLogTps)
		go currentNode.DoBeaconSyncing()
	} else {
		// Start Profiler for leader if profile argument is on
		if nodeConfig.StringRole == "leader" && (*profile || *metricsReportURL != "") {
			prof := profiler.GetProfiler()
			prof.Config(nodeConfig.ShardID, *metricsReportURL)
			if *profile {
				prof.Start()
			}
		}
		consensus, currentNode = setUpConsensusAndNode(nodeConfig)
		if consensus.IsLeader {
			go currentNode.SendPongMessage()
		}
		// Init logging.
		loggingInit(*logFolder, nodeConfig.StringRole, *ip, *port, *onlyLogTps)
		go currentNode.SupportSyncing()
	}
	utils.GetLogInstance().Info("New Harmony Node ====", "Role", currentNode.NodeConfig.Role(), "multiaddress", fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, nodeConfig.Host.GetID().Pretty()))
	currentNode.ServiceManagerSetup()
	currentNode.RunServices()
	currentNode.StartServer()
}
