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
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/shardchain"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	bls2 "github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/api/client"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/genesis"
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
	checkFrequency = 2 //checkfrequency checks whether the transaction generator is ready to send the next batch of transactions.
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

var (
	ip              = flag.String("ip", "127.0.0.1", "IP of the node")
	port            = flag.String("port", "9999", "port of the node.")
	numTxns         = flag.Int("numTxns", 100, "number of transactions to send per message")
	logFolder       = flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	duration        = flag.Int("duration", 30, "duration of the tx generation in second. If it's negative, the experiment runs forever.")
	versionFlag     = flag.Bool("version", false, "Output version info")
	crossShardRatio = flag.Int("cross_shard_ratio", 30, "The percentage of cross shard transactions.") //Keeping this for backward compatibility
	shardIDFlag     = flag.Int("shardID", 0, "The shardID the node belongs to.")
	// Key file to store the private key
	keyFile = flag.String("key", "./.txgenkey", "the private key file of the txgen")
	// logging verbosity
	verbosity = flag.Int("verbosity", 5, "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 5)")
)

func setUpTXGen() *node.Node {
	nodePriKey, _, err := utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		panic(err)
	}

	peerPubKey := bls.RandPrivateKey().GetPublicKey()
	if peerPubKey == nil {
		panic(fmt.Errorf("generate key error"))
	}
	shardID := *shardIDFlag
	selfPeer := p2p.Peer{IP: *ip, Port: *port, ConsensusPubKey: peerPubKey}

	gsif, err := consensus.NewGenesisStakeInfoFinder()
	// Nodes containing blockchain data to mirror the shards' data in the network

	myhost, err := p2pimpl.NewHost(&selfPeer, nodePriKey)
	if err != nil {
		panic("unable to new host in txgen")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error :%v \n", err)
		os.Exit(1)
	}
	consensusObj, err := consensus.New(myhost, uint32(shardID), p2p.Peer{}, nil)
	chainDBFactory := &shardchain.MemDBFactory{}
	txGen := node.New(myhost, consensusObj, chainDBFactory, false) //Changed it : no longer archival node.
	txGen.Client = client.NewClient(txGen.GetHost(), uint32(shardID))
	consensusObj.SetStakeInfoFinder(gsif)
	consensusObj.ChainReader = txGen.Blockchain()
	consensusObj.PublicKeys = nil
	startIdx := 0
	endIdx := startIdx + core.GenesisShardSize
	for _, acct := range genesis.GenesisAccounts[startIdx:endIdx] {
		secretKey := bls2.SecretKey{}
		if err := secretKey.SetHexString(acct.BlsPriKey); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "cannot parse secret key: %v\n",
				err)
			os.Exit(1)
		}
		consensusObj.PublicKeys = append(consensusObj.PublicKeys, secretKey.GetPublicKey())
	}
	txGen.NodeConfig.SetRole(nodeconfig.ClientNode)
	if shardID == 0 {
		txGen.NodeConfig.SetIsBeacon(true)
		txGen.NodeConfig.SetShardGroupID(p2p.GroupIDBeacon)
	} else {
		txGen.NodeConfig.SetShardGroupID(p2p.NewGroupIDByShardID(p2p.ShardID(shardID)))
	}

	txGen.NodeConfig.SetIsClient(true)

	return txGen
}
func main() {
	flag.Var(&utils.BootNodes, "bootnodes", "a list of bootnode multiaddress")
	flag.Parse()
	if *versionFlag {
		printVersion(os.Args[0])
	}
	// Add GOMAXPROCS to achieve max performance.
	runtime.GOMAXPROCS(1024)
	// Logging setup
	utils.SetLogContext(*port, *ip)
	utils.SetLogVerbosity(log.Lvl(*verbosity))
	if len(utils.BootNodes) == 0 {
		bootNodeAddrs, err := utils.StringsToAddrs(utils.DefaultBootNodeAddrStrings)
		if err != nil {
			panic(err)
		}
		utils.BootNodes = bootNodeAddrs
	}
	// Init with LibP2P enabled, FIXME: (leochen) right now we support only one shard
	setting := Settings{
		NumOfAddress:      10000,
		MaxNumTxsPerBatch: *numTxns,
	}
	shardID := *shardIDFlag
	utils.GetLogInstance().Debug("Cross Shard Ratio Is Set But not used", "cx ratio", *crossShardRatio)

	// TODO(Richard): refactor this chuck to a single method
	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/txgen.log", *logFolder)
	h := log.MultiHandler(
		log.StreamHandler(os.Stdout, log.TerminalFormat(false)),
		log.Must.FileHandler(logFileName, log.LogfmtFormat()), // Log to file
	)
	log.Root().SetHandler(h)
	txGen := setUpTXGen()
	txGen.ServiceManagerSetup()
	txGen.RunServices()
	start := time.Now()
	totalTime := float64(*duration)
	utils.GetLogInstance().Debug("Total Duration", "totalTime", totalTime, "RunForever", isDurationForever(totalTime))
	ticker := time.NewTicker(checkFrequency * time.Second)
	txGen.DoSyncWithoutConsensus()
syncLoop:
	for {
		t := time.Now()
		if totalTime > 0 && t.Sub(start).Seconds() >= totalTime {
			utils.GetLogInstance().Debug("Generator timer ended in syncLoop.", "duration", (int(t.Sub(start))), "startTime", start, "totalTime", totalTime)
			break syncLoop
		}
		select {
		case <-ticker.C:
			if txGen.State.String() == "NodeReadyForConsensus" {
				utils.GetLogInstance().Debug("Generator is now in Sync.", "txgen node", txGen.SelfPeer, "Node State", txGen.State.String())
				ticker.Stop()
				break syncLoop
			}
		}
	}
	readySignal := make(chan uint32)
	// This func is used to update the client's blockchain when new blocks are received from the leaders
	updateBlocksFunc := func(blocks []*types.Block) {
		utils.GetLogInstance().Info("[Txgen] Received new block", "block num", blocks[0].NumberU64())
		for _, block := range blocks {
			shardID := block.ShardID()
			if txGen.Consensus.ShardID == shardID {
				utils.GetLogInstance().Info("Got block from leader", "txNum", len(block.Transactions()), "shardID", shardID, "preHash", block.ParentHash().Hex(), "currentBlock", txGen.Blockchain().CurrentBlock().NumberU64(), "incoming block", block.NumberU64())
				if block.NumberU64()-txGen.Blockchain().CurrentBlock().NumberU64() == 1 {
					txGen.AddNewBlock(block)
					stateMutex.Lock()
					if err := txGen.Worker.UpdateCurrent(); err != nil {
						ctxerror.Warn(utils.GetLogger(), err,
							"(*Worker).UpdateCurrent failed")
					}
					stateMutex.Unlock()
					readySignal <- shardID
				}
			} else {
				continue
			}
		}
	}
	txGen.Client.UpdateBlocks = updateBlocksFunc
	// Start the client server to listen to leader's message
	go func() {
		// wait for 3 seconds for client to send ping message to leader
		// FIXME (leo) the readySignal should be set once we really sent ping message to leader
		time.Sleep(1 * time.Second) // wait for nodes to be ready
		readySignal <- uint32(shardID)
	}()
pushLoop:
	for {
		t := time.Now()
		utils.GetLogInstance().Debug("Current running time", "running time", t.Sub(start).Seconds(), "totaltime", totalTime)
		if !isDurationForever(totalTime) && t.Sub(start).Seconds() >= totalTime {
			utils.GetLogInstance().Debug("Generator timer ended.", "duration", (int(t.Sub(start))), "startTime", start, "totalTime", totalTime)
			break pushLoop
		}
		if shardID != 0 {
			if otherHeight, flag := txGen.IsSameHeight(); flag {
				if otherHeight >= 1 {
					go func() {
						readySignal <- uint32(shardID)
						utils.GetLogInstance().Debug("Same blockchain height so readySignal generated")
						time.Sleep(3 * time.Second) // wait for nodes to be ready
					}()
				}
			}
		}
		select {
		case shardID := <-readySignal:
			lock := sync.Mutex{}
			utils.GetLogInstance().Warn("STARTING TX GEN PUSH LOOP", "gomaxprocs", runtime.GOMAXPROCS(0))
			txs, err := GenerateSimulatedTransactionsAccount(uint32(shardID), txGen, setting)
			if err != nil {
				utils.GetLogInstance().Debug("Error in Generating Txns", "Err", err)
			}
			lock.Lock()
			SendTxsToShard(txGen, txs, uint32(shardID))
			lock.Unlock()
		case <-time.After(10 * time.Second):
			utils.GetLogInstance().Warn("No new block is received so far")
		}
	}
}

// SendTxsToShard sends txs to shard, currently just to beacon shard
func SendTxsToShard(clientNode *node.Node, txs types.Transactions, shardID uint32) {
	msg := proto_node.ConstructTransactionListMessageAccount(txs)
	var err error
	if shardID == 0 {
		err = clientNode.GetHost().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeaconClient}, p2p_host.ConstructP2pMessage(byte(0), msg))
	} else {
		clientGroup := p2p.NewClientGroupIDByShardID(p2p.ShardID(shardID))
		err = clientNode.GetHost().SendMessageToGroups([]p2p.GroupID{clientGroup}, p2p_host.ConstructP2pMessage(byte(0), msg))
	}
	if err != nil {
		utils.GetLogInstance().Debug("Error in Sending Txns", "Err", err)
	}
}

// GenerateSimulatedTransactionsAccount generates simulated transaction for account model.
func GenerateSimulatedTransactionsAccount(shardID uint32, node *node.Node, setting Settings) (types.Transactions, error) {
	TxnsToGenerate := setting.MaxNumTxsPerBatch // TODO: make use of settings
	txs := make([]*types.Transaction, TxnsToGenerate)
	rounds := (TxnsToGenerate / 100)
	remainder := TxnsToGenerate % 100
	for i := 0; i < 100; i++ {
		baseNonce := node.Worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(node.TestBankKeys[i].PublicKey))
		for j := 0; j < rounds; j++ {
			randomUserAddress := crypto.PubkeyToAddress(node.TestBankKeys[rand.Intn(100)].PublicKey)
			randAmount := rand.Float32()
			tx, _ := types.SignTx(types.NewTransaction(baseNonce+uint64(j), randomUserAddress, shardID, big.NewInt(int64(denominations.One*randAmount)), params.TxGas, nil, nil), types.HomesteadSigner{}, node.TestBankKeys[i])
			txs[100*j+i] = tx
		}
		if i < remainder {
			randomUserAddress := crypto.PubkeyToAddress(node.TestBankKeys[rand.Intn(100)].PublicKey)
			randAmount := rand.Float32()
			tx, _ := types.SignTx(types.NewTransaction(baseNonce+uint64(rounds), randomUserAddress, shardID, big.NewInt(int64(denominations.One*randAmount)), params.TxGas, nil, nil), types.HomesteadSigner{}, node.TestBankKeys[i])
			txs[100*rounds+i] = tx
		}
	}
	return txs, nil
}

func isDurationForever(duration float64) bool {
	return duration <= 0
}
