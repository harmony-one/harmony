package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/log"

	"github.com/harmony-one/harmony/api/client"
	clientService "github.com/harmony-one/harmony/api/client/service"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	p2p_host "github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/p2pimpl"

	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
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
	// New subcommands
	newCommand          = flag.NewFlagSet("new", flag.ExitOnError)
	newCommandNoPassPtr = newCommand.Bool("nopass", false, "The account has no pass phrase")

	// List subcommands
	listCommand = flag.NewFlagSet("list", flag.ExitOnError)

	// Export subcommands
	exportCommand           = flag.NewFlagSet("export", flag.ExitOnError)
	exportCommandAccountPtr = exportCommand.String("account", "", "The account to be exported")

	// Account subcommands
	accountImportCommand = flag.NewFlagSet("import", flag.ExitOnError)
	accountImportPtr     = accountImportCommand.String("privateKey", "", "Specify the private keyfile to import")
	accountImportPassPtr = accountImportCommand.String("pass", "", "Specify the passphrase of the private key")

	// Transfer subcommands
	transferCommand       = flag.NewFlagSet("transfer", flag.ExitOnError)
	transferSenderPtr     = transferCommand.String("from", "0", "Specify the sender account address or index")
	transferReceiverPtr   = transferCommand.String("to", "", "Specify the receiver account")
	transferAmountPtr     = transferCommand.Float64("amount", 0, "Specify the amount to transfer")
	transferShardIDPtr    = transferCommand.Int("shardID", 0, "Specify the shard ID for the transfer")
	transferInputDataPtr  = transferCommand.String("inputData", "", "Base64-encoded input data to embed in the transaction")
	transferSenderPassPtr = transferCommand.String("pass", "", "Passphrase of the sender's private key")

	freeTokenCommand    = flag.NewFlagSet("getFreeToken", flag.ExitOnError)
	freeTokenAddressPtr = freeTokenCommand.String("address", "", "Specify the account address to receive the free token")

	balanceCommand    = flag.NewFlagSet("getFreeToken", flag.ExitOnError)
	balanceAddressPtr = balanceCommand.String("address", "", "Specify the account address to check balance for")
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
		fmt.Println("    1. new           - Generates a new account and store the private key locally")
		fmt.Println("        --nopass         - The private key has no passphrase (for test only)")
		fmt.Println("    2. list          - Lists all accounts in local keystore")
		fmt.Println("    3. removeAll     - Removes all accounts in local keystore")
		fmt.Println("    4. import        - Imports a new account by private key")
		fmt.Println("        --pass           - The passphrase of the private key to import")
		fmt.Println("        --privateKey     - The private key to import")
		fmt.Println("    5. balances      - Shows the balances of all addresses or specific address")
		fmt.Println("        --address        - The address to check balance for")
		fmt.Println("    6. getFreeToken  - Gets free token on each shard")
		fmt.Println("        --address        - The free token receiver account's address")
		fmt.Println("    7. transfer      - Transfer token from one account to another")
		fmt.Println("        --from           - The sender account's address or index in the local keystore")
		fmt.Println("        --to             - The receiver account's address")
		fmt.Println("        --amount         - The amount of token to transfer")
		fmt.Println("        --shardID        - The shard Id for the transfer")
		fmt.Println("        --inputData      - Base64-encoded input data to embed in the transaction")
		fmt.Println("        --pass           - Passphrase of sender's private key")
		fmt.Println("    8. export        - Export account key to a new file")
		fmt.Println("        --account        - Specify the account to export. Empty will export every key.")
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
	case "new":
		processNewCommnad()
	case "list":
		processListCommand()
	case "export":
		processExportCommand()
	case "removeAll":
		clearKeystore()
	case "import":
		processImportCommnad()
	case "balances":
		readProfile(profile)
		processBalancesCommand()
	case "getFreeToken":
		readProfile(profile)
		processGetFreeToken()
	case "transfer":
		readProfile(profile)
		processTransferCommand()
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

func processNewCommnad() {
	newCommand.Parse(os.Args[2:])
	noPass := *newCommandNoPassPtr
	password := ""

	if !noPass {
		password = utils.AskForPassphrase("Passphrase: ")
		password2 := utils.AskForPassphrase("Passphrase again: ")
		if password != password2 {
			fmt.Printf("Passphrase doesn't match. Please try again!\n")
			os.Exit(3)
		}
	}

	account, err := ks.NewAccount(password)
	if err != nil {
		fmt.Printf("new account error: %v\n", err)
	}
	fmt.Printf("account: %s\n", account.Address.Hex())
	fmt.Printf("URL: %s\n", account.URL)
}

func _exportAccount(account accounts.Account) {
	fmt.Printf("account: %s\n", account.Address.Hex())
	fmt.Printf("URL: %s\n", account.URL)
	pass := utils.AskForPassphrase("Original Passphrase: ")
	newpass := utils.AskForPassphrase("Export Passphrase: ")

	data, err := ks.Export(account, pass, newpass)
	if err == nil {
		filename := fmt.Sprintf(".hmy/%s.key", account.Address.Hex())
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic("Failed to open keystore")
		}
		_, err = f.Write(data)
		if err != nil {
			panic("Failed to write to keystore")
		}
		f.Close()
		fmt.Printf("Exported keyfile to: %v\n", filename)
		return
	}
	switch err {
	case accounts.ErrInvalidPassphrase:
		fmt.Println("Invalid Passphrase")
	default:
		fmt.Printf("export error: %v\n", err)
	}
}

func processListCommand() {
	listCommand.Parse(os.Args[2:])

	allAccounts := ks.Accounts()
	for _, account := range allAccounts {
		fmt.Printf("account: %s\n", account.Address.Hex())
		fmt.Printf("URL: %s\n", account.URL)
	}
}

func processExportCommand() {
	exportCommand.Parse(os.Args[2:])
	acc := *exportCommandAccountPtr

	allAccounts := ks.Accounts()
	for _, account := range allAccounts {
		if acc == "" || acc == account.Address.Hex() {
			_exportAccount(account)
		}
	}
}

func processImportCommnad() {
	accountImportCommand.Parse(os.Args[2:])
	priKey := *accountImportPtr
	if priKey == "" {
		fmt.Println("Error: --privateKey is required")
		return
	}
	if !accountImportCommand.Parsed() {
		fmt.Println("Failed to parse flags")
	}
	pass := *accountImportPassPtr

	data, err := ioutil.ReadFile(priKey)
	if err != nil {
		panic("Failed to readfile")
	}

	account, err := ks.Import(data, pass, pass)
	if err != nil {
		panic("Failed to import the private key")
	}
	fmt.Printf("Private key imported for account: %s\n", account.Address.Hex())
}

func processBalancesCommand() {
	balanceCommand.Parse(os.Args[2:])

	if *balanceAddressPtr == "" {
		allAccounts := ks.Accounts()
		for i, account := range allAccounts {
			fmt.Printf("Account %d:\n", i)
			fmt.Printf("    Address: %s\n", account.Address.Hex())
			for shardID, balanceNonce := range FetchBalance(account.Address) {
				fmt.Printf("    Balance in Shard %d:  %s, nonce: %v \n", shardID, convertBalanceIntoReadableFormat(balanceNonce.balance), balanceNonce.nonce)
			}
		}
	} else {
		address := common.HexToAddress(*balanceAddressPtr)
		fmt.Printf("Account: %s:\n", address.Hex())
		for shardID, balanceNonce := range FetchBalance(address) {
			fmt.Printf("    Balance in Shard %d:  %s, nonce: %v \n", shardID, convertBalanceIntoReadableFormat(balanceNonce.balance), balanceNonce.nonce)
		}
	}
}

func processGetFreeToken() {
	freeTokenCommand.Parse(os.Args[2:])

	if *freeTokenAddressPtr == "" {
		fmt.Println("Error: --address is required")
	} else {
		address := common.HexToAddress(*freeTokenAddressPtr)
		GetFreeToken(address)
	}
}

func processTransferCommand() {
	transferCommand.Parse(os.Args[2:])
	if !transferCommand.Parsed() {
		fmt.Println("Failed to parse flags")
		return
	}
	sender := *transferSenderPtr
	receiver := *transferReceiverPtr
	amount := *transferAmountPtr
	shardID := *transferShardIDPtr
	base64InputData := *transferInputDataPtr
	senderPass := *transferSenderPassPtr

	inputData, err := base64.StdEncoding.DecodeString(base64InputData)
	if err != nil {
		fmt.Printf("Cannot base64-decode input data (%s): %s\n",
			base64InputData, err)
		return
	}

	if shardID == -1 {
		fmt.Println("Please specify the shard ID for the transfer (e.g. --shardID=0)")
		return
	}
	if amount <= 0 {
		fmt.Println("Please specify positive amount to transfer")
		return
	}

	receiverAddress := common.HexToAddress(receiver)
	if len(receiverAddress) != 20 {
		fmt.Println("The receiver address is not valid.")
		return
	}

	senderAddress := common.HexToAddress(sender)
	if len(senderAddress) != 20 {
		fmt.Println("The sender address is not valid.")
		return
	}

	walletNode := createWalletNode()

	shardIDToAccountState := FetchBalance(senderAddress)

	state, ok := shardIDToAccountState[uint32(shardID)]
	if !ok {
		fmt.Printf("Failed connecting to the shard %d\n", shardID)
		return
	}

	balance := state.balance
	balance = balance.Div(balance, big.NewInt(denominations.Nano))
	if amount > float64(balance.Uint64())/denominations.Nano {
		fmt.Printf("Balance is not enough for the transfer, current balance is %.6f\n", float64(balance.Uint64())/denominations.Nano)
		return
	}

	amountBigInt := big.NewInt(int64(amount * denominations.Nano))
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.Nano))
	gas, err := core.IntrinsicGas(inputData, false, true)
	if err != nil {
		fmt.Printf("cannot calculate required gas: %v\n", err)
		return
	}

	tx := types.NewTransaction(
		state.nonce, receiverAddress, uint32(shardID), amountBigInt,
		gas, nil, inputData)

	account, err := ks.Find(accounts.Account{Address: senderAddress})
	if err != nil {
		fmt.Printf("Find Account Error: %v\n", err)
		return
	}

	err = ks.Unlock(account, senderPass)
	if err != nil {
		fmt.Printf("Unlock account failed! %v\n", err)
		return
	}

	fmt.Printf("Unlock account succeeded! '%v'\n", senderPass)

	tx, err = ks.SignTx(account, tx, nil)
	if err != nil {
		fmt.Printf("SignTx Error: %v\n", err)
		return
	}

	submitTransaction(tx, walletNode, uint32(shardID))
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
func FetchBalance(address common.Address) map[uint32]AccountState {
	result := make(map[uint32]AccountState)
	for i := 0; i < walletProfile.Shards; i++ {
		balance := big.NewInt(0)
		var nonce uint64

		result[uint32(i)] = AccountState{balance, 0}

		for retry := 0; retry < rpcRetry; retry++ {
			server := walletProfile.RPCServer[i][rand.Intn(len(walletProfile.RPCServer[i]))]
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
			balance.SetBytes(response.Balance)
			nonce = response.Nonce
			break
		}
		result[uint32(i)] = AccountState{balance, nonce}
	}
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
		os.RemoveAll(path.Join([]string{keystoreDir, d.Name()}...))
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
