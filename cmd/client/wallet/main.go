package main

import (
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/fatih/color"
	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
	"github.com/harmony-one/harmony/api/client"
	clientService "github.com/harmony-one/harmony/api/client/service"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/blsgen"
	common2 "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/wallet"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	p2p_host "github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	p2putils "github.com/harmony-one/harmony/p2p/utils"
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
	defaultProfile    = "main"
	keystoreDir       = ".hmy/keystore"
)

var (
	// New subcommands
	newCommand          = flag.NewFlagSet("new", flag.ExitOnError)
	newCommandNoPassPtr = newCommand.Bool("nopass", false, "The account has no pass phrase")
	// -pass takes on "pass:password", "env:var", "file:pathname",
	// "fd:number", or "stdin" form.
	// See “PASS PHRASE ARGUMENTS” section of openssl(1) for details.
	newCommandPassPtr = newCommand.String("pass", "", "how to get passphrase for the key")

	// List subcommands
	listCommand = flag.NewFlagSet("list", flag.ExitOnError)

	// Export subcommands
	exportCommand           = flag.NewFlagSet("export", flag.ExitOnError)
	exportCommandAccountPtr = exportCommand.String("account", "", "The account to be exported")

	// ExportPriKey subcommands
	exportPriKeyCommand           = flag.NewFlagSet("exportPriKey", flag.ExitOnError)
	exportPriKeyCommandAccountPtr = exportPriKeyCommand.String("account", "", "The account whose private key to be exported")

	// Account subcommands
	accountImportCommand = flag.NewFlagSet("import", flag.ExitOnError)
	accountImportPtr     = accountImportCommand.String("privateKey", "", "Specify the private keyfile to import")
	accountImportPassPtr = accountImportCommand.String("pass", "", "Specify the passphrase of the private key")

	// Transfer subcommands
	transferCommand       = flag.NewFlagSet("transfer", flag.ExitOnError)
	transferSenderPtr     = transferCommand.String("from", "0", "Specify the sender account address or index")
	transferReceiverPtr   = transferCommand.String("to", "", "Specify the receiver account")
	transferAmountPtr     = transferCommand.Float64("amount", 0, "Specify the amount to transfer")
	transferGasPricePtr   = transferCommand.Uint64("gasPrice", 1, "Specify the gas price amount. Unit is Nano.")
	transferShardIDPtr    = transferCommand.Int("shardID", -1, "Specify the shard ID for the transfer")
	transferToShardIDPtr  = transferCommand.Int("toShardID", -1, "Specify the destination shard ID for the transfer")
	transferInputDataPtr  = transferCommand.String("inputData", "", "Base64-encoded input data to embed in the transaction")
	transferSenderPassPtr = transferCommand.String("pass", "", "Passphrase of the sender's private key")
	waitForColoredBalance = transferCommand.Bool("waitThenBal", false, "")

	freeTokenCommand    = flag.NewFlagSet("getFreeToken", flag.ExitOnError)
	freeTokenAddressPtr = freeTokenCommand.String("address", "", "Specify the account address to receive the free token")

	balanceCommand    = flag.NewFlagSet("balances", flag.ExitOnError)
	balanceAddressPtr = balanceCommand.String("address", "", "Specify the account address to check balance for")

	formatCommand    = flag.NewFlagSet("format", flag.ExitOnError)
	formatAddressPtr = formatCommand.String("address", "", "Specify the account address to display different encoding formats")

	blsrecoveryCommand = flag.NewFlagSet("blsRecovery", flag.ExitOnError)
	blsPass            = blsrecoveryCommand.String("pass", "", "Passphrase to decrypt the bls file.")
	blsFile            = blsrecoveryCommand.String("file", "", "Non-human readable bls file.")

	blsImportCommand = flag.NewFlagSet("importBLS", flag.ExitOnError)
	blsKey           = blsImportCommand.String("key", "", "The raw private key.")

	getBLSPublicCommand = flag.NewFlagSet("getBLSPublic", flag.ExitOnError)
	blsKey2             = getBLSPublicCommand.String("key", "", "The raw private key.")
	blsFile2            = getBLSPublicCommand.String("file", "", "The encrypted bls file.")
)

var (
	networkType   string
	walletProfile *wallet.Profile
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
		fmt.Println("    -p profile       - Specify the profile of the wallet, either main, local, beta, or pangaea. Default is main.")
		fmt.Println("                       The profile is in file:", defaultConfigFile)
		fmt.Println()
		fmt.Println("Actions:")
		fmt.Println("    1. new           - Generates a new account and store the private key locally")
		fmt.Println("        --nopass         - The private key has no passphrase (for test only)")
		fmt.Println("        --pass           - The passphrase for the private key, in the format of: pass:password, env:var, file:pathname, fd:number, or stdin")
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
		fmt.Println("        --toShardID      - The destination shard Id for the transfer")
		fmt.Println("        --inputData      - Base64-encoded input data to embed in the transaction")
		fmt.Println("        --pass           - Passphrase of sender's private key")
		fmt.Println("        --waitThenBal    - Wait after the transfer with colored balances output")
		fmt.Println("    8. export        - Export account key to a new file")
		fmt.Println("        --account        - Specify the account to export. Empty will export every key.")
		fmt.Println("    9. exportPriKey  - Export account private key")
		fmt.Println("        --account        - Specify the account to export private key.")
		fmt.Println("   10. blsgen        - Generate a bls key and store private key locally.")
		fmt.Println("        --nopass         - The private key has no passphrase (for test only)")
		fmt.Println("   11. format        - Shows different encoding formats of specific address")
		fmt.Println("        --address        - The address to display the different encoding formats for")
		fmt.Println("   12. blsRecovery    - Recover non-human readable file.")
		fmt.Println("        --pass           - The file containg the passphrase to decrypt the bls key.")
		fmt.Println("        --file           - Non-human readable bls file.")
		fmt.Println("   13. importBLS      - Convert raw private key into encrypted bls key.")
		fmt.Println("        --key            - Raw private key.")
		fmt.Println("   14. getBLSPublic   - Show BLS public key given raw private bls key.")
		fmt.Println("        --key            - Raw private key.")
		fmt.Println("        --file           - encrypted bls file.")
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
	case "exportPriKey":
		processExportPriKeyCommand()
	case "blsgen":
		processBLSgenCommand()
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
	case "format":
		formatAddressCommand()
	case "blsRecovery":
		blsRecoveryCommand()
	case "importBLS":
		importBLS()
	case "getBLSPublic":
		getBLSPublic()
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

	walletProfile, err = wallet.ReadProfile(iniBytes, profile)
	if err != nil {
		fmt.Printf("Read wallet profile error: %v\nExiting ...\n", err)
		os.Exit(2)
	}
}

// createWalletNode creates wallet server node.
func createWalletNode() *node.Node {
	bootNodeAddrs, err := p2putils.StringsToAddrs(walletProfile.Bootnodes)
	if err != nil {
		utils.FatalErrMsg(err, "cannot parse bootnodes %#v", walletProfile.Bootnodes)
	}
	p2putils.BootNodes = bootNodeAddrs
	shardID := 0
	// dummy host for wallet
	// TODO: potentially, too many dummy IP may flush out good IP address from our bootnode DHT
	// we need to understand the impact to bootnode DHT with this dummy host ip added
	port := fmt.Sprintf("%d", 16999+rand.Intn(1000))
	self := p2p.Peer{IP: "127.0.0.1", Port: port}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", port)
	host, err := p2pimpl.NewHost(&self, priKey)
	if err != nil {
		utils.FatalErrMsg(err, "cannot initialize network")
	}
	chainDBFactory := &shardchain.MemDBFactory{}
	w := node.New(host, nil, chainDBFactory, nil, false)
	w.Client = client.NewClient(w.GetHost(), uint32(shardID))

	w.NodeConfig.SetRole(nodeconfig.ClientNode)
	netType := nodeconfig.NetworkType(walletProfile.Network)
	nodeconfig.SetNetworkType(netType)
	w.ServiceManagerSetup()
	w.RunServices()
	return w
}

func processNewCommnad() {
	if err := newCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}
	noPass := *newCommandNoPassPtr
	pass := *newCommandPassPtr
	password := ""

	if !noPass {
		if pass == "" {
			password = utils.AskForPassphrase("Passphrase: ")
			password2 := utils.AskForPassphrase("Passphrase again: ")
			if password != password2 {
				fmt.Printf("Passphrase doesn't match. Please try again!\n")
				os.Exit(3)
			}
		} else if newPass, err := utils.GetPassphraseFromSource(pass); err != nil {
			fmt.Printf("Cannot read passphrase: %s\n", err)
			os.Exit(3)
		} else {
			password = newPass
		}
	}

	account, err := ks.NewAccount(password)
	if err != nil {
		fmt.Printf("new account error: %v\n", err)
	}
	fmt.Printf("account: %s\n", common2.MustAddressToBech32(account.Address))
	fmt.Printf("URL: %s\n", account.URL)
}

func _exportAccount(account accounts.Account) {
	fmt.Printf("account: %s\n", common2.MustAddressToBech32(account.Address))
	fmt.Printf("URL: %s\n", account.URL)
	pass := utils.AskForPassphrase("Original Passphrase: ")
	newpass := utils.AskForPassphrase("Export Passphrase: ")

	data, err := ks.Export(account, pass, newpass)
	if err == nil {
		filename := fmt.Sprintf(".hmy/%s.key", common2.MustAddressToBech32(account.Address))
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			utils.FatalErrMsg(err, "cannot open exported key file %s", filename)
		}
		_, err = f.Write(data)
		if err != nil {
			utils.FatalErrMsg(err, "cannot write to exported key file %s", filename)
		}
		if err := f.Close(); err != nil {
			fmt.Println(ctxerror.New("cannot close key file",
				"filename", filename).WithCause(err))
		}
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

func _exportPriKeyAccount(account accounts.Account) {
	fmt.Printf("account: %s\n", common2.MustAddressToBech32(account.Address))
	fmt.Printf("URL: %s\n", account.URL)
	pass := utils.AskForPassphrase("Original Passphrase: ")

	account, key, err := ks.GetDecryptedKey(account, pass)
	if err != nil {
		fmt.Printf("Failed to decrypt the account: %s \n", err)
	} else {
		fmt.Printf("Private key: %s \n", hex.EncodeToString(key.PrivateKey.D.Bytes()))
	}
}

func processListCommand() {
	if err := listCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}

	allAccounts := ks.Accounts()
	for _, account := range allAccounts {
		fmt.Printf("account: %s\n", common2.MustAddressToBech32(account.Address))
		fmt.Printf("URL: %s\n", account.URL)
	}
}

func processExportCommand() {
	if err := exportCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}
	acc := *exportCommandAccountPtr

	allAccounts := ks.Accounts()
	for _, account := range allAccounts {
		if acc == "" || common2.ParseAddr(acc) == account.Address {
			_exportAccount(account)
		}
	}
}

func processExportPriKeyCommand() {
	if err := exportCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}
	acc := *exportCommandAccountPtr

	allAccounts := ks.Accounts()
	for _, account := range allAccounts {
		if acc == "" || common2.ParseAddr(acc) == account.Address {
			_exportPriKeyAccount(account)
		}
	}
}

func processBLSgenCommand() {
	newCommand.Parse(os.Args[2:])
	noPass := *newCommandNoPassPtr
	pass := *newCommandPassPtr
	// Default password is an empty string
	password := ""

	if !noPass {
		if pass == "" {
			password = utils.AskForPassphrase("Passphrase: ")
			password2 := utils.AskForPassphrase("Passphrase again: ")
			if password != password2 {
				fmt.Printf("Passphrase doesn't match. Please try again!\n")
				os.Exit(3)
			}
		} else if newPass, err := utils.GetPassphraseFromSource(pass); err != nil {
			fmt.Printf("Cannot read passphrase: %s\n", err)
			os.Exit(3)
		} else {
			password = newPass
		}
	}

	privateKey, fileName, err := blsgen.GenBLSKeyWithPassPhrase(password)
	if err != nil {
		fmt.Printf("error when generating bls key: %v\n", err)
		os.Exit(100)
	}
	publickKey := privateKey.GetPublicKey()
	fmt.Printf("BLS private key: %s\n", privateKey.SerializeToHexStr())
	fmt.Printf("BLS public key: %s\n", publickKey.SerializeToHexStr())
	fmt.Printf("File storing the ENCRYPTED private key with your passphrase: %s\n", fileName)
}

func processImportCommnad() {
	if err := accountImportCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}
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
		utils.FatalErrMsg(err, "cannot load private key file %s", priKey)
	}

	account, err := ks.Import(data, pass, pass)
	if err != nil {
		utils.FatalErrMsg(err, "cannot import private key in file %s", priKey)
	}
	fmt.Printf("Private key imported for account: %s\n", common2.MustAddressToBech32(account.Address))
}

func showAllBalances(sender, receiver string, fromS, toS int) {
	allAccounts := ks.Accounts()
	for i, account := range allAccounts {
		asBech32 := common2.MustAddressToBech32(account.Address)
		fmt.Printf("Account %d:\n", i)
		if asBech32 == sender {
			color.Yellow("    Address: %s\n", asBech32)
		} else if asBech32 == receiver {
			color.Cyan("    Address: %s\n", asBech32)
		} else {
			fmt.Printf("    Address: %s\n", asBech32)
		}
		for shardID, balanceNonce := range FetchBalance(account.Address) {
			if balanceNonce != nil {
				if shardID == fromS && asBech32 == sender {
					color.Yellow("    Balance in Shard %d:  %s, nonce: %v \n", shardID, convertBalanceIntoReadableFormat(balanceNonce.balance), balanceNonce.nonce)
				} else if shardID == toS && asBech32 == receiver {
					color.Cyan("    Balance in Shard %d:  %s, nonce: %v \n", shardID, convertBalanceIntoReadableFormat(balanceNonce.balance), balanceNonce.nonce)
				} else {
					fmt.Printf("    Balance in Shard %d:  %s, nonce: %v \n", shardID, convertBalanceIntoReadableFormat(balanceNonce.balance), balanceNonce.nonce)
				}

			} else {
				fmt.Printf("    Balance in Shard %d:  connection failed", shardID)
			}
		}
	}
}

func processBalancesCommand() {
	if err := balanceCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}
	if *balanceAddressPtr == "" {
		showAllBalances("", "", -1, -1)
	} else {
		address := common2.ParseAddr(*balanceAddressPtr)
		valid, errorMessage := validateAddress(*balanceAddressPtr, address, "")

		if !valid && len(errorMessage) > 0 {
			fmt.Println(errorMessage)
			return
		}

		fmt.Printf("Account: %s:\n", common2.MustAddressToBech32(address))
		for shardID, balanceNonce := range FetchBalance(address) {
			if balanceNonce != nil {
				fmt.Printf("    Balance in Shard %d:  %s, nonce: %v \n", shardID, convertBalanceIntoReadableFormat(balanceNonce.balance), balanceNonce.nonce)
			} else {
				fmt.Printf("    Balance in Shard %d:  connection failed \n", shardID)
			}
		}
	}
}

func formatAddressCommand() {
	if err := formatCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}

	if *formatAddressPtr == "" {
		fmt.Println("Please specify the --address to show formats for.")
	} else {
		address := common2.ParseAddr(*formatAddressPtr)
		valid, errorMessage := validateAddress(*formatAddressPtr, address, "")

		if !valid && len(errorMessage) > 0 {
			fmt.Println(errorMessage)
			return
		}

		fmt.Printf("account address in Bech32: %s\n", common2.MustAddressToBech32(address))
		fmt.Printf("account address in Base16 (deprecated): %s\n", address.Hex())
	}
}

func blsRecoveryCommand() {
	if err := blsrecoveryCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}

	if *blsPass == "" || *blsFile == "" {
		fmt.Println("Please specify the --file and --pass for bls passphrase.")
	} else {
		priKey, err := blsgen.LoadNonHumanReadableBLSKeyWithPassPhrase(*blsFile, *blsPass)
		if err != nil {
			fmt.Printf("Not able to load non-human readable bls key. err:%v", err)
			os.Exit(100)
		}
		fileName, err := blsgen.WritePriKeyWithPassPhrase(priKey, *blsPass)
		if err != nil {
			fmt.Println("Error to read non-human readable bls")
		}
		fmt.Printf("Generated human readabled bls key and wrote at %s. Please use the same passphrase to decrypt the private key.\n", fileName)
	}
}

func importBLS() {
	if err := blsImportCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}

	if *blsKey != "" {
		privateKey := &ffi_bls.SecretKey{}
		err := privateKey.DeserializeHexStr(*blsKey)
		if err != nil {
			fmt.Printf("Your raw private key is not valid.\n Err: %v\n", err)
			os.Exit(101)
		}
		passphrase := utils.AskForPassphrase("Passphrase: ")
		passphrase2 := utils.AskForPassphrase("Passphrase again: ")
		if passphrase != passphrase2 {
			fmt.Printf("Passphrase doesn't match. Please try again!\n")
			os.Exit(3)
		}

		fileName, err := blsgen.WritePriKeyWithPassPhrase(privateKey, passphrase)
		if err != nil {
			fmt.Printf("Can not generate encrypted bls file.\n Err: %v\n", err)
			os.Exit(101)
		} else {
			fmt.Printf("Your encrypted bls file with the passphrased is written at %s.\n", fileName)
		}
		privateKey2, err := blsgen.LoadBLSKeyWithPassPhrase(fileName, passphrase)
		if err != nil {
			fmt.Printf("Error when loading the private key with the passphrase. Err: %v", err)
			os.Exit(101)
		}
		if !privateKey.IsEqual(privateKey2) {
			fmt.Println("The loaded private key does not match your private key.")
			os.Exit(101)
		}
		fmt.Println("We loaded the file and the loaded private key DOES match your private key!")
	} else {
		fmt.Println("Please specify the hexadecimal private key string using --key")
	}
}

func getBLSPublic() {
	if err := getBLSPublicCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("failed to parse flags").WithCause(err))
		return
	}

	if *blsKey2 != "" {
		privateKey := &ffi_bls.SecretKey{}
		err := privateKey.DeserializeHexStr(*blsKey2)
		if err != nil {
			fmt.Printf("Your raw private key is not valid.\n Err: %v\n", err)
			os.Exit(101)
		}
		fmt.Printf("Your bls public key is: %s\n", privateKey.GetPublicKey().SerializeToHexStr())
	} else if *blsFile2 != "" {
		password := utils.AskForPassphrase("Passphrase: ")
		password2 := utils.AskForPassphrase("Passphrase again: ")
		if password != password2 {
			fmt.Printf("Passphrase doesn't match. Please try again!\n")
			os.Exit(100)
		}
		privateKey, err := blsgen.LoadBLSKeyWithPassPhrase(*blsFile2, password)
		if err != nil {
			fmt.Printf("error when loading bls key, err :%v\n", err)
			os.Exit(100)
		}
		fmt.Printf("Your bls public key is: %s\n", privateKey.GetPublicKey().SerializeToHexStr())
	} else {
		fmt.Println("Please specify the hexadecimal private key string using --key")
	}
}

func processGetFreeToken() {
	if err := freeTokenCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("Failed to parse flags").WithCause(err))
		return
	}

	if *freeTokenAddressPtr == "" {
		fmt.Println("Error: --address is required")
	} else {
		address := common2.ParseAddr(*freeTokenAddressPtr)
		valid, errorMessage := validateAddress(*freeTokenAddressPtr, address, "")

		if !valid && len(errorMessage) > 0 {
			fmt.Println(errorMessage)
			return
		}

		GetFreeToken(address)
	}
}

func processTransferCommand() {
	if err := transferCommand.Parse(os.Args[2:]); err != nil {
		fmt.Println(ctxerror.New("Failed to parse flags").WithCause(err))
		return
	}
	sender := *transferSenderPtr
	receiver := *transferReceiverPtr
	amount := *transferAmountPtr
	gasPrice := *transferGasPricePtr
	shardID := *transferShardIDPtr
	toShardID := *transferToShardIDPtr
	base64InputData := *transferInputDataPtr
	senderPass := *transferSenderPassPtr

	inputData, err := base64.StdEncoding.DecodeString(base64InputData)
	if err != nil {
		fmt.Printf("Cannot base64-decode input data (%s): %s\n",
			base64InputData, err)
		return
	}

	if !validShard(shardID, walletProfile.Shards) {
		fmt.Println("Please specify a valid sender shard ID for the transfer (e.g. --shardID=0)")
		return
	}

	if !validShard(toShardID, walletProfile.Shards) {
		fmt.Println("Please specify a valid receiver shard ID for the transfer (e.g. --toShardID=0)")
		return
	}

	if amount <= 0 {
		fmt.Println("Please specify positive amount to transfer")
		return
	}

	senderAddress := common2.ParseAddr(sender)
	valid, errorMessage := validateAddress(sender, senderAddress, "sender")

	if !valid && len(errorMessage) > 0 {
		fmt.Println(errorMessage)
		return
	}

	receiverAddress := common2.ParseAddr(receiver)
	valid, errorMessage = validateAddress(receiver, receiverAddress, "receiver")

	if !valid && len(errorMessage) > 0 {
		fmt.Println(errorMessage)
		return
	}

	shardIDToAccountState := FetchBalance(senderAddress)

	state := shardIDToAccountState[shardID]
	if state == nil {
		fmt.Printf("Failed connecting to the shard %d\n", shardID)
		return
	}

	balance := state.balance
	balance = balance.Div(balance, big.NewInt(denominations.Nano))
	if amount > float64(balance.Uint64())/denominations.Nano {
		fmt.Printf("Balance is not enough for the transfer, current balance is %.6f\n", float64(balance.Uint64())/denominations.Nano)
		return
	}

	walletNode := createWalletNode()

	amountBigInt := big.NewInt(int64(amount * denominations.Nano))
	amountBigInt = amountBigInt.Mul(amountBigInt, big.NewInt(denominations.Nano))
	gas, err := core.IntrinsicGas(inputData, false, true, false)
	if err != nil {
		fmt.Printf("cannot calculate required gas: %v\n", err)
		return
	}

	fromShard := uint32(shardID)
	toShard := uint32(toShardID)
	var tx *types.Transaction

	gasPriceBigInt := big.NewInt(int64(gasPrice))
	gasPriceBigInt = gasPriceBigInt.Mul(gasPriceBigInt, big.NewInt(denominations.Nano))

	tx = types.NewCrossShardTransaction(
		state.nonce, &receiverAddress, fromShard, toShard, amountBigInt,
		gas, gasPriceBigInt, inputData)

	account, err := ks.Find(accounts.Account{Address: senderAddress})
	if err != nil {
		fmt.Printf("Find Account Error: %v\n", err)
		return
	}

	newPass, err := utils.GetPassphraseFromSource(senderPass)
	if err != nil {
		fmt.Printf("Cannot read passphrase: %s\n", err)
		os.Exit(3)
	}

	err = ks.Unlock(account, newPass)
	if err != nil {
		fmt.Printf("Unlock account failed! %v\n", err)
		return
	}

	fmt.Printf("Unlock account succeeded! '%v'\n", senderPass)

	chainID, ok := new(big.Int).SetString(walletProfile.ChainID, 0)
	if !ok {
		fmt.Printf("invalid chain ID %#v in config", walletProfile.ChainID)
		return
	}
	tx, err = ks.SignTx(account, tx, chainID)
	if err != nil {
		fmt.Printf("SignTx Error: %v\n", err)
		return
	}

	if err := submitTransaction(tx, walletNode, uint32(shardID)); err != nil {
		fmt.Println(ctxerror.New("submitTransaction failed",
			"tx", tx, "shardID", shardID).WithCause(err))
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
			balance := big.NewInt(-1)
			var nonce uint64

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
			if balance.Cmp(big.NewInt(-1)) > 0 {
				result[shardID] = &AccountState{balance, nonce}
			}
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
		utils.FatalErrMsg(err, "cannot read keystore directory %s", keystoreDir)
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
	clientGroup := nodeconfig.NewClientGroupIDByShardID(nodeconfig.ShardID(shardID))

	err := walletNode.GetHost().SendMessageToGroups([]nodeconfig.GroupID{clientGroup}, p2p_host.ConstructP2pMessage(byte(0), msg))
	if err != nil {
		fmt.Printf("Error in SubmitTransaction: %v\n", err)
		return err
	}
	fmt.Printf("Transaction Id for shard %d: %s\n", int(shardID), tx.Hash().Hex())
	json, err := tx.MarshalJSON()
	fmt.Printf("Transaction Submitted for shard %d: %s\n", int(shardID), string(json))
	// FIXME (leo): how to we know the tx was successful sent to the network
	// this is a hacky way to wait for sometime
	time.Sleep(3 * time.Second)
	if *waitForColoredBalance {
		sender := *transferSenderPtr
		receiver := *transferReceiverPtr
		shardID := *transferShardIDPtr
		toShardID := *transferToShardIDPtr
		showAllBalances(sender, receiver, shardID, toShardID)
	}

	return nil
}

var (
	addressValidationRegexp = regexp.MustCompile(`(?i)^(one[a-zA-Z0-9]{39})|(0x[a-fA-F0-9]{40})`)
)

func validateAddress(address string, commonAddress common.Address, addressType string) (bool, string) {
	var valid = true
	var errorMessage string

	if len(addressType) > 0 {
		addressType = fmt.Sprintf("%s ", addressType)
	}

	matches := addressValidationRegexp.FindAllStringSubmatch(address, -1)
	if len(matches) == 0 || len(commonAddress) != 20 {
		valid = false
		errorMessage = fmt.Sprintf("The %saddress you supplied (%s) is in an invalid format. Please provide a valid address.", addressType, address)
	}

	return valid, errorMessage
}

func validShard(shardID int, shardCount int) bool {
	if shardID < 0 || shardID > (shardCount-1) {
		return false
	}

	return true
}
