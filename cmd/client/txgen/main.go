package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/api/client"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/cmd/client/txgen/txgen"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/newnode"
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
	duration := flag.Int("duration", 10, "duration of the tx generation in second. If it's negative, the experiment runs forever.")
	versionFlag := flag.Bool("version", false, "Output version info")
	crossShardRatio := flag.Int("cross_shard_ratio", 30, "The percentage of cross shard transactions.")

	bcIP := flag.String("bc", "127.0.0.1", "IP of the identity chain")
	bcPort := flag.String("bc_port", "8081", "port of the identity chain")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Add GOMAXPROCS to achieve max performance.
	runtime.GOMAXPROCS(1024)

	var clientPeer *p2p.Peer
	var shardIDLeaderMap map[uint32]p2p.Peer

	candidateNode := newnode.New(*ip, *port)
	BCPeer := p2p.Peer{IP: *bcIP, Port: *bcPort}
	candidateNode.ContactBeaconChain(BCPeer)
	clientPeer = &p2p.Peer{IP: *ip, Port: *port}

	shardIDLeaderMap = candidateNode.Leaders

	if clientPeer == nil {
		panic("Client Peer is nil!")
	}
	debugPrintShardIDLeaderMap(shardIDLeaderMap)

	// Do cross shard tx if there are more than one shard
	setting := txgen.Settings{
		NumOfAddress:      10000,
		CrossShard:        len(shardIDLeaderMap) > 1,
		MaxNumTxsPerBatch: *maxNumTxsPerBatch,
		CrossShardRatio:   *crossShardRatio,
	}

	// TODO(Richard): refactor this chuck to a single method
	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/txgen.log", *logFolder)
	h := log.MultiHandler(
		log.StreamHandler(os.Stdout, log.TerminalFormat(false)),
		log.Must.FileHandler(logFileName, log.LogfmtFormat()), // Log to file
	)
	log.Root().SetHandler(h)

	// Nodes containing blockchain data to mirror the shards' data in the network
	nodes := []*node.Node{}
	_, pubKey := utils.GenKey(clientPeer.IP, clientPeer.Port)
	clientPeer.PubKey = pubKey
	host := p2pimpl.NewHost(*clientPeer)
	for shardID := range shardIDLeaderMap {
		node := node.New(host, &consensus.Consensus{ShardID: shardID}, nil)
		// Assign many fake addresses so we have enough address to play with at first
		nodes = append(nodes, node)
	}

	// Client/txgenerator server node setup
	consensusObj := consensus.New(host, "0", nil, p2p.Peer{})
	clientNode := node.New(host, consensusObj, nil)
	clientNode.Client = client.NewClient(clientNode.GetHost(), &shardIDLeaderMap)

	readySignal := make(chan uint32)
	go func() {
		for i := range shardIDLeaderMap {
			readySignal <- i
		}
	}()
	// This func is used to update the client's blockchain when new blocks are received from the leaders
	updateBlocksFunc := func(blocks []*types.Block) {
		log.Info("[Txgen] Received new block", "block", blocks)
		for _, block := range blocks {
			for _, node := range nodes {
				shardID := block.ShardID()

				if node.Consensus.ShardID == shardID {
					// Add it to blockchain
					log.Info("Current Block", "hash", node.Blockchain().CurrentBlock().Hash().Hex())
					log.Info("Adding block from leader", "txNum", len(block.Transactions()), "shardID", shardID, "preHash", block.ParentHash().Hex())
					node.AddNewBlock(block)
					stateMutex.Lock()
					node.Worker.UpdateCurrent()
					stateMutex.Unlock()
					readySignal <- shardID
				} else {
					continue
				}
			}
		}
	}
	clientNode.Client.UpdateBlocks = updateBlocksFunc

	// Start the client server to listen to leader's message
	go clientNode.StartServer()

	for _, leader := range shardIDLeaderMap {
		log.Debug("Client Join Shard", "leader", leader)
		go clientNode.JoinShard(leader)
		// wait for 3 seconds for client to send ping message to leader
		time.Sleep(3 * time.Second)
		clientNode.StopPing <- struct{}{}
		clientNode.State = node.NodeReadyForConsensus
	}

	// Transaction generation process
	time.Sleep(5 * time.Second) // wait for nodes to be ready
	start := time.Now()
	totalTime := float64(*duration)

	for {
		t := time.Now()
		if totalTime > 0 && t.Sub(start).Seconds() >= totalTime {
			log.Debug("Generator timer ended.", "duration", (int(t.Sub(start))), "startTime", start, "totalTime", totalTime)
			break
		}
		select {
		case shardID := <-readySignal:
			shardIDTxsMap := make(map[uint32]types.Transactions)
			lock := sync.Mutex{}

			stateMutex.Lock()
			log.Warn("STARTING TX GEN", "gomaxprocs", runtime.GOMAXPROCS(0))
			txs, _ := txgen.GenerateSimulatedTransactionsAccount(int(shardID), nodes, setting)

			lock.Lock()
			// Put txs into corresponding shards
			shardIDTxsMap[shardID] = append(shardIDTxsMap[shardID], txs...)
			lock.Unlock()
			stateMutex.Unlock()

			lock.Lock()
			for shardID, txs := range shardIDTxsMap { // Send the txs to corresponding shards
				go func(shardID uint32, txs types.Transactions) {
					SendTxsToLeader(clientNode, shardIDLeaderMap[shardID], txs)
				}(shardID, txs)
			}
			lock.Unlock()
		case <-time.After(2 * time.Second):
			log.Warn("No new block is received so far")
		}
	}

	// Send a stop message to stop the nodes at the end
	msg := proto_node.ConstructStopMessage()
	clientNode.BroadcastMessage(clientNode.Client.GetLeaders(), msg)
	time.Sleep(3000 * time.Millisecond)
}

// SendTxsToLeader sends txs to leader account.
func SendTxsToLeader(clientNode *node.Node, leader p2p.Peer, txs types.Transactions) {
	log.Debug("[Generator] Sending account-based txs to...", "leader", leader, "numTxs", len(txs))
	msg := proto_node.ConstructTransactionListMessageAccount(txs)
	clientNode.SendMessage(leader, msg)
}

func debugPrintShardIDLeaderMap(leaderMap map[uint32]p2p.Peer) {
	for k, v := range leaderMap {
		log.Debug("Leader", "ShardID", k, "Leader", v)
	}
}
