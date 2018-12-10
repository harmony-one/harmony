package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/client"
	client_config "github.com/harmony-one/harmony/client/config"

	"github.com/harmony-one/harmony/client/txgen/txgen"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/newnode"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	proto_node "github.com/harmony-one/harmony/proto/node"
	"github.com/harmony-one/harmony/utils"
)

var (
	version       string
	builtBy       string
	builtAt       string
	commit        string
	utxoPoolMutex sync.Mutex
)

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2018. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9999", "port of the node.")

	accountModel := flag.Bool("account_model", true, "Whether to use account model")
	configFile := flag.String("config_file", "local_config.txt", "file containing all ip addresses and config")
	maxNumTxsPerBatch := flag.Int("max_num_txs_per_batch", 20000, "number of transactions to send per message")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	numSubset := flag.Int("numSubset", 3, "the number of subsets of utxos to process separately")
	duration := flag.Int("duration", 10, "duration of the tx generation in second. If it's negative, the experiment runs forever.")
	versionFlag := flag.Bool("version", false, "Output version info")
	crossShardRatio := flag.Int("cross_shard_ratio", 30, "The percentage of cross shard transactions.")

	bcIP := flag.String("bc", "127.0.0.1", "IP of the identity chain")
	bcPort := flag.String("bc_port", "8081", "port of the identity chain")
	peerDiscovery := flag.Bool("peer_discovery", false, "Enable Peer Discovery")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Add GOMAXPROCS to achieve max performance.
	runtime.GOMAXPROCS(1024)

	var clientPeer *p2p.Peer
	var peers []p2p.Peer
	var shardIDLeaderMap map[uint32]p2p.Peer
	var config *client_config.Config

	if *peerDiscovery {
		candidateNode := newnode.New(*ip, *port)
		BCPeer := p2p.Peer{IP: *bcIP, Port: *bcPort}
		candidateNode.ContactBeaconChain(BCPeer)
		peers = nil
		clientPeer = &p2p.Peer{IP: *ip, Port: *port}
		_, pubKey := utils.GenKey(clientPeer.IP, clientPeer.Port)
		clientPeer.PubKey = pubKey

		shardIDLeaderMap = candidateNode.Leaders
	} else {
		// Read the configs
		config = client_config.NewConfig()
		config.ReadConfigFile(*configFile)
		shardIDLeaderMap = config.GetShardIDToLeaderMap()
		clientPeer = config.GetClientPeer()
		_, pubKey := utils.GenKey(clientPeer.IP, clientPeer.Port)
		clientPeer.PubKey = pubKey
	}
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
		log.StdoutHandler,
		log.Must.FileHandler(logFileName, log.LogfmtFormat()), // Log to file
	)
	log.Root().SetHandler(h)

	// Nodes containing utxopools to mirror the shards' data in the network
	nodes := []*node.Node{}
	for shardID := range shardIDLeaderMap {
		_, pubKey := utils.GenKey(clientPeer.IP, clientPeer.Port)
		clientPeer.PubKey = pubKey
		host := p2pimpl.NewHost(*clientPeer)
		node := node.New(host, &consensus.Consensus{ShardID: shardID}, nil)
		// Assign many fake addresses so we have enough address to play with at first
		node.AddTestingAddresses(setting.NumOfAddress)
		nodes = append(nodes, node)
	}

	// Client/txgenerator server node setup
	host := p2pimpl.NewHost(*clientPeer)
	consensusObj := consensus.New(host, "0", nil, p2p.Peer{})
	clientNode := node.New(host, consensusObj, nil)
	clientNode.Client = client.NewClient(clientNode.GetHost(), &shardIDLeaderMap)

	// This func is used to update the client's utxopool when new blocks are received from the leaders
	updateBlocksFunc := func(blocks []*blockchain.Block) {
		log.Debug("Received new block from leader", "len", len(blocks))
		for _, block := range blocks {
			for _, node := range nodes {
				shardID := block.ShardID

				accountBlock := new(types.Block)
				err := rlp.DecodeBytes(block.AccountBlock, accountBlock)
				if err == nil {
					shardID = accountBlock.ShardID()
				}
				if node.Consensus.ShardID == shardID {
					log.Debug("Adding block from leader", "shardID", shardID)
					// Add it to blockchain
					node.AddNewBlock(block)
					utxoPoolMutex.Lock()
					node.UpdateUtxoAndState(block)
					utxoPoolMutex.Unlock()

					if err != nil {
						log.Error("Failed decoding the block with RLP")
					} else {
						fmt.Println("RECEIVED NEW BLOCK ", len(accountBlock.Transactions()))
						node.AddNewBlockAccount(accountBlock)
						node.Worker.UpdateCurrent()
						if err != nil {
							log.Debug("Failed to add new block to worker", "Error", err)
						}
					}
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
		clientNode.State = node.NodeJoinedShard
	}

	// Transaction generation process
	time.Sleep(10 * time.Second) // wait for nodes to be ready
	start := time.Now()
	totalTime := float64(*duration)

	client.InitLookUpIntPriKeyMap()
	subsetCounter := 0

	if *accountModel {
		for {
			t := time.Now()
			if totalTime > 0 && t.Sub(start).Seconds() >= totalTime {
				log.Debug("Generator timer ended.", "duration", (int(t.Sub(start))), "startTime", start, "totalTime", totalTime)
				break
			}
			shardIDTxsMap := make(map[uint32]types.Transactions)
			lock := sync.Mutex{}
			var wg sync.WaitGroup
			wg.Add(len(shardIDLeaderMap))

			utxoPoolMutex.Lock()
			log.Warn("STARTING TX GEN", "gomaxprocs", runtime.GOMAXPROCS(0))
			for shardID := range shardIDLeaderMap { // Generate simulated transactions
				go func(shardID uint32) {
					txs, _ := txgen.GenerateSimulatedTransactionsAccount(int(shardID), nodes, setting)

					// TODO: Put cross shard tx into a pending list waiting for proofs from leaders

					lock.Lock()
					// Put txs into corresponding shards
					shardIDTxsMap[shardID] = append(shardIDTxsMap[shardID], txs...)
					lock.Unlock()
					wg.Done()
				}(shardID)
			}
			wg.Wait()
			utxoPoolMutex.Unlock()

			lock.Lock()
			for shardID, txs := range shardIDTxsMap { // Send the txs to corresponding shards
				go func(shardID uint32, txs types.Transactions) {
					SendTxsToLeaderAccount(clientNode, shardIDLeaderMap[shardID], txs)
				}(shardID, txs)
			}
			lock.Unlock()

			subsetCounter++
			time.Sleep(10000 * time.Millisecond)
		}
	} else {
		for {
			t := time.Now()
			if totalTime > 0 && t.Sub(start).Seconds() >= totalTime {
				log.Debug("Generator timer ended.", "duration", (int(t.Sub(start))), "startTime", start, "totalTime", totalTime)
				break
			}
			shardIDTxsMap := make(map[uint32][]*blockchain.Transaction)
			lock := sync.Mutex{}
			var wg sync.WaitGroup
			wg.Add(len(shardIDLeaderMap))

			utxoPoolMutex.Lock()
			log.Warn("STARTING TX GEN", "gomaxprocs", runtime.GOMAXPROCS(0))
			for shardID := range shardIDLeaderMap { // Generate simulated transactions
				go func(shardID uint32) {
					txs, crossTxs := txgen.GenerateSimulatedTransactions(subsetCounter, *numSubset, int(shardID), nodes, setting)

					// Put cross shard tx into a pending list waiting for proofs from leaders
					if clientPeer != nil {
						clientNode.Client.PendingCrossTxsMutex.Lock()
						for _, tx := range crossTxs {
							clientNode.Client.PendingCrossTxs[tx.ID] = tx
						}
						clientNode.Client.PendingCrossTxsMutex.Unlock()
					}

					lock.Lock()
					// Put txs into corresponding shards
					shardIDTxsMap[shardID] = append(shardIDTxsMap[shardID], txs...)
					for _, crossTx := range crossTxs {
						for curShardID := range client.GetInputShardIDsOfCrossShardTx(crossTx) {
							shardIDTxsMap[curShardID] = append(shardIDTxsMap[curShardID], crossTx)
						}
					}
					lock.Unlock()
					wg.Done()
				}(shardID)
			}
			wg.Wait()
			utxoPoolMutex.Unlock()

			lock.Lock()
			for shardID, txs := range shardIDTxsMap { // Send the txs to corresponding shards
				go func(shardID uint32, txs []*blockchain.Transaction) {
					SendTxsToLeader(clientNode, shardIDLeaderMap[shardID], txs)
				}(shardID, txs)
			}
			lock.Unlock()

			subsetCounter++
			time.Sleep(10000 * time.Millisecond)
		}
	}

	// Send a stop message to stop the nodes at the end
	msg := proto_node.ConstructStopMessage()
	if *peerDiscovery {
		peers = clientNode.Consensus.GetValidatorPeers()
	} else {
		peers = append(config.GetValidators(), clientNode.Client.GetLeaders()...)
	}
	clientNode.BroadcastMessage(peers, msg)
	time.Sleep(3000 * time.Millisecond)
}

// SendTxsToLeader sends txs to leader.
func SendTxsToLeader(clientNode *node.Node, leader p2p.Peer, txs []*blockchain.Transaction) {
	log.Debug("[Generator] Sending txs to...", "leader", leader, "numTxs", len(txs))
	msg := proto_node.ConstructTransactionListMessage(txs)
	clientNode.SendMessage(leader, msg)
}

// SendTxsToLeaderAccount sends txs to leader account.
func SendTxsToLeaderAccount(clientNode *node.Node, leader p2p.Peer, txs types.Transactions) {
	log.Debug("[Generator] Sending account-based txs to...", "leader", leader, "numTxs", len(txs))
	msg := proto_node.ConstructTransactionListMessageAccount(txs)
	clientNode.SendMessage(leader, msg)
}

func debugPrintShardIDLeaderMap(leaderMap map[uint32]p2p.Peer) {
	for k, v := range leaderMap {
		log.Debug("Leader", "ShardID", k, "Leader", v)
	}
}
