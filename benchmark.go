package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/harmony-one/harmony/attack"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/db"
	"github.com/harmony-one/harmony/discovery"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/profiler"
	"github.com/harmony-one/harmony/utils"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

// Constants used by the benchmark.
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
func InitLDBDatabase(ip string, port string) (*db.LDBDatabase, error) {
	// TODO(minhdoan): Refactor this.
	dbFileName := "/tmp/harmony_" + ip + port + ".dat"
	var err = os.RemoveAll(dbFileName)
	if err != nil {
		fmt.Println(err.Error())
	}
	return db.NewLDBDatabase(dbFileName, 0, 0)
}

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2018. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

func loggingInit(logFolder, role, ip, port string, onlyLogTps bool) {
	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/%s-%v-%v.log", logFolder, role, ip, port)
	h := log.MultiHandler(
		log.StdoutHandler,
		log.Must.FileHandler(logFileName, log.JSONFormat()), // Log to file
	)
	if onlyLogTps {
		h = log.MatchFilterHandler("msg", "TPS Report", h)
	}
	log.Root().SetHandler(h)

}
func main() {
	accountModel := flag.Bool("account_model", true, "Whether to use account model")
	// TODO: use http://getmyipaddress.org/ or http://www.get-myip.com/ to retrieve my IP address
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	configFile := flag.String("config_file", "config.txt", "file containing all ip addresses")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	attackedMode := flag.Int("attacked_mode", 0, "0 means not attacked, 1 means attacked, 2 means being open to be selected as attacked")
	dbSupported := flag.Bool("db_supported", false, "false means not db_supported, true means db_supported")
	profile := flag.Bool("profile", false, "Turn on profiling (CPU, Memory).")
	metricsReportURL := flag.String("metrics_report_url", "", "If set, reports metrics to this URL.")
	versionFlag := flag.Bool("version", false, "Output version info")
	onlyLogTps := flag.Bool("only_log_tps", false, "Only log TPS if true")

	// This IP belongs to jenkins.harmony.one
	idcIP := flag.String("idc", "54.183.5.66", "IP of the identity chain")
	idcPort := flag.String("idc_port", "8080", "port of the identity chain")
	peerDisvoery := flag.Bool("peer_discovery", false, "Enable Peer Discovery")

	// Leader needs to have a minimal number of peers to start consensus
	minPeers := flag.Int("min_peers", 100, "Minimal number of Peers in shard")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Add GOMAXPROCS to achieve max performance.
	runtime.GOMAXPROCS(1024)

	// Set up randomization seed.
	rand.Seed(int64(time.Now().Nanosecond()))

	var shardID string
	var peers []p2p.Peer
	var leader p2p.Peer
	var selfPeer p2p.Peer
	var clientPeer *p2p.Peer
	// Use Peer Discovery to get shard/leader/peer/...
	priKey, pubKey := utils.GenKey(*ip, *port)
	if *peerDisvoery {
		// Contact Identity Chain
		// This is a blocking call
		// Assume @ak has get it working
		// TODO: this has to work with @ak's fix
		discoveryConfig := discovery.New(priKey, pubKey)

		err := discoveryConfig.StartClientMode(*idcIP, *idcPort)
		if err != nil {
			fmt.Println("Unable to start peer discovery! ", err)
			os.Exit(1)
		}

		shardID = discoveryConfig.GetShardID()
		leader = discoveryConfig.GetLeader()
		peers = discoveryConfig.GetPeers()
		selfPeer = discoveryConfig.GetSelfPeer()
	} else {
		distributionConfig := utils.NewDistributionConfig()
		distributionConfig.ReadConfigFile(*configFile)
		shardID = distributionConfig.GetShardID(*ip, *port)
		peers = distributionConfig.GetPeers(*ip, *port, shardID)
		leader = distributionConfig.GetLeader(shardID)
		selfPeer = distributionConfig.GetSelfPeer(*ip, *port, shardID)

		// Create client peer.
		clientPeer = distributionConfig.GetClientPeer()
	}
	selfPeer.PubKey = pubKey

	var role string
	if leader.IP == *ip && leader.Port == *port {
		role = "leader"
	} else {
		role = "validator"
	}

	if role == "validator" {
		// Attack determination.
		attack.GetInstance().SetAttackEnabled(attackDetermination(*attackedMode))
	}

	// Init logging.
	loggingInit(*logFolder, role, *ip, *port, *onlyLogTps)

	// Initialize leveldb if dbSupported.
	var ldb *db.LDBDatabase

	if *dbSupported {
		ldb, _ = InitLDBDatabase(*ip, *port)
	}

	// Consensus object.
	consensus := consensus.New(selfPeer, shardID, peers, leader)
	consensus.MinPeers = *minPeers

	// Start Profiler for leader if profile argument is on
	if role == "leader" && (*profile || *metricsReportURL != "") {
		prof := profiler.GetProfiler()
		prof.Config(consensus.Log, shardID, *metricsReportURL)
		if *profile {
			prof.Start()
		}
	}

	// Set logger to attack model.
	attack.GetInstance().SetLogger(consensus.Log)
	// Current node.
	currentNode := node.New(consensus, ldb, selfPeer)
	// Add self peer.
	currentNode.SelfPeer = selfPeer
	// If there is a client configured in the node list.
	if clientPeer != nil {
		currentNode.ClientPeer = clientPeer
	}

	// Assign closure functions to the consensus object
	consensus.BlockVerifier = currentNode.VerifyNewBlock
	consensus.OnConsensusDone = currentNode.PostConsensusProcessing

	// Temporary testing code, to be removed.
	currentNode.AddTestingAddresses(10000)

	currentNode.State = node.NodeWaitToJoin

	if consensus.IsLeader {
		currentNode.State = node.NodeLeader
		if *accountModel {
			// Let consensus run
			go func() {
				consensus.WaitForNewBlockAccount(currentNode.BlockChannelAccount)
			}()
			// Node waiting for consensus readiness to create new block
			go func() {
				currentNode.WaitForConsensusReadyAccount(consensus.ReadySignal)
			}()
		} else {
			// Let consensus run
			go func() {
				consensus.WaitForNewBlock(currentNode.BlockChannel)
			}()
			// Node waiting for consensus readiness to create new block
			go func() {
				currentNode.WaitForConsensusReady(consensus.ReadySignal)
			}()
		}
	} else {
		if *peerDisvoery {
			go currentNode.JoinShard(leader)
		} else {
			node.State = node.NodeDoingConsensus
		}
	}

	go currentNode.SupportSyncing()
	currentNode.StartServer(*port)
}
