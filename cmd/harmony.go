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
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/attack"
	"github.com/harmony-one/harmony/internal/profiler"
	"github.com/harmony-one/harmony/internal/utils"
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

// Constants used by the harmony.
const (
	AttackProbability = 20
)

func attackDetermination(attackedMode int) bool {
	switch attackedMode {
	case 0:
		return false
	case 1:
		return true
	case 2:
		return rand.Intn(100) < AttackProbability
	}
	return false
}

// InitLDBDatabase initializes a LDBDatabase.
func InitLDBDatabase(ip string, port string, freshDB bool) (*ethdb.LDBDatabase, error) {
	dbFileName := fmt.Sprintf("./db/harmony_%s_%s", ip, port)
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

func main() {
	// TODO: use http://getmyipaddress.org/ or http://www.get-myip.com/ to retrieve my IP address
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	attackedMode := flag.Int("attacked_mode", 0, "0 means not attacked, 1 means attacked, 2 means being open to be selected as attacked")
	dbSupported := flag.Bool("db_supported", true, "false means not db_supported, true means db_supported")
	freshDB := flag.Bool("fresh_db", false, "true means the existing disk based db will be removed")
	profile := flag.Bool("profile", false, "Turn on profiling (CPU, Memory).")
	metricsReportURL := flag.String("metrics_report_url", "", "If set, reports metrics to this URL.")
	versionFlag := flag.Bool("version", false, "Output version info")
	onlyLogTps := flag.Bool("only_log_tps", false, "Only log TPS if true")
	// Leader needs to have a minimal number of peers to start consensus
	minPeers := flag.Int("min_peers", 100, "Minimal number of Peers in shard")
	// Key file to store the private key
	keyFile := flag.String("key", "./.hmykey", "the private key file of the bootnode")
	flag.Var(&utils.BootNodes, "bootnodes", "a list of bootnode multiaddress")

	// The initial shard ID (TODO: leo) this is a workaround for initial blockchain bootstrap
	shardID := flag.String("shardID", "0", "the shardID. 0 is beacon chain")
	// This node is a leader node. (TODO: leo) this is also a workaround for initial blockchain bootstrap
	isLeader := flag.Bool("leader", false, "false means the node is not a shard leader")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	var peers []p2p.Peer
	var leader p2p.Peer
	var clientPeer *p2p.Peer
	var selfPeer = p2p.Peer{IP: *ip, Port: *port}
	_, selfPeer.PubKey = utils.GenKey(*ip, *port)

	// Initialize leveldb if dbSupported.
	var ldb *ethdb.LDBDatabase

	// Logging setup
	utils.SetPortAndIP(*port, *ip)

	// set leader to be selfPeer if isLeader
	if *isLeader {
		leader = selfPeer
		loggingInit(*logFolder, "leader", *ip, *port, *onlyLogTps)
	} else {
		loggingInit(*logFolder, "validator", *ip, *port, *onlyLogTps)
	}

	if len(utils.BootNodes) == 0 {
		bootNodeAddrs, err := utils.StringsToAddrs(utils.DefaultBootNodeAddrStrings)
		if err != nil {
			panic(err)
		}
		utils.BootNodes = bootNodeAddrs
	}

	// Add GOMAXPROCS to achieve max performance.
	runtime.GOMAXPROCS(1024)

	// Set up randomization seed.
	rand.Seed(int64(time.Now().Nanosecond()))

	if !*isLeader {
		// Attack determination.
		attack.GetInstance().SetAttackEnabled(attackDetermination(*attackedMode))
	}

	if *dbSupported {
		ldb, _ = InitLDBDatabase(*ip, *port, *freshDB)
	}

	privKey, err := utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	host, err := p2pimpl.NewHost(&selfPeer, privKey)
	if err != nil {
		panic("unable to new host in harmony")
	}

	log.Info("HARMONY", "multiaddress", fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, host.GetID().Pretty()))

	// Consensus object.
	consensus := consensus.New(host, *shardID, peers, leader)
	consensus.MinPeers = *minPeers

	// Start Profiler for leader if profile argument is on
	if *isLeader && (*profile || *metricsReportURL != "") {
		prof := profiler.GetProfiler()
		prof.Config(*shardID, *metricsReportURL)
		if *profile {
			prof.Start()
		}
	}

	// Current node.
	currentNode := node.New(host, consensus, ldb)
	currentNode.Consensus.OfflinePeers = currentNode.OfflinePeers

	if *isLeader {
		currentNode.Role = node.ShardLeader
	} else {
		currentNode.Role = node.ShardValidator
	}

	// If there is a client configured in the node list.
	if clientPeer != nil {
		currentNode.ClientPeer = clientPeer
	}

	// Assign closure functions to the consensus object
	consensus.BlockVerifier = currentNode.VerifyNewBlock
	consensus.OnConsensusDone = currentNode.PostConsensusProcessing
	currentNode.State = node.NodeWaitToJoin

	if consensus.IsLeader {
		log.Info("I AM A LEADER", "peer", selfPeer)
		currentNode.State = node.NodeLeader
	} else {
		// go currentNode.JoinShard(leader)
	}

	go currentNode.SupportSyncing()
	if consensus.IsLeader {
		go currentNode.SupportClient()
	}

	log.Info("ServiceManagerSetup")
	currentNode.ServiceManagerSetup()

	log.Info("RunServices")
	currentNode.RunServices()

	log.Info("StartServer")
	currentNode.StartServer()
}
