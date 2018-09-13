package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/simple-rules/harmony-benchmark/attack"
	"github.com/simple-rules/harmony-benchmark/consensus"
	"github.com/simple-rules/harmony-benchmark/db"
	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/profiler"
	"github.com/simple-rules/harmony-benchmark/utils"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

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

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	configFile := flag.String("config_file", "config.txt", "file containing all ip addresses")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	attackedMode := flag.Int("attacked_mode", 0, "0 means not attacked, 1 means attacked, 2 means being open to be selected as attacked")
	dbSupported := flag.Bool("db_supported", false, "false means not db_supported, true means db_supported")
	profile := flag.Bool("profile", false, "Turn on profiling (CPU, Memory).")
	metricsReportURL := flag.String("metrics_report_url", "", "If set, reports metrics to this URL.")
	versionFlag := flag.Bool("version", false, "Output version info")
	syncNode := flag.Bool("sync_node", false, "Whether this node is a new node joining blockchain and it needs to get synced before joining consensus.")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Set up randomization seed.
	rand.Seed(int64(time.Now().Nanosecond()))

	// Attack determination.
	attack.GetInstance().SetAttackEnabled(attackDetermination(*attackedMode))

	distributionConfig := utils.NewDistributionConfig()
	distributionConfig.ReadConfigFile(*configFile)
	shardID := distributionConfig.GetShardID(*ip, *port)
	peers := distributionConfig.GetPeers(*ip, *port, shardID)
	leader := distributionConfig.GetLeader(shardID)
	selfPeer := distributionConfig.GetSelfPeer(*ip, *port, shardID)

	var role string
	if leader.Ip == *ip && leader.Port == *port {
		role = "leader"
	} else {
		role = "validator"
	}

	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/%s-%v-%v.log", *logFolder, role, *ip, *port)
	h := log.MultiHandler(
		log.StdoutHandler,
		log.Must.FileHandler(logFileName, log.JSONFormat()), // Log to file
	)
	log.Root().SetHandler(h)

	// Initialize leveldb if dbSupported.
	var ldb *db.LDBDatabase = nil

	if *dbSupported {
		ldb, _ = InitLDBDatabase(*ip, *port)
	}

	// Consensus object.
	consensus := consensus.NewConsensus(*ip, *port, shardID, peers, leader)

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
	currentNode := node.New(consensus, ldb)
	// Add self peer.
	currentNode.SelfPeer = selfPeer
	// Add sync node configuration.
	currentNode.SyncNode = *syncNode
	// Create client peer.
	clientPeer := distributionConfig.GetClientPeer()
	// If there is a client configured in the node list.
	if clientPeer != nil {
		currentNode.ClientPeer = clientPeer
	}

	// Assign closure functions to the consensus object
	consensus.BlockVerifier = currentNode.VerifyNewBlock
	consensus.OnConsensusDone = currentNode.PostConsensusProcessing

	// Temporary testing code, to be removed.
	currentNode.AddTestingAddresses(10000)

	if consensus.IsLeader {
		// Let consensus run
		go func() {
			consensus.WaitForNewBlock(currentNode.BlockChannel)
		}()
		// Node waiting for consensus readiness to create new block
		go func() {
			currentNode.WaitForConsensusReady(consensus.ReadySignal)
		}()
	}

	currentNode.StartServer(*port)
}
