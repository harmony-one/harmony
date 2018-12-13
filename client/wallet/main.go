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
	client2 "github.com/harmony-one/harmony/client/service"
	"log"
	"math/big"
	"strings"

	"github.com/harmony-one/harmony/p2p/p2pimpl"

	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/client"
	client_config "github.com/harmony-one/harmony/client/config"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	proto_node "github.com/harmony-one/harmony/proto/node"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"time"
)

func main() {
	// Account subcommands
	accountImportCommand := flag.NewFlagSet("import", flag.ExitOnError)
	accountImportPtr := accountImportCommand.String("privateKey", "", "Specify the private key to import")

	// Transfer subcommands
	transferCommand := flag.NewFlagSet("transfer", flag.ExitOnError)
	transferSenderPtr := transferCommand.String("sender", "0", "Specify the sender account address or index")
	transferReceiverPtr := transferCommand.String("receiver", "", "Specify the receiver account")
	transferAmountPtr := transferCommand.Int("amount", 0, "Specify the amount to transfer")

	// Verify that a subcommand has been provided
	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		fmt.Println("account or transfer subcommand is required")
		os.Exit(1)
	}

	// Switch on the subcommand
	switch os.Args[1] {
	case "account":
		switch os.Args[2] {
		case "new":
			randomBytes := [32]byte{}
			_, err := io.ReadFull(rand.Reader, randomBytes[:])

			if err != nil {
				fmt.Println("Failed to create a new private key...")
				return
			}
			priKey := crypto.Ed25519Curve.Scalar().SetBytes(randomBytes[:])
			priKeyBytes, err := priKey.MarshalBinary()
			if err != nil {
				panic("Failed to serialize the private key")
			}
			pubKey := pki.GetPublicKeyFromScalar(priKey)
			address := pki.GetAddressFromPublicKey(pubKey)
			StorePrivateKey(priKeyBytes)
			fmt.Printf("New account created:\nAddress: {%x}\n", address)
		case "list":
			for i, address := range ReadAddresses() {
				fmt.Printf("Account %d:\n  {%s}\n", i+1, address.Hex())
			}
		case "clearAll":
			ClearKeystore()
			fmt.Println("All existing accounts deleted...")
		case "import":
			accountImportCommand.Parse(os.Args[3:])
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
		case "showBalance":
			walletNode := CreateWalletServerNode()

			for _, address := range ReadAddresses() {
				fmt.Printf("Account %s:\n  %d ether \n", address.Hex(), FetchBalance(address, walletNode).Uint64()/params.Ether)

			}
		case "test":
			// Testing code
			priKey := pki.GetPrivateKeyScalarFromInt(444)
			address := pki.GetAddressFromPrivateKey(priKey)
			priKeyBytes, err := priKey.MarshalBinary()
			if err != nil {
				panic("Failed to deserialize private key scalar.")
			}
			fmt.Printf("Private Key :\n {%x}\n", priKeyBytes)
			fmt.Printf("Address :\n {%x}\n", address)
		}
	case "transfer":
		transferCommand.Parse(os.Args[2:])
		if !transferCommand.Parsed() {
			fmt.Println("Failed to parse flags")
		}
		sender := *transferSenderPtr
		receiver := *transferReceiverPtr
		amount := *transferAmountPtr

		if amount <= 0 {
			fmt.Println("Please specify positive amount to transfer")
		}
		priKeys := ReadPrivateKeys()
		if len(priKeys) == 0 {
			fmt.Println("No imported account to use.")
			return
		}
		senderIndex, err := strconv.Atoi(sender)
		senderAddress := ""
		addresses := ReadAddresses()
		if err != nil {
			senderIndex = -1
			for i, address := range addresses {
				if fmt.Sprintf("%x", address) == senderAddress {
					senderIndex = i
					break
				}
			}
			if senderIndex == -1 {
				fmt.Println("The specified sender account is not imported yet.")
				break
			}
		}
		if senderIndex >= len(priKeys) {
			fmt.Println("Sender account index out of bounds.")
			return
		}
		receiverAddress, err := hex.DecodeString(receiver)
		if err != nil || len(receiverAddress) != 20 {
			fmt.Println("The receiver address is not a valid.")
			return
		}

		// Generate transaction
		trimmedReceiverAddress := [20]byte{}
		copy(trimmedReceiverAddress[:], receiverAddress[:20])

		senderPriKey := priKeys[senderIndex]
		_ = senderPriKey

		// TODO: implement account transaction logic
	default:
		flag.PrintDefaults()
		os.Exit(1)
	}
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

// CreateWalletServerNode creates wallet server node.
func CreateWalletServerNode() *node.Node {
	configr := client_config.NewConfig()
	var shardIDLeaderMap map[uint32]p2p.Peer
	var clientPeer *p2p.Peer
	if true {
		configr.ReadConfigFile("local_config1.txt")
		shardIDLeaderMap = configr.GetShardIDToLeaderMap()
		clientPeer = configr.GetClientPeer()
	} else {
		shardIDLeaderMap = getShardIDToLeaderMap()
		clientPeer = &p2p.Peer{Port: "127.0.0.1", IP: "1234"}
	}
	host := p2pimpl.NewHost(*clientPeer)
	walletNode := node.New(host, nil, nil)
	walletNode.Client = client.NewClient(walletNode.GetHost(), &shardIDLeaderMap)
	return walletNode
}

// ExecuteTransaction issues the transaction to the Harmony network
func ExecuteTransaction(tx blockchain.Transaction, walletNode *node.Node) error {
	if tx.IsCrossShard() {
		walletNode.Client.PendingCrossTxsMutex.Lock()
		walletNode.Client.PendingCrossTxs[tx.ID] = &tx
		walletNode.Client.PendingCrossTxsMutex.Unlock()
	}

	msg := proto_node.ConstructTransactionListMessage([]*blockchain.Transaction{&tx})
	walletNode.BroadcastMessage(walletNode.Client.GetLeaders(), msg)

	doneSignal := make(chan int)
	go func() {
		for {
			if len(walletNode.Client.PendingCrossTxs) == 0 {
				doneSignal <- 0
				break
			}
		}
	}()

	select {
	case <-doneSignal:
		time.Sleep(100 * time.Millisecond)
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("Cross-shard Transaction processing timed out")
	}
}

// FetchBalance fetches account balance of specified address from the Harmony network
func FetchBalance(address common.Address, walletNode *node.Node) *big.Int {
	fmt.Println("Fetching account balance...")
	clients := []*client2.Client{}
	for _, leader := range *walletNode.Client.Leaders {
		clients = append(clients, client2.NewClient(leader.IP, "1841"))
	}

	balance := big.NewInt(0)
	for _, client := range clients {
		response := client.GetBalance(address)
		theirBalance := big.NewInt(0)
		theirBalance.SetBytes(response.Balance)
		balance.Add(balance, theirBalance)
	}
	return balance
}

// FetchUtxos fetches utxos of specified address from the Harmony network
func FetchUtxos(addresses [][20]byte, walletNode *node.Node) (map[uint32]blockchain.UtxoMap, error) {
	fmt.Println("Fetching account balance...")
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
