package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/consensus"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

var (
	version    string
	builtBy    string
	builtAt    string
	commit     string
	stateMutex sync.Mutex
)
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
	// isNewNode indicates this node is a new node
	isNewNode    = flag.Bool("is_newnode", false, "true means this node is a new node")
	accountIndex = flag.Int("account_index", 0, "the index of the staking account to use")
	// isLeader indicates this node is a beacon chain leader node during the bootstrap process
	isLeader = flag.Bool("is_leader", false, "true means this node is a beacon chain leader node")
	// logConn logs incoming/outgoing connections
	logConn = flag.Bool("log_conn", false, "log incoming/outgoing connections")
)

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2018. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

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

func createArchivalNode() (*node.Node, *nodeconfig.ConfigType) { //Mix of setUpConsensusAndNode and createGlobalConfig
	var err error
	nodeConfig := nodeconfig.GetGlobalConfig()

	// Currently we hardcode only one shard.
	nodeConfig.ShardIDString = "0"
	// Consensus keys are the BLS12-381 keys used to sign consensus messages
	nodeConfig.ConsensusPriKey, nodeConfig.ConsensusPubKey = utils.GenKey(*ip, *port)
	// P2p private key is used for secure message transfer between p2p nodes.
	nodeConfig.P2pPriKey, _, err = utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	// Initialize leveldb for main blockchain and beacon.
	if nodeConfig.MainDB, err = InitLDBDatabase(*ip, *port, *freshDB, false); err != nil {
		panic(err)
	}
	if nodeConfig.BeaconDB, err = InitLDBDatabase(*ip, *port, *freshDB, true); err != nil {
		panic(err)
	}
	nodeConfig.SelfPeer = p2p.Peer{IP: *ip, Port: *port, ValidatorID: -1, ConsensusPubKey: nodeConfig.ConsensusPubKey}
	nodeConfig.StringRole = "Backup"
	nodeConfig.Host, err = p2pimpl.NewHost(&nodeConfig.SelfPeer, nodeConfig.P2pPriKey)
	if *logConn {
		nodeConfig.Host.GetP2PHost().Network().Notify(utils.ConnLogger)
	}
	if err != nil {
		panic("unable to new host in harmony")
	}

	currentNode := node.New(nodeConfig.Host, &consensus.Consensus{ShardID: uint32(10000)}, nodeConfig.BeaconDB) //at the moment the database supplied is beacondb as this is a beacon sync node
	currentNode.NodeConfig.SetRole(nodeconfig.BackupNode)
	currentNode.NodeConfig.SetShardGroupID(p2p.GroupIDBeacon)
	currentNode.AddBeaconChainDatabase(nodeConfig.BeaconDB)
	return currentNode, nodeConfig
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

// The main entrance for the backup/archival node program
func main() {
	flag.Var(&utils.BootNodes, "bootnodes", "a list of bootnode multiaddress")
	flag.Parse()
	initSetup()
	archivalNode, nodeConfig := createArchivalNode()
	fmt.Println(nodeConfig)
	// Init logging.
	loggingInit(*logFolder, "backupNode", *ip, *port, *onlyLogTps)
	log.Info("New Harmony Node ====", "Role", archivalNode.NodeConfig.Role(), "multiaddress", fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, nodeConfig.Host.GetID().Pretty()))
	go archivalNode.DoBeaconSyncing()
}
