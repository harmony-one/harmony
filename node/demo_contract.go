package node

// CreateTransactionForEnterMethod creates transaction to call enter method of lottery contract.
import (
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/contracts"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	contract_constants "github.com/harmony-one/harmony/internal/utils/contract"
)

// Constants for lottery.
const (
	Enter      = "enter"
	GetPlayers = "getPlayers"
)

// AddLotteryContract adds the demo lottery contract the genesis block.
func (node *Node) AddLotteryContract() {
	// Add a lottery demo contract.
	priKey, err := crypto.HexToECDSA(contract_constants.DemoAccounts[0].Private)
	if err != nil {
		utils.GetLogInstance().Error("Error when creating private key for demo contract")
		// Exit here to recognize the coding working.
		// Basically we will remove this logic when launching so it's fine for now.
		os.Exit(1)
	}

	dataEnc := common.FromHex(contracts.LotteryBin)
	// Unsigned transaction to avoid the case of transaction address.

	contractFunds := big.NewInt(0)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	demoContract, _ := types.SignTx(
		types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*10, nil, dataEnc),
		types.HomesteadSigner{},
		priKey)
	node.DemoContractAddress = crypto.CreateAddress(crypto.PubkeyToAddress(priKey.PublicKey), uint64(0))
	node.addPendingTransactions(types.Transactions{demoContract})
}

// CreateTransactionForEnterMethod generates transaction for enter method and add it into pending tx list.
func (node *Node) CreateTransactionForEnterMethod(amount int64, priKey string) {
	var err error
	toAddress := node.DemoContractAddress

	abi, err := abi.JSON(strings.NewReader(contracts.LotteryABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
	}
	bytesData, err := abi.Pack(Enter)
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
	}

	key, err := crypto.HexToECDSA(priKey)
	nonce := node.GetNonceOfAddress(crypto.PubkeyToAddress(key.PublicKey))
	tx := types.NewTransaction(
		nonce,
		toAddress,
		0,
		big.NewInt(amount),
		params.TxGas*10,
		nil,
		bytesData,
	)

	if err != nil {
		utils.GetLogInstance().Error("Failed to get private key", "error", err)
	}
	if signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key); err == nil {
		node.addPendingTransactions(types.Transactions{signedTx})
	} else {
		utils.GetLogInstance().Error("Unable to call enter method", "error", err)
	}
}
