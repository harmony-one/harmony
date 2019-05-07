package node

import (
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/contracts"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	contract_constants "github.com/harmony-one/harmony/internal/utils/contract"
)

// Constants for puzzle.
const (
	Play    = "play"
	Payout  = "payout"
	EndGame = "endGame"
)

// OneEther is one ether
var OneEther = big.NewInt(params.Ether)

// AddPuzzleContract adds the demo puzzle contract the genesis block.
func (node *Node) AddPuzzleContract() {
	// Add a puzzle demo contract.
	priKey, err := crypto.HexToECDSA(contract_constants.PuzzleAccounts[0].Private)
	if err != nil {
		utils.GetLogInstance().Error("Error when creating private key for puzzle demo contract")
		// Exit here to recognize the coding working.
		// Basically we will remove this logic when launching so it's fine for now.
		os.Exit(1)
	}

	dataEnc := common.FromHex(contracts.PuzzleBin)
	// Unsigned transaction to avoid the case of transaction address.

	contractFunds := big.NewInt(PuzzleFund)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	demoContract, _ := types.SignTx(
		types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*1000, nil, dataEnc),
		types.HomesteadSigner{},
		priKey)
	node.PuzzleContractAddress = crypto.CreateAddress(crypto.PubkeyToAddress(priKey.PublicKey), uint64(0))
	node.PuzzleManagerPrivateKey = priKey
	node.addPendingTransactions(types.Transactions{demoContract})
}

// CreateTransactionForPlayMethod generates transaction for play method and add it into pending tx list.
func (node *Node) CreateTransactionForPlayMethod(priKey string, amount int64) (string, error) {
	var err error
	toAddress := node.PuzzleContractAddress
	abi, err := abi.JSON(strings.NewReader(contracts.PuzzleABI))
	if err != nil {
		utils.GetLogInstance().Error("puzzle-play: Failed to generate staking contract's ABI", "error", err)
		return "", err
	}
	bytesData, err := abi.Pack(Play)
	if err != nil {
		utils.GetLogInstance().Error("puzzle-play: Failed to generate ABI function bytes data", "error", err)
		return "", err
	}

	Stake := big.NewInt(0)
	Stake = Stake.Mul(OneEther, big.NewInt(amount))

	key, err := crypto.HexToECDSA(priKey)
	if err != nil {
		utils.GetLogInstance().Error("Failed to parse private key", "error", err)
		return "", err
	}
	address := crypto.PubkeyToAddress(key.PublicKey)
	balance, err := node.GetBalanceOfAddress(address)
	if err != nil {
		utils.GetLogInstance().Error("puzzle-play: can not get address", "error", err)
		return "", err
	} else if balance.Cmp(Stake) == -1 {
		utils.GetLogInstance().Error("puzzle-play: insufficient fund", "error", err, "stake", Stake, "balance", balance)
		return "", ErrPuzzleInsufficientFund
	}
	nonce := node.GetAndIncreaseAddressNonce(address)
	tx := types.NewTransaction(
		nonce,
		toAddress,
		node.NodeConfig.ShardID,
		Stake,
		params.TxGas*100,
		nil,
		bytesData,
	)

	if err != nil {
		utils.GetLogInstance().Error("puzzle-play: Failed to get private key", "error", err)
		return "", err
	}
	if signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key); err == nil {
		node.addPendingTransactions(types.Transactions{signedTx})
		return signedTx.Hash().Hex(), nil
	}
	utils.GetLogInstance().Error("puzzle-play: Unable to call enter method", "error", err)
	return "", err
}

// CreateTransactionForPayoutMethod generates transaction for payout method and add it into pending tx list.
func (node *Node) CreateTransactionForPayoutMethod(priKey string, level int, sequence string) (string, error) {
	var err error
	toAddress := node.PuzzleContractAddress

	abi, err := abi.JSON(strings.NewReader(contracts.PuzzleABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
		return "", err
	}

	key, err := crypto.HexToECDSA(priKey)
	if err != nil {
		utils.GetLogInstance().Error("Failed to parse private key", "error", err)
		return "", err
	}
	address := crypto.PubkeyToAddress(key.PublicKey)

	// add params for address payable player, uint8 new_level, steps string
	fmt.Println("Payout: address", address)
	bytesData, err := abi.Pack(Payout, address, big.NewInt(int64(level)), sequence)
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
		return "", err
	}

	if key == nil {
		return "", fmt.Errorf("user key is nil")
	}
	nonce := node.GetAndIncreaseAddressNonce(address)
	Amount := big.NewInt(0)
	tx := types.NewTransaction(
		nonce,
		toAddress,
		node.NodeConfig.ShardID,
		Amount,
		params.TxGas*10,
		nil,
		bytesData,
	)

	if err != nil {
		utils.GetLogInstance().Error("Failed to get private key", "error", err)
		return "", err
	}
	if signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key); err == nil {
		node.addPendingTransactions(types.Transactions{signedTx})
		return signedTx.Hash().Hex(), nil
	}
	utils.GetLogInstance().Error("Unable to call enter method", "error", err)
	return "", err
}

// CreateTransactionForEndMethod generates transaction for endGame method and add it into pending tx list.
func (node *Node) CreateTransactionForEndMethod(priKey string) (string, error) {
	var err error
	toAddress := node.PuzzleContractAddress

	abi, err := abi.JSON(strings.NewReader(contracts.PuzzleABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
		return "", err
	}
	key, err := crypto.HexToECDSA(priKey)
	if err != nil {
		utils.GetLogInstance().Error("Failed to parse private key", "error", err)
		return "", err
	}
	address := crypto.PubkeyToAddress(key.PublicKey)

	// add params for address payable player, uint8 new_level, steps string
	fmt.Println("EndGame: address", address)
	bytesData, err := abi.Pack(EndGame, address)
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
		return "", err
	}

	if key == nil {
		return "", fmt.Errorf("user key is nil")
	}
	nonce := node.GetAndIncreaseAddressNonce(address)
	Amount := big.NewInt(0)
	tx := types.NewTransaction(
		nonce,
		toAddress,
		node.NodeConfig.ShardID,
		Amount,
		params.TxGas*10,
		nil,
		bytesData,
	)

	if err != nil {
		utils.GetLogInstance().Error("Failed to get private key", "error", err)
		return "", err
	}
	if signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key); err == nil {
		node.addPendingTransactions(types.Transactions{signedTx})
		return signedTx.Hash().Hex(), nil
	}
	utils.GetLogInstance().Error("Unable to call enter method", "error", err)
	return "", err
}

// GetAndIncreaseAddressNonce get and increase the address's nonce
func (node *Node) GetAndIncreaseAddressNonce(address common.Address) uint64 {
	if value, ok := node.AddressNonce[address]; ok {
		nonce := atomic.AddUint64(value, 1)
		return nonce - 1
	}
	nonce := node.GetNonceOfAddress(address) + 1
	node.AddressNonce[address] = &nonce
	return nonce - 1
}
