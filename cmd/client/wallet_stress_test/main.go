package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
	"github.com/harmony-one/harmony/api/client"
	clientService "github.com/harmony-one/harmony/api/client/service"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	p2p_host "github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2019. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

// AccountState includes the balance and nonce of an account
type AccountState struct {
	balance *big.Int
	nonce   uint64
}

const (
	rpcRetry          = 3
	defaultConfigFile = ".hmy/wallet.ini"
	defaultProfile    = "default"
	keystoreDir       = ".hmy/keystore"
)

var (
	// Transfer subcommands
	transferCommand       = flag.NewFlagSet("transfer", flag.ExitOnError)
	transferSenderPtr     = transferCommand.String("from", "0", "Specify the sender account address or index")
	transferReceiverPtr   = transferCommand.String("to", "", "Specify the receiver account")
	transferAmountPtr     = transferCommand.Float64("amount", 0, "Specify the amount to transfer")
	transferGasPricePtr   = transferCommand.Uint64("gasPrice", 0, "Specify the gas price amount. Unit is Nano.")
	transferShardIDPtr    = transferCommand.Int("shardID", 0, "Specify the shard ID for the transfer")
	transferInputDataPtr  = transferCommand.String("inputData", "", "Base64-encoded input data to embed in the transaction")
	transferSenderPassPtr = transferCommand.String("pass", "", "Passphrase of the sender's private key")
)

var (
	walletProfile *utils.WalletProfile
	ks            *keystore.KeyStore
)

// setupLog setup log for verbose output
func setupLog() {
	// enable logging for wallet
	h := log.StreamHandler(os.Stdout, log.TerminalFormat(true))
	log.Root().SetHandler(h)
}

// The main wallet program entrance. Note the this wallet program is for demo-purpose only. It does not implement
// the secure storage of keys.
func main() {
	rand.Seed(int64(time.Now().Nanosecond()))

	// Verify that a subcommand has been provided
	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("    wallet -p profile <action> <params>")
		fmt.Println("    -p profile       - Specify the profile of the wallet, either testnet/devnet or others configured. Default is: testnet")
		fmt.Println("                       The profile is in file:", defaultConfigFile)
		fmt.Println()
		fmt.Println("Actions:")
		fmt.Println("   1. stressTest	   	 - Stress test transactions with corner cases.")
		os.Exit(1)
	}

ARG:
	for {
		lastArg := os.Args[len(os.Args)-1]
		switch lastArg {
		case "--verbose":
			setupLog()
			os.Args = os.Args[:len(os.Args)-1]
		default:
			break ARG
		}
	}

	var profile string
	if os.Args[1] == "-p" {
		profile = os.Args[2]
		os.Args = os.Args[2:]
	} else {
		profile = defaultProfile
	}
	if len(os.Args) == 1 {
		fmt.Println("Missing action")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// create new keystore backend
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	ks = keystore.NewKeyStore(keystoreDir, scryptN, scryptP)

	// Switch on the subcommand
	switch os.Args[1] {
	case "-version":
		printVersion(os.Args[0])
	case "stressTest":
		readProfile(profile)
		processStressTestCommand()
	default:
		fmt.Printf("Unknown action: %s\n", os.Args[1])
		flag.PrintDefaults()
		os.Exit(1)
	}
}

//go:generate go run ../../../scripts/wallet_embed_ini_files.go

func readProfile(profile string) {
	fmt.Printf("Using %s profile for wallet\n", profile)

	// try to load .hmy/wallet.ini from filesystem
	// use default_wallet_ini if .hmy/wallet.ini doesn't exist
	var err error
	var iniBytes []byte

	iniBytes, err = ioutil.ReadFile(defaultConfigFile)
	if err != nil {
		log.Debug(fmt.Sprintf("%s doesn't exist, using default ini\n", defaultConfigFile))
		iniBytes = []byte(defaultWalletIni)
	}

	walletProfile, err = utils.ReadWalletProfile(iniBytes, profile)
	if err != nil {
		fmt.Printf("Read wallet profile error: %v\nExiting ...\n", err)
		os.Exit(2)
	}
}

// createWalletNode creates wallet server node.
func createWalletNode() *node.Node {
	bootNodeAddrs, err := utils.StringsToAddrs(walletProfile.Bootnodes)
	if err != nil {
		panic(err)
	}
	utils.BootNodes = bootNodeAddrs
	shardID := 0
	// dummy host for wallet
	// TODO: potentially, too many dummy IP may flush out good IP address from our bootnode DHT
	// we need to understand the impact to bootnode DHT with this dummy host ip added
	self := p2p.Peer{IP: "127.0.0.1", Port: "6999"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "6999")
	host, err := p2pimpl.NewHost(&self, priKey)
	if err != nil {
		panic(err)
	}
	chainDBFactory := &shardchain.MemDBFactory{}
	w := node.New(host, nil, chainDBFactory, false)
	w.Client = client.NewClient(w.GetHost(), uint32(shardID))

	w.NodeConfig.SetRole(nodeconfig.ClientNode)
	w.ServiceManagerSetup()
	w.RunServices()
	return w
}

// ./bin/wallet -p local transfer
// --from one1uyshu2jgv8w465yc8kkny36thlt2wvel89tcmg
// --to one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj
// --amount 1 --shardID 1

func processStressTestCommand() {
	/*

			Account 17:
		    Address: one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj
		    Balance in Shard 0:  x.xxx, nonce: 0
		    Balance in Shard 1:  0.0000, nonce: 0
			Account 18:
		    Address: one1uyshu2jgv8w465yc8kkny36thlt2wvel89tcmg
		    Balance in Shard 0:  0.0000, nonce: 0
		    Balance in Shard 1:  x.xxx, nonce: 0

	*/

	senderAddress := common2.ParseAddr("one1uyshu2jgv8w465yc8kkny36thlt2wvel89tcmg")
	receiverAddress := common2.ParseAddr("one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj")
	shardID := 1

	walletNode := createWalletNode()

	shardIDToAccountState := FetchBalance(senderAddress)
	state := shardIDToAccountState[shardID]

	balance := state.balance
	// amount 1/10th of the balance
	amountBigInt := balance.Div(balance, big.NewInt(10))

	// default inputData
	data := make([]byte, 0)

	gasLimit, _ := core.IntrinsicGas(data, false, true)

	gasPrice := 0
	gasPriceBigInt := big.NewInt(int64(gasPrice))
	gasPriceBigInt = gasPriceBigInt.Mul(gasPriceBigInt, big.NewInt(denominations.Nano))

	senderPass := ""

	for i := 0; i < 4; i++ {
		currNonce := state.nonce
		tx := types.NewTransaction(
			currNonce, receiverAddress, uint32(shardID), amountBigInt,
			gasLimit, gasPriceBigInt, data)

		account, _ := ks.Find(accounts.Account{Address: senderAddress})

		ks.Unlock(account, senderPass)

		tx, _ = ks.SignTx(account, tx, nil)

		if err := submitTransaction(tx, walletNode, uint32(shardID)); err != nil {
			fmt.Println(ctxerror.New("submitTransaction failed",
				"tx", tx, "shardID", shardID).WithCause(err))
		}

		for retry := 0; retry < 10; retry++ {
			accountStates := FetchBalance(senderAddress)
			state = accountStates[shardID]
			fmt.Println("state.nonce", state.nonce)
			if state.nonce == currNonce+1 {
				break
			}
			time.Sleep(3 * time.Second)
		}
	}

	fmt.Printf("Sender Account: %s:\n", common2.MustAddressToBech32(senderAddress))
	for shardID, balanceNonce := range FetchBalance(senderAddress) {
		fmt.Printf("    Balance in Shard %d:  %s, nonce: %v \n", shardID, convertBalanceIntoReadableFormat(balanceNonce.balance), balanceNonce.nonce)
	}
}

func convertBalanceIntoReadableFormat(balance *big.Int) string {
	balance = balance.Div(balance, big.NewInt(denominations.Nano))
	strBalance := fmt.Sprintf("%d", balance.Uint64())

	bytes := []byte(strBalance)
	hasDecimal := false
	for i := 0; i < 11; i++ {
		if len(bytes)-1-i < 0 {
			bytes = append([]byte{'0'}, bytes...)
		}
		if bytes[len(bytes)-1-i] != '0' && i < 9 {
			hasDecimal = true
		}
		if i == 9 {
			newBytes := append([]byte{'.'}, bytes[len(bytes)-i:]...)
			bytes = append(bytes[:len(bytes)-i], newBytes...)
		}
	}
	zerosToRemove := 0
	for i := 0; i < len(bytes); i++ {
		if hasDecimal {
			if bytes[len(bytes)-1-i] == '0' {
				bytes = bytes[:len(bytes)-1-i]
				i--
			} else {
				break
			}
		} else {
			if zerosToRemove < 5 {
				bytes = bytes[:len(bytes)-1-i]
				i--
				zerosToRemove++
			} else {
				break
			}
		}
	}
	return string(bytes)
}

// FetchBalance fetches account balance of specified address from the Harmony network
func FetchBalance(address common.Address) []*AccountState {
	result := []*AccountState{}
	for shardID := 0; shardID < walletProfile.Shards; shardID++ {
		// Fill in nil pointers for each shard; nil represent failed balance fetch.
		result = append(result, nil)
	}

	var wg sync.WaitGroup
	wg.Add(walletProfile.Shards)

	for shardID := 0; shardID < walletProfile.Shards; shardID++ {
		go func(shardID int) {
			defer wg.Done()
			balance := big.NewInt(0)
			var nonce uint64
			result[uint32(shardID)] = &AccountState{balance, 0}

			var wgShard sync.WaitGroup
			wgShard.Add(len(walletProfile.RPCServer[shardID]))

			var mutexAccountState = &sync.Mutex{}

			for rpcServerID := 0; rpcServerID < len(walletProfile.RPCServer[shardID]); rpcServerID++ {
				go func(rpcServerID int) {
					for retry := 0; retry < rpcRetry; retry++ {

						server := walletProfile.RPCServer[shardID][rpcServerID]
						client, err := clientService.NewClient(server.IP, server.Port)
						if err != nil {
							continue
						}

						log.Debug("FetchBalance", "server", server)
						response, err := client.GetBalance(address)
						if err != nil {
							log.Info("failed to get balance, retrying ...")
							time.Sleep(200 * time.Millisecond)
							continue
						}
						log.Debug("FetchBalance", "response", response)
						respBalance := big.NewInt(0)
						respBalance.SetBytes(response.Balance)

						mutexAccountState.Lock()
						if balance.Cmp(respBalance) < 0 {
							balance.SetBytes(response.Balance)
							nonce = response.Nonce
						}
						mutexAccountState.Unlock()
						break
					}
					wgShard.Done()
				}(rpcServerID)
			}
			wgShard.Wait()

			result[shardID] = &AccountState{balance, nonce}
		}(shardID)
	}
	wg.Wait()
	return result
}

// GetFreeToken requests for token test token on each shard
func GetFreeToken(address common.Address) {
	for i := 0; i < walletProfile.Shards; i++ {
		// use the 1st server (leader) to make the getFreeToken call
		server := walletProfile.RPCServer[i][0]
		client, err := clientService.NewClient(server.IP, server.Port)
		if err != nil {
			continue
		}

		log.Debug("GetFreeToken", "server", server)

		for retry := 0; retry < rpcRetry; retry++ {
			response, err := client.GetFreeToken(address)
			if err != nil {
				log.Info("failed to get free token, retrying ...")
				time.Sleep(200 * time.Millisecond)
				continue
			}
			log.Debug("GetFreeToken", "response", response)
			txID := common.Hash{}
			txID.SetBytes(response.TxId)
			fmt.Printf("Transaction Id requesting free token in shard %d: %s\n", i, txID.Hex())
			break
		}
	}
}

// clearKeystore deletes all data in the local keystore
func clearKeystore() {
	dir, err := ioutil.ReadDir(keystoreDir)
	if err != nil {
		panic("Failed to read keystore directory")
	}
	for _, d := range dir {
		subdir := path.Join([]string{keystoreDir, d.Name()}...)
		if err := os.RemoveAll(subdir); err != nil {
			fmt.Println(ctxerror.New("cannot remove directory",
				"path", subdir).WithCause(err))
		}
	}
	fmt.Println("All existing accounts deleted...")
}

// submitTransaction submits the transaction to the Harmony network
func submitTransaction(tx *types.Transaction, walletNode *node.Node, shardID uint32) error {
	msg := proto_node.ConstructTransactionListMessageAccount(types.Transactions{tx})
	clientGroup := p2p.NewClientGroupIDByShardID(p2p.ShardID(shardID))

	err := walletNode.GetHost().SendMessageToGroups([]p2p.GroupID{clientGroup}, p2p_host.ConstructP2pMessage(byte(0), msg))
	if err != nil {
		fmt.Printf("Error in SubmitTransaction: %v\n", err)
		return err
	}
	fmt.Printf("Transaction Id for shard %d: %s\n", int(shardID), tx.Hash().Hex())
	// FIXME (leo): how to we know the tx was successful sent to the network
	// this is a hacky way to wait for sometime
	time.Sleep(3 * time.Second)
	return nil
}
