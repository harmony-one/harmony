package main

import (
	"flag"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/harmony-one/harmony/consensus"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/api/client"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	p2p_host "github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

var (
	version    string
	builtBy    string
	builtAt    string
	commit     string
	stateMutex sync.Mutex
)

const (
	checkFrequency = 20 //checkfrequency checks whether the transaction generator is ready to send the next batch of transactions.
)

// Settings is the settings for TX generation. No Cross-Shard Support!
type Settings struct {
	NumOfAddress      int
	MaxNumTxsPerBatch int
}

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2018. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

// The main entrance for the transaction generator program which simulate transactions and send to the network for
// processing.
func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9999", "port of the node.")

	maxNumTxsPerBatch := flag.Int("max_num_txs_per_batch", 20000, "number of transactions to send per message")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	duration := flag.Int("duration", 0, "duration of the tx generation in second. If it's negative, the experiment runs forever.")
	versionFlag := flag.Bool("version", false, "Output version info")
	//crossShardRatio := flag.Int("cross_shard_ratio", 30, "The percentage of cross shard transactions.")
	shardIDFlag := flag.Int("shardID", 0, "The shardID the node belongs to.")
	// Key file to store the private key
	keyFile := flag.String("key", "./.txgenkey", "the private key file of the txgen")
	flag.Var(&utils.BootNodes, "bootnodes", "a list of bootnode multiaddress")
	flag.Parse()
	time.Sleep(checkFrequency * time.Second) //This lets other nodes start their services before I query for blockchain height.
	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Add GOMAXPROCS to achieve max performance.
	runtime.GOMAXPROCS(1024)

	// Logging setup
	utils.SetPortAndIP(*port, *ip)

	if len(utils.BootNodes) == 0 {
		bootNodeAddrs, err := utils.StringsToAddrs(utils.DefaultBootNodeAddrStrings)
		if err != nil {
			panic(err)
		}
		utils.BootNodes = bootNodeAddrs
	}

	nodePriKey, _, err := utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	peerPubKey := bls.RandPrivateKey().GetPublicKey()
	if peerPubKey == nil {
		panic(fmt.Errorf("generate key error"))
	}
	shardID := *shardIDFlag
	var shardIDs []uint32
	shardIDs = append(shardIDs, uint32(shardID))
	selfPeer := p2p.Peer{IP: *ip, Port: *port, ConsensusPubKey: peerPubKey}

	// Init with LibP2P enabled, FIXME: (leochen) right now we support only one shard
	setting := Settings{
		NumOfAddress:      10000,
		MaxNumTxsPerBatch: *maxNumTxsPerBatch,
	}

	// TODO(Richard): refactor this chuck to a single method
	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/txgen-%v.log", *logFolder, shardID)
	h := log.MultiHandler(
		log.StreamHandler(os.Stdout, log.TerminalFormat(false)),
		log.Must.FileHandler(logFileName, log.LogfmtFormat()), // Log to file
	)
	log.Root().SetHandler(h)

	// Nodes containing blockchain data to mirror the shards' data in the network

	host, err := p2pimpl.NewHost(&selfPeer, nodePriKey)
	if err != nil {
		panic("unable to new host in txgen")
	}
	node := node.New(host, &consensus.Consensus{ShardID: uint32(shardID)}, nil, true) //Changed it : no longer archival node.
	node.Client = client.NewClient(node.GetHost(), shardIDs)
	node.NodeConfig.SetRole(nodeconfig.ClientNode)
	node.NodeConfig.SetIsBeacon(false)
	//node.NodeConfig.SetShardGroupID(p2p.GroupIDBeacon)
	node.ServiceManagerSetup()
	node.RunServices()

	time.Sleep(checkFrequency * time.Second) //Time for txgen to start its services. This gets me peers.
	utils.GetLogInstance().Debug("Running Go Sync", "node", node.SelfPeer)
	go node.GetSync()
	time.Sleep(checkFrequency * time.Second) //Time for txgen to boot and get its peers and for services to be up and running
	utils.GetLogInstance().Debug("Finished waiting for Go Sync", "node", node.SelfPeer)

	start := time.Now()
	totalTime := float64(*duration)
	ticker := time.NewTicker(checkFrequency * time.Second)
	//utils.GetLogInstance().Debug("Setting up Ticker", "node", node.SelfPeer)
	for {
		t := time.Now()
		if totalTime > 0 && t.Sub(start).Seconds() >= totalTime {
			utils.GetLogInstance().Debug("Generator timer ended.", "duration", (int(t.Sub(start))), "startTime", start, "totalTime", totalTime)
			break
		}
		select {
		case <-ticker.C:
			if node.State.String() == "NodeReadyForConsensus" {
				utils.GetLogInstance().Debug("Generator Will Send Txns.", "node", node.SelfPeer)
				utils.GetLogInstance().Debug("Node State", "node state", node.State.String())
				txs, _ := GenerateSimulatedTransactionsAccount(int(shardID), node, setting)
				SendTxsToShard(node, txs)
				go node.GetSync()
			}
		}
	}

	msg := proto_node.ConstructStopMessage()
	node.GetHost().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeaconClient}, p2p_host.ConstructP2pMessage(byte(0), msg))
	node.GetHost().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, p2p_host.ConstructP2pMessage(byte(0), msg))
	time.Sleep(3 * time.Second)
	node.StartServer()
}

// SendTxsToShard sends txs to shard, currently just to beacon shard
func SendTxsToShard(clientNode *node.Node, txs types.Transactions) {
	msg := proto_node.ConstructTransactionListMessageAccount(txs)
	clientNode.GetHost().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeaconClient}, p2p_host.ConstructP2pMessage(byte(0), msg))
}

// GenerateSimulatedTransactionsAccount generates simulated transaction for account model.
func GenerateSimulatedTransactionsAccount(shardID int, node *node.Node, setting Settings) (types.Transactions, types.Transactions) {
	_ = setting // TODO: make use of settings
	txs := make([]*types.Transaction, 100)
	for i := 0; i < 100; i++ {
		baseNonce := node.Worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(node.TestBankKeys[i].PublicKey))
		randomUserAddress := crypto.PubkeyToAddress(node.TestBankKeys[rand.Intn(100)].PublicKey)
		randAmount := rand.Float32()
		tx, _ := types.SignTx(types.NewTransaction(baseNonce+uint64(0), randomUserAddress, uint32(shardID), big.NewInt(int64(params.Ether*randAmount)), params.TxGas, nil, nil), types.HomesteadSigner{}, node.TestBankKeys[i])
		txs[i] = tx
	}
	return txs, nil
}
