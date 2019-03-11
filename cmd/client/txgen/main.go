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

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/api/client"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/cmd/client/txgen/txgen"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
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

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2018. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

var (
	ip   = flag.String("ip", "127.0.0.1", "IP of the node")
	port = flag.String("port", "9999", "port of the node.")

	maxNumTxsPerBatch = flag.Int("max_num_txs_per_batch", 20000, "number of transactions to send per message")
	logFolder         = flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	duration          = flag.Int("duration", 10, "duration of the tx generation in second. If it's negative, the experiment runs forever.")
	versionFlag       = flag.Bool("version", false, "Output version info")
	crossShardRatio   = flag.Int("cross_shard_ratio", 30, "The percentage of cross shard transactions.")

	// Key file to store the private key
	keyFile = flag.String("key", "./.txgenkey", "the private key file of the txgen")
)

func initSetup() {
	flag.Var(&utils.BootNodes, "bootnodes", "a list of bootnode multiaddress")
	flag.Parse()

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
	nodeConfig.ShardIDString = "0"

	// P2p private key is used for secure message transfer between p2p nodes.
	nodeConfig.P2pPriKey, _, err = utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	// Consensus keys are the BLS12-381 keys used to sign consensus messages
	nodeConfig.ConsensusPriKey, nodeConfig.ConsensusPubKey = utils.GenKey(*ip, *port)
	if nodeConfig.ConsensusPriKey == nil || nodeConfig.ConsensusPubKey == nil {
		panic(fmt.Errorf("generate key error"))
	}
	// Key Setup ================= [End]

	nodeConfig.SelfPeer = p2p.Peer{IP: *ip, Port: *port, ValidatorID: -1, ConsensusPubKey: nodeConfig.ConsensusPubKey}
	nodeConfig.StringRole = "txgen"

	nodeConfig.Host, err = p2pimpl.NewHost(&nodeConfig.SelfPeer, nodeConfig.P2pPriKey)
	if err != nil {
		panic("unable to new host in harmony")
	}

	nodeConfig.Host.AddPeer(&nodeConfig.Leader)

	return nodeConfig
}

// The main entrance for the transaction generator program which simulate transactions and send to the network for
// processing.
func main() {
	initSetup()
	nodeConfig := createGlobalConfig()

	var shardIDs = []uint32{0}

	// Do cross shard tx if there are more than one shard
	setting := txgen.Settings{
		NumOfAddress:      10000,
		CrossShard:        false,
		MaxNumTxsPerBatch: *maxNumTxsPerBatch,
		CrossShardRatio:   *crossShardRatio,
	}

	// Nodes containing blockchain data to mirror the shards' data in the network
	nodes := []*node.Node{}

	for _, shardID := range shardIDs {
		node := node.New(nodeConfig.Host, &consensus.Consensus{ShardID: shardID}, nil)
		// Assign many fake addresses so we have enough address to play with at first
		nodes = append(nodes, node)
	}

	// Client/txgenerator server node setup
	consensusObj := consensus.New(nodeConfig.Host, nodeConfig.ShardIDString, nil, p2p.Peer{})
	clientNode := node.New(nodeConfig.Host, consensusObj, nil)
	clientNode.Client = client.NewClient(clientNode.GetHost(), shardIDs)

	readySignal := make(chan uint32)

	// This func is used to update the client's blockchain when new blocks are received from the leaders
	updateBlocksFunc := func(blocks []*types.Block) {
		utils.GetLogInstance().Info("[Txgen] Received new block", "block", blocks)
		for _, block := range blocks {
			for _, node := range nodes {
				shardID := block.ShardID()

				if node.Consensus.ShardID == shardID {
					// Add it to blockchain
					utils.GetLogInstance().Info("Current Block", "hash", node.Blockchain().CurrentBlock().Hash().Hex())
					utils.GetLogInstance().Info("Adding block from leader", "txNum", len(block.Transactions()), "shardID", shardID, "preHash", block.ParentHash().Hex())
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

	clientNode.NodeConfig.SetRole(nodeconfig.ClientNode)
	clientNode.ServiceManagerSetup()
	clientNode.RunServices()

	log.Info("Harmony Client Node", "Role", clientNode.NodeConfig.Role(), "multiaddress", fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, nodeConfig.Host.GetID().Pretty()))
	go clientNode.SupportSyncing()

	// Start the client server to listen to leader's message
	go clientNode.StartServer()
	clientNode.State = node.NodeReadyForConsensus

	go func() {
		// wait for 3 seconds for client to send ping message to leader
		// FIXME (leo) the readySignal should be set once we really sent ping message to leader
		time.Sleep(3 * time.Second) // wait for nodes to be ready
		for _, i := range shardIDs {
			readySignal <- i
		}
	}()

	// Transaction generation process
	start := time.Now()
	totalTime := float64(*duration)

	for {
		t := time.Now()
		if totalTime > 0 && t.Sub(start).Seconds() >= totalTime {
			utils.GetLogInstance().Debug("Generator timer ended.", "duration", (int(t.Sub(start))), "startTime", start, "totalTime", totalTime)
			break
		}
		select {
		case shardID := <-readySignal:
			shardIDTxsMap := make(map[uint32]types.Transactions)
			lock := sync.Mutex{}

			stateMutex.Lock()
			utils.GetLogInstance().Warn("STARTING TX GEN", "gomaxprocs", runtime.GOMAXPROCS(0))
			txs, _ := txgen.GenerateSimulatedTransactionsAccount(int(shardID), nodes, setting)

			lock.Lock()
			// Put txs into corresponding shards
			shardIDTxsMap[shardID] = append(shardIDTxsMap[shardID], txs...)
			lock.Unlock()
			stateMutex.Unlock()

			lock.Lock()
			for shardID, txs := range shardIDTxsMap { // Send the txs to corresponding shards
				go func(shardID uint32, txs types.Transactions) {
					SendTxsToShard(clientNode, txs)
				}(shardID, txs)
			}
			lock.Unlock()
		case <-time.After(10 * time.Second):
			utils.GetLogInstance().Warn("No new block is received so far")
		}
	}

	// Send a stop message to stop the nodes at the end
	msg := proto_node.ConstructStopMessage()
	clientNode.GetHost().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeaconClient}, p2p_host.ConstructP2pMessage(byte(0), msg))
	clientNode.GetHost().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, p2p_host.ConstructP2pMessage(byte(0), msg))

	time.Sleep(3 * time.Second)
}

// SendTxsToShard sends txs to shard, currently just to beacon shard
func SendTxsToShard(clientNode *node.Node, txs types.Transactions) {
	msg := proto_node.ConstructTransactionListMessageAccount(txs)
	clientNode.GetHost().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeaconClient}, p2p_host.ConstructP2pMessage(byte(0), msg))
}
