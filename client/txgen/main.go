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
	"github.com/harmony-one/harmony/client/txgen/txgen"
	"github.com/harmony-one/harmony/core/types"

	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/client"
	client_config "github.com/harmony-one/harmony/client/config"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	proto_node "github.com/harmony-one/harmony/proto/node"
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
	accountModel := flag.Bool("account_model", true, "Whether to use account model")
	configFile := flag.String("config_file", "local_config.txt", "file containing all ip addresses and config")
	maxNumTxsPerBatch := flag.Int("max_num_txs_per_batch", 20000, "number of transactions to send per message")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	numSubset := flag.Int("numSubset", 3, "the number of subsets of utxos to process separately")
	duration := flag.Int("duration", 10, "duration of the tx generation in second. If it's negative, the experiment runs forever.")
	versionFlag := flag.Bool("version", false, "Output version info")
	crossShardRatio := flag.Int("cross_shard_ratio", 30, "The percentage of cross shard transactions.")
	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Add GOMAXPROCS to achieve max performance.
	runtime.GOMAXPROCS(1024)

	// Read the configs
	config := client_config.NewConfig()
	config.ReadConfigFile(*configFile)
	shardIDLeaderMap := config.GetShardIDToLeaderMap()

	// Do cross shard tx if there are more than one shard
	setting := txgen.TxGenSettings{
		10000,
		len(shardIDLeaderMap) > 1,
		*maxNumTxsPerBatch,
		*crossShardRatio,
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
		node := node.New(&consensus.Consensus{ShardID: shardID}, nil)
		// Assign many fake addresses so we have enough address to play with at first
		node.AddTestingAddresses(setting.NumOfAddress)
		nodes = append(nodes, node)
	}

	// Client/txgenerator server node setup
	clientPort := config.GetClientPort()
	consensusObj := consensus.NewConsensus("0", clientPort, "0", nil, p2p.Peer{})
	clientNode := node.New(consensusObj, nil)

	if clientPort != "" {
		clientNode.Client = client.NewClient(&shardIDLeaderMap)

		// This func is used to update the client's utxopool when new blocks are received from the leaders
		updateBlocksFunc := func(blocks []*blockchain.Block) {
			log.Debug("Received new block from leader", "len", len(blocks))
			for _, block := range blocks {
				for _, node := range nodes {
					shardId := block.ShardID

					accountBlock := new(types.Block)
					err := rlp.DecodeBytes(block.AccountBlock, accountBlock)
					if err == nil {
						shardId = accountBlock.ShardId()
					}
					if node.Consensus.ShardID == shardId {
						log.Debug("Adding block from leader", "shardID", shardId)
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
		go func() {
			clientNode.StartServer(clientPort)
		}()
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
			for shardID, _ := range shardIDLeaderMap { // Generate simulated transactions
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
					SendTxsToLeaderAccount(shardIDLeaderMap[shardID], txs)
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
			for shardID, _ := range shardIDLeaderMap { // Generate simulated transactions
				go func(shardID uint32) {
					txs, crossTxs := txgen.GenerateSimulatedTransactions(subsetCounter, *numSubset, int(shardID), nodes, setting)

					// Put cross shard tx into a pending list waiting for proofs from leaders
					if clientPort != "" {
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
						for curShardID, _ := range client.GetInputShardIDsOfCrossShardTx(crossTx) {
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
					SendTxsToLeader(shardIDLeaderMap[shardID], txs)
				}(shardID, txs)
			}
			lock.Unlock()

			subsetCounter++
			time.Sleep(10000 * time.Millisecond)
		}
	}

	// Send a stop message to stop the nodes at the end
	msg := proto_node.ConstructStopMessage()
	peers := append(config.GetValidators(), clientNode.Client.GetLeaders()...)
	p2p.BroadcastMessage(peers, msg)
	time.Sleep(3000 * time.Millisecond)
}

func SendTxsToLeader(leader p2p.Peer, txs []*blockchain.Transaction) {
	log.Debug("[Generator] Sending txs to...", "leader", leader, "numTxs", len(txs))
	msg := proto_node.ConstructTransactionListMessage(txs)
	p2p.SendMessage(leader, msg)
}

func SendTxsToLeaderAccount(leader p2p.Peer, txs types.Transactions) {
	log.Debug("[Generator] Sending account-based txs to...", "leader", leader, "numTxs", len(txs))
	msg := proto_node.ConstructTransactionListMessageAccount(txs)
	p2p.SendMessage(leader, msg)
}
