package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	bls2 "github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils/contract"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/api/client"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/txgen"
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

	// Key file to store the private key
	keyFile := flag.String("key", "./.txgenkey", "the private key file of the txgen")
	flag.Var(&utils.BootNodes, "bootnodes", "a list of bootnode multiaddress")

	flag.Parse()

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

	var shardIDs []uint32
	nodePriKey, _, err := utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	peerPubKey := bls.RandPrivateKey().GetPublicKey()
	if peerPubKey == nil {
		panic(fmt.Errorf("generate key error"))
	}

	selfPeer := p2p.Peer{IP: *ip, Port: *port, ConsensusPubKey: peerPubKey}

	// Init with LibP2P enabled, FIXME: (leochen) right now we support only one shard
	for i := 0; i < core.GenesisShardNum; i++ {
		shardIDs = append(shardIDs, uint32(i))
	}

	// Do cross shard tx if there are more than one shard
	setting := txgen.Settings{
		NumOfAddress:      10000,
		CrossShard:        false, // len(shardIDs) > 1,
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

	gsif, err := consensus.NewGenesisStakeInfoFinder()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Cannot initialize stake info: %v\n", err)
		os.Exit(1)
	}

	// Nodes containing blockchain data to mirror the shards' data in the network
	nodes := []*node.Node{}
	host, err := p2pimpl.NewHost(&selfPeer, nodePriKey)
	if err != nil {
		panic("unable to new host in txgen")
	}
	for _, shardID := range shardIDs {
		c := &consensus.Consensus{ShardID: shardID}
		node := node.New(host, c, nil, false)
		c.SetStakeInfoFinder(gsif)
		c.ChainReader = node.Blockchain()
		// Replace public keys with genesis accounts for the shard
		c.PublicKeys = nil
		startIdx := core.GenesisShardSize * shardID
		endIdx := startIdx + core.GenesisShardSize
		for _, acct := range contract.GenesisBLSAccounts[startIdx:endIdx] {
			secretKey := bls2.SecretKey{}
			if err := secretKey.SetHexString(acct.Private); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "cannot parse secret key: %v\n",
					err)
				os.Exit(1)
			}
			c.PublicKeys = append(c.PublicKeys, secretKey.GetPublicKey())
		}
		// Assign many fake addresses so we have enough address to play with at first
		nodes = append(nodes, node)
	}

	// Client/txgenerator server node setup
	consensusObj, err := consensus.New(host, 0, p2p.Peer{}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error :%v \n", err)
		os.Exit(1)
	}

	clientNode := node.New(host, consensusObj, nil, false)
	clientNode.Client = client.NewClient(clientNode.GetHost(), shardIDs)

	consensusObj.SetStakeInfoFinder(gsif)
	consensusObj.ChainReader = clientNode.Blockchain()

	readySignal := make(chan uint32)

	// This func is used to update the client's blockchain when new blocks are received from the leaders
	updateBlocksFunc := func(blocks []*types.Block) {
		utils.GetLogInstance().Info("[Txgen] Received new block", "block num", blocks[0].NumberU64())
		for _, block := range blocks {
			for _, node := range nodes {
				shardID := block.ShardID()

				if node.Consensus.ShardID == shardID {
					// Add it to blockchain
					utils.GetLogInstance().Info("Current Block", "block num", node.Blockchain().CurrentBlock().NumberU64())
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
	clientNode.NodeConfig.SetIsClient(true)
	clientNode.ServiceManagerSetup()
	clientNode.RunServices()

	// Start the client server to listen to leader's message
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
