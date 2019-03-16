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
	node.DemoContractPrivateKey = priKey
	node.addPendingTransactions(types.Transactions{demoContract})
}

// CreateTransactionForEnterMethod generates transaction for enter method and add it into pending tx list.
func (node *Node) CreateTransactionForEnterMethod(amount int64, priKey string) error {
	var err error
	toAddress := node.DemoContractAddress

	abi, err := abi.JSON(strings.NewReader(contracts.LotteryABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
		return err
	}
	bytesData, err := abi.Pack(Enter)
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
		return err
	}

	key, err := crypto.HexToECDSA(priKey)
	nonce := node.GetNonceOfAddress(crypto.PubkeyToAddress(key.PublicKey))
	Amount := big.NewInt(amount)
	Amount = Amount.Mul(Amount, big.NewInt(params.Ether))
	tx := types.NewTransaction(
		nonce,
		toAddress,
		0,
		Amount,
		params.TxGas*10,
		nil,
		bytesData,
	)

	if err != nil {
		utils.GetLogInstance().Error("Failed to get private key", "error", err)
		return err
	}
	if signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key); err == nil {
		node.addPendingTransactions(types.Transactions{signedTx})
		return nil
	}
	utils.GetLogInstance().Error("Unable to call enter method", "error", err)
	return err
}

// GetResult2 get current players and their balances.
func (node *Node) GetResult2() (players []string, balances []uint64) {
	for _, account := range contract_constants.DemoAccounts {
		players = append(players, account.Address)
		key, err := crypto.HexToECDSA(account.Private)
		if err != nil {
			utils.GetLogInstance().Error("Error when HexToECDSA")
		}
		address := crypto.PubkeyToAddress(key.PublicKey)
		balances = append(balances, node.GetBalanceOfAddress(address))
	}
	return players, balances
}

// GetResult get current players and their balances.
func (node *Node) GetResult() (players []string, balances []uint64) {
	return node.GetResult2()
	// abi, err := abi.JSON(strings.NewReader(contracts.LotteryABI))
	// if err != nil {
	// 	utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
	// }
	// bytesData, err := abi.Pack("getPlayers")
	// if err != nil {
	// 	utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
	// }

	// demoContractAddress := node.DemoContractAddress
	// demoContractPrivateKey := node.DemoContractPrivateKey

	// tx := types.NewTransaction(
	// 	node.GetNonceOfAddress(demoContractAddress),
	// 	demoContractAddress,
	// 	0,
	// 	nil,
	// 	math.MaxUint64,
	// 	nil,
	// 	bytesData,
	// )
	// signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, demoContractPrivateKey)
	// if err != nil {
	// 	utils.GetLogInstance().Error("Failed to sign contract call tx", "error", err)
	// 	return nil
	// }
	// output, err := node.ContractCaller.CallContract(signedTx)

	// if err != nil {
	// 	utils.GetLogInstance().Error("Failed to call staking contract", "error", err)
	// 	return nil
	// }

	// ret := &structs.PlayersInfo{}
	// err = abi.Unpack(ret, "getPlayers", output)

	// if err != nil {
	// 	utils.GetLogInstance().Error("Failed to unpack stake info", "error", err)
	// 	return nil
	// }
	// return ret
}
