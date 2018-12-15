package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	crypto2 "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	libs "github.com/harmony-one/harmony/beaconchain/libs"
	"github.com/harmony-one/harmony/beaconchain/rpc"
	client2 "github.com/harmony-one/harmony/client/service"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	"log"
	"math/big"
	"strings"

	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/client"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	proto_node "github.com/harmony-one/harmony/proto/node"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2018. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

// AccountState includes the state of an account
type AccountState struct {
	balance *big.Int
	nonce   uint64
}

func main() {
	// Account subcommands
	accountImportCommand := flag.NewFlagSet("import", flag.ExitOnError)
	accountImportPtr := accountImportCommand.String("privateKey", "", "Specify the private key to import")

	// Transfer subcommands
	transferCommand := flag.NewFlagSet("transfer", flag.ExitOnError)
	transferSenderPtr := transferCommand.String("from", "0", "Specify the sender account address or index")
	transferReceiverPtr := transferCommand.String("to", "", "Specify the receiver account")
	transferAmountPtr := transferCommand.Float64("amount", 0, "Specify the amount to transfer")
	transferShardIDPtr := transferCommand.Int("shardID", -1, "Specify the shard ID for the transfer")

	freeTokenCommand := flag.NewFlagSet("getFreeToken", flag.ExitOnError)
	freeTokenAddressPtr := freeTokenCommand.String("address", "", "Specify the account address to receive the free token")

	balanceCommand := flag.NewFlagSet("getFreeToken", flag.ExitOnError)
	balanceAddressPtr := balanceCommand.String("address", "", "Specify the account address to check balance for")

	// Verify that a subcommand has been provided
	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("    wallet <action> <params>")
		fmt.Println("Actions:")
		fmt.Println("    1. new           - Generates a new account and store the private key locally")
		fmt.Println("    2. list          - Lists all accounts in local keystore")
		fmt.Println("    3. removeAll     - Removes all accounts in local keystore")
		fmt.Println("    4. import        - Imports a new account by private key")
		fmt.Println("        --privateKey     - the private key to import")
		fmt.Println("    5. balances      - Shows the balances of all addresses or specific address")
		fmt.Println("        --address        - The address to check balance for")
		fmt.Println("    6. getFreeToken  - Gets free token on each shard")
		fmt.Println("        --address        - The free token receiver account's address")
		fmt.Println("    7. transfer")
		fmt.Println("        --from           - The sender account's address or index in the local keystore")
		fmt.Println("        --to             - The receiver account's address")
		fmt.Println("        --amount         - The amount of token to transfer")
		fmt.Println("        --shardId        - The shard Id for the transfer")
		os.Exit(1)
	}

	// Switch on the subcommand
	switch os.Args[1] {
	case "-version":
		printVersion(os.Args[0])
	case "new":
		randomBytes := [32]byte{}
		_, err := io.ReadFull(rand.Reader, randomBytes[:])

		if err != nil {
			fmt.Println("Failed to get randomness for the private key...")
			return
		}
		priKey, err := crypto2.GenerateKey()
		if err != nil {
			panic("Failed to generate the private key")
		}
		StorePrivateKey(crypto2.FromECDSA(priKey))
		fmt.Printf("New account created with address:\n    {%s}\n", crypto2.PubkeyToAddress(priKey.PublicKey).Hex())
		fmt.Printf("Please keep a copy of the private key:\n    {%s}\n", hex.EncodeToString(crypto2.FromECDSA(priKey)))
	case "list":
		for i, key := range ReadPrivateKeys() {
			fmt.Printf("Account %d:\n  {%s}\n", i, crypto2.PubkeyToAddress(key.PublicKey).Hex())
			fmt.Printf("    PrivateKey: {%s}\n", hex.EncodeToString(key.D.Bytes()))
		}
	case "removeAll":
		ClearKeystore()
		fmt.Println("All existing accounts deleted...")
	case "import":
		accountImportCommand.Parse(os.Args[2:])
		priKey := *accountImportPtr
		if priKey == "" {
			fmt.Println("Error: --privateKey is required")
			return
		}
		if !accountImportCommand.Parsed() {
			fmt.Println("Failed to parse flags")
		}
		priKeyBytes, err := hex.DecodeString(priKey)
		if err != nil {
			panic("Failed to parse the private key into bytes")
		}
		StorePrivateKey(priKeyBytes)
		fmt.Println("Private key imported...")
	case "balances":
		balanceCommand.Parse(os.Args[2:])
		walletNode := CreateWalletNode()

		if *balanceAddressPtr == "" {
			for i, address := range ReadAddresses() {
				fmt.Printf("Account %d: %s:\n", i, address.Hex())
				for shardID, balanceNonce := range FetchBalance(address, walletNode) {
					fmt.Printf("    Balance in Shard %d:  %s \n", shardID, convertBalanceIntoReadableFormat(balanceNonce.balance))
				}
			}
		} else {
			address := common.HexToAddress(*freeTokenAddressPtr)
			for shardID, balanceNonce := range FetchBalance(address, walletNode) {
				balance := balanceNonce.balance
				balance = balance.Div(balance, big.NewInt(params.GWei))
				fmt.Printf("    Balance in Shard %d:  %s \n", shardID, convertBalanceIntoReadableFormat(balanceNonce.balance))
			}
		}
	case "getFreeToken":
		freeTokenCommand.Parse(os.Args[2:])
		walletNode := CreateWalletNode()

		if *freeTokenAddressPtr == "" {
			fmt.Println("Error: --address is required")
			return
		}
		address := common.HexToAddress(*freeTokenAddressPtr)

		GetFreeToken(address, walletNode)
	case "transfer":
		transferCommand.Parse(os.Args[2:])
		if !transferCommand.Parsed() {
			fmt.Println("Failed to parse flags")
		}
		sender := *transferSenderPtr
		receiver := *transferReceiverPtr
		amount := *transferAmountPtr
		shardID := *transferShardIDPtr

		if shardID == -1 {
			fmt.Println("Please specify the shard ID for the transfer (e.g. --shardID=0)")
			return
		}
		if amount <= 0 {
			fmt.Println("Please specify positive amount to transfer")
			return
		}
		priKeys := ReadPrivateKeys()
		if len(priKeys) == 0 {
			fmt.Println("No imported account to use.")
			return
		}
		senderIndex, err := strconv.Atoi(sender)
		addresses := ReadAddresses()
		if err != nil {
			senderIndex = -1
			for i, address := range addresses {
				if address.Hex() == sender {
					senderIndex = i
					break
				}
			}
			if senderIndex == -1 {
				fmt.Println("The specified sender account does not exist in the wallet.")
				break
			}
		}

		if senderIndex >= len(priKeys) {
			fmt.Println("Sender account index out of bounds.")
			return
		}

		receiverAddress := common.HexToAddress(receiver)
		if len(receiverAddress) != 20 {
			fmt.Println("The receiver address is not valid.")
			return
		}

		// Generate transaction
		senderPriKey := priKeys[senderIndex]
		senderAddress := addresses[senderIndex]
		walletNode := CreateWalletNode()
		shardIDToAccountState := FetchBalance(senderAddress, walletNode)

		balance := shardIDToAccountState[uint32(shardID)].balance
		balance = balance.Div(balance, big.NewInt(params.GWei))
		if amount > float64(balance.Uint64())/params.GWei {
			fmt.Printf("Balance is not enough for the transfer, current balance is %.6f\n", float64(balance.Uint64())/params.GWei)
			return
		}

		amountBigInt := big.NewInt(int64(amount * params.GWei))
		amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(params.GWei))
		tx, _ := types.SignTx(types.NewTransaction(shardIDToAccountState[uint32(shardID)].nonce, receiverAddress, uint32(shardID), amountBigInt, params.TxGas, nil, nil), types.HomesteadSigner{}, senderPriKey)
		SubmitTransaction(tx, walletNode, uint32(shardID))
	default:
		fmt.Printf("Unknown action: %s\n", os.Args[1])
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func convertBalanceIntoReadableFormat(balance *big.Int) string {
	balance = balance.Div(balance, big.NewInt(params.GWei))
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
func getShardIDToLeaderMap() map[uint32]p2p.Peer {
	// TODO(ricl): Later use data.harmony.one for API.
	str, _ := client.DownloadURLAsString("https://s3-us-west-2.amazonaws.com/unique-bucket-bin/leaders.txt")
	lines := strings.Split(str, "\n")
	shardIDLeaderMap := map[uint32]p2p.Peer{}
	log.Print(lines)
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")

		shardID := parts[3]
		id, err := strconv.Atoi(shardID)
		if err == nil {
			shardIDLeaderMap[uint32(id)] = p2p.Peer{IP: parts[0], Port: parts[1]}
		} else {
			log.Print("[Generator] Error parsing the shard Id ", shardID)
		}
	}
	return shardIDLeaderMap
}

// CreateWalletNode creates wallet server node.
func CreateWalletNode() *node.Node {
	shardIDLeaderMap := make(map[uint32]p2p.Peer)

	port, _ := strconv.Atoi("9999")
	bcClient := beaconchain.NewClient("54.183.5.66", strconv.Itoa(port+libs.BeaconchainServicePortDiff))
	response := bcClient.GetLeaders()

	for _, leader := range response.Leaders {
		shardIDLeaderMap[leader.ShardId] = p2p.Peer{IP: leader.Ip, Port: leader.Port}
	}

	host := p2pimpl.NewHost(p2p.Peer{})
	walletNode := node.New(host, nil, nil)
	walletNode.Client = client.NewClient(walletNode.GetHost(), &shardIDLeaderMap)
	return walletNode
}

// SubmitTransaction submits the transaction to the Harmony network
func SubmitTransaction(tx *types.Transaction, walletNode *node.Node, shardID uint32) error {
	msg := proto_node.ConstructTransactionListMessageAccount(types.Transactions{tx})
	leader := (*walletNode.Client.Leaders)[shardID]
	walletNode.SendMessage(leader, msg)
	fmt.Printf("Transaction Id for shard %d: %s\n", int(shardID), tx.Hash().Hex())
	time.Sleep(300 * time.Millisecond)
	return nil
}

// FetchBalance fetches account balance of specified address from the Harmony network
func FetchBalance(address common.Address, walletNode *node.Node) map[uint32]AccountState {
	result := make(map[uint32]AccountState)
	for shardID, leader := range *walletNode.Client.Leaders {
		port, _ := strconv.Atoi(leader.Port)
		client := client2.NewClient(leader.IP, strconv.Itoa(port+node.ClientServicePortDiff))
		response := client.GetBalance(address)
		balance := big.NewInt(0)
		balance.SetBytes(response.Balance)
		result[shardID] = AccountState{balance, response.Nonce}
	}

	return result
}

// GetFreeToken requests for token test token on each shard
func GetFreeToken(address common.Address, walletNode *node.Node) {
	for shardID, leader := range *walletNode.Client.Leaders {
		port, _ := strconv.Atoi(leader.Port)
		client := client2.NewClient(leader.IP, strconv.Itoa(port+node.ClientServicePortDiff))
		response := client.GetFreeToken(address)

		txID := common.Hash{}
		txID.SetBytes(response.TxId)
		fmt.Printf("Transaction Id requesting free token in shard %d: %s\n", int(shardID), txID.Hex())
	}
}

// FetchUtxos fetches utxos of specified address from the Harmony network
func FetchUtxos(addresses [][20]byte, walletNode *node.Node) (map[uint32]blockchain.UtxoMap, error) {
	walletNode.Client.ShardUtxoMap = make(map[uint32]blockchain.UtxoMap)
	walletNode.BroadcastMessage(walletNode.Client.GetLeaders(), proto_node.ConstructFetchUtxoMessage(*walletNode.ClientPeer, addresses))

	doneSignal := make(chan int)
	go func() {
		for {
			if len(walletNode.Client.ShardUtxoMap) == len(*walletNode.Client.Leaders) {
				doneSignal <- 0
				break
			}
		}
	}()

	select {
	case <-doneSignal:
		return walletNode.Client.ShardUtxoMap, nil
	case <-time.After(3 * time.Second):
		return nil, errors.New("Utxo fetch timed out")
	}
}

// ReadAddresses reads the addresses stored in local keystore
func ReadAddresses() []common.Address {
	priKeys := ReadPrivateKeys()
	addresses := []common.Address{}
	for _, key := range priKeys {
		addresses = append(addresses, crypto2.PubkeyToAddress(key.PublicKey))
	}
	return addresses
}

// StorePrivateKey stores the specified private key in local keystore
func StorePrivateKey(priKey []byte) {
	privateKey, err := crypto2.ToECDSA(priKey)
	if err != nil {
		panic("Failed to deserialize private key")
	}
	for _, address := range ReadAddresses() {
		if address == crypto2.PubkeyToAddress(privateKey.PublicKey) {
			fmt.Println("The key already exists in the keystore")
			return
		}
	}
	f, err := os.OpenFile("keystore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		panic("Failed to open keystore")
	}
	_, err = f.Write(priKey)

	if err != nil {
		panic("Failed to write to keystore")
	}
	f.Close()
}

// ClearKeystore deletes all data in the local keystore
func ClearKeystore() {
	ioutil.WriteFile("keystore", []byte{}, 0644)
}

// ReadPrivateKeys reads all the private key stored in local keystore
func ReadPrivateKeys() []*ecdsa.PrivateKey {
	keys, err := ioutil.ReadFile("keystore")
	if err != nil {
		return []*ecdsa.PrivateKey{}
	}
	priKeys := []*ecdsa.PrivateKey{}
	for i := 0; i < len(keys); i += 32 {
		priKey, err := crypto2.ToECDSA(keys[i : i+32])
		if err != nil {
			fmt.Println("Failed deserializing key data: ", keys[i:i+32])
			continue
		}
		priKeys = append(priKeys, priKey)
	}
	return priKeys
}
