package node

// CreateTransactionForEnterMethod creates transaction to call enter method of lottery contract.
import (
	"fmt"
	"math"
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
	PickWinner = "pickWinner"
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
	node.LotteryManagerPrivateKey = priKey
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

// GetResultDirectly get current players and their balances, not from smart contract.
func (node *Node) GetResultDirectly(priKey string) (players []string, balances []*big.Int) {
	for _, account := range contract_constants.DemoAccounts {
		players = append(players, account.Private)
		key, err := crypto.HexToECDSA(account.Private)
		if err != nil {
			utils.GetLogInstance().Error("Error when HexToECDSA")
		}
		address := crypto.PubkeyToAddress(key.PublicKey)
		balance, err := node.GetBalanceOfAddress(address)
		balances = append(balances, balance)
	}
	return players, balances
}

// GenerateResultDirectly get current players and their balances, not from smart contract.
func (node *Node) GenerateResultDirectly(addresses []common.Address) (players []string, balances []*big.Int) {
	for _, address := range addresses {
		players = append(players, address.String())
		balance, _ := node.GetBalanceOfAddress(address)
		balances = append(balances, balance)
	}
	fmt.Println("generate result", players, balances)
	return players, balances
}

// GetResult get current players and their balances.
func (node *Node) GetResult(priKey string) (players []string, balances []*big.Int) {
	// TODO(minhdoan): get result from smart contract is current not working. Fix it later.
	abi, err := abi.JSON(strings.NewReader(contracts.LotteryABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
	}
	bytesData, err := abi.Pack("getPlayers")
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
	}

	demoContractAddress := node.DemoContractAddress
	key, err := crypto.HexToECDSA(priKey)
	if err != nil {
		utils.GetLogInstance().Error("Failed to parse private key", "error", err)
	}

	nonce := node.GetNonceOfAddress(crypto.PubkeyToAddress(key.PublicKey))

	tx := types.NewTransaction(
		nonce,
		demoContractAddress,
		0,
		nil,
		math.MaxUint64,
		nil,
		bytesData,
	)
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign contract call tx", "error", err)
		return nil, nil
	}
	output, err := node.ContractCaller.CallContract(signedTx)
	if err != nil {
		utils.GetLogInstance().Error("Failed to call staking contract", "error", err)
		return nil, nil
	}

	ret := []common.Address{}
	err = abi.Unpack(&ret, "getPlayers", output)

	if err != nil {
		utils.GetLogInstance().Error("Failed to unpack getPlayers", "error", err)
		return nil, nil
	}
	utils.GetLogInstance().Info("get result: ", "ret", ret)
	fmt.Println("get result called:", ret)
	return node.GenerateResultDirectly(ret)
}

// CreateTransactionForPickWinner generates transaction for enter method and add it into pending tx list.
func (node *Node) CreateTransactionForPickWinner() error {
	var err error
	toAddress := node.DemoContractAddress

	abi, err := abi.JSON(strings.NewReader(contracts.LotteryABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
		return err
	}
	bytesData, err := abi.Pack(PickWinner)
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
		return err
	}

	key := node.LotteryManagerPrivateKey
	nonce := node.GetNonceOfAddress(crypto.PubkeyToAddress(key.PublicKey))
	Amount := big.NewInt(0)
	tx := types.NewTransaction(
		nonce,
		toAddress,
		0,
		Amount,
		params.TxGas*1000,
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
