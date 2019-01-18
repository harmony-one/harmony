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
	pkg_newnode "github.com/harmony-one/harmony/internal/newnode"
	"github.com/harmony-one/harmony/internal/profiler"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"

	peerstore "github.com/libp2p/go-libp2p-peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
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
func InitLDBDatabase(ip string, port string) (*ethdb.LDBDatabase, error) {
	// TODO(minhdoan): Refactor this.
	dbFileName := "/tmp/harmony_" + ip + port + ".dat"
	var err = os.RemoveAll(dbFileName)
	if err != nil {
		fmt.Println(err.Error())
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
	dbSupported := flag.Bool("db_supported", false, "false means not db_supported, true means db_supported")
	profile := flag.Bool("profile", false, "Turn on profiling (CPU, Memory).")
	metricsReportURL := flag.String("metrics_report_url", "", "If set, reports metrics to this URL.")
	versionFlag := flag.Bool("version", false, "Output version info")
	onlyLogTps := flag.Bool("only_log_tps", false, "Only log TPS if true")

	//This IP belongs to jenkins.harmony.one
	bcIP := flag.String("bc", "127.0.0.1", "IP of the identity chain")
	bcPort := flag.String("bc_port", "8081", "port of the identity chain")
	bcAddr := flag.String("bc_addr", "", "MultiAddr of the identity chain")

	//Leader needs to have a minimal number of peers to start consensus
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
	var BCPeer *p2p.Peer

	if *bcAddr != "" {
		// Turn the destination into a multiaddr.
		maddr, err := multiaddr.NewMultiaddr(*bcAddr)
		if err != nil {
			panic(err)
		}

		// Extract the peer ID from the multiaddr.
		info, err := peerstore.InfoFromP2pAddr(maddr)
		if err != nil {
			panic(err)
		}

		BCPeer = &p2p.Peer{IP: *bcIP, Port: *bcPort, Addrs: info.Addrs, PeerID: info.ID}
	} else {
		BCPeer = &p2p.Peer{IP: *bcIP, Port: *bcPort}
	}

	//Use Peer Discovery to get shard/leader/peer/...
	candidateNode := pkg_newnode.New(*ip, *port)
	candidateNode.AddPeer(BCPeer)
	candidateNode.ContactBeaconChain(*BCPeer)

	shardID = candidateNode.GetShardID()
	leader = candidateNode.GetLeader()
	selfPeer = candidateNode.GetSelfPeer()
	clientPeer = candidateNode.GetClientPeer()
	selfPeer.PubKey = candidateNode.PubK

	// fmt.Println(peers, leader, selfPeer, clientPeer, *logFolder, *minPeers) //TODO: to be replaced by a logger later: ak, rl

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
	var ldb *ethdb.LDBDatabase

	if *dbSupported {
		ldb, _ = InitLDBDatabase(*ip, *port)
	}

	host, err := p2pimpl.NewHost(&selfPeer)
	if err != nil {
		panic("unable to new host in harmony")
	}

	fmt.Printf("[Harmony] selfPeer: %v\n", selfPeer)
	fmt.Printf("[Harmony] leader: %v\n", leader)

	host.AddPeer(&leader)

	// Consensus object.
	consensus := consensus.New(host, shardID, peers, leader)
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
	currentNode := node.New(host, consensus, ldb)
	currentNode.Consensus.OfflinePeers = currentNode.OfflinePeers

	// If there is a client configured in the node list.
	if clientPeer != nil {
		currentNode.ClientPeer = clientPeer
	}

	// Assign closure functions to the consensus object
	consensus.BlockVerifier = currentNode.VerifyNewBlock
	consensus.OnConsensusDone = currentNode.PostConsensusProcessing

	currentNode.State = node.NodeWaitToJoin

	if consensus.IsLeader {
		currentNode.State = node.NodeLeader
		// Let consensus run
		go func() {
			consensus.WaitForNewBlock(currentNode.BlockChannel)
		}()
		// Node waiting for consensus readiness to create new block
		go func() {
			currentNode.WaitForConsensusReady(consensus.ReadySignal)
		}()
	} else {
		go currentNode.JoinShard(leader)
	}

	go currentNode.SupportSyncing()
	if consensus.IsLeader {
		go currentNode.SupportClient()
		go currentNode.SupportExplorer()
	}
	currentNode.StartServer()
}
