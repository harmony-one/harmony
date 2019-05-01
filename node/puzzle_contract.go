package node

import (
	"fmt"
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

// Constants for puzzle.
const (
	Play   = "play"
	Payout = "payout"
)

// GameStake represents one ether.
var GameStake = 2 * big.NewInt(params.Ether)

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
		types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*10, nil, dataEnc),
		types.HomesteadSigner{},
		priKey)
	node.PuzzleContractAddress = crypto.CreateAddress(crypto.PubkeyToAddress(priKey.PublicKey), uint64(0))
	node.PuzzleManagerPrivateKey = priKey
	node.addPendingTransactions(types.Transactions{demoContract})
}

// CreateTransactionForPlayMethod generates transaction for enter method and add it into pending tx list.
func (node *Node) CreateTransactionForPlayMethod(priKey string) error {
	var err error
	toAddress := node.PuzzleContractAddress

	abi, err := abi.JSON(strings.NewReader(contracts.PuzzleABI))
	if err != nil {
		utils.GetLogInstance().Error("puzzle-play: Failed to generate staking contract's ABI", "error", err)
		return err
	}
	bytesData, err := abi.Pack(Play)
	if err != nil {
		utils.GetLogInstance().Error("puzzle-play: Failed to generate ABI function bytes data", "error", err)
		return err
	}

	key, err := crypto.HexToECDSA(priKey)
	address := crypto.PubkeyToAddress(key.PublicKey)
	balance, err := node.GetBalanceOfAddress(address)
	if err != nil {
		utils.GetLogInstance().Error("puzzle-play: can not get address", "error", err)
		return err
	} else if balance.Cmp(GameStake) == -1 {
		utils.GetLogInstance().Error("puzzle-play: insufficient fund", "error", err)
		return ErrPuzzleInsufficientFund
	}
	nonce := node.GetNonceOfAddress(address)
	tx := types.NewTransaction(
		nonce,
		toAddress,
		0,
		GameStake,
		params.TxGas*10,
		nil,
		bytesData,
	)

	if err != nil {
		utils.GetLogInstance().Error("puzzle-play: Failed to get private key", "error", err)
		return err
	}
	if signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key); err == nil {
		node.addPendingTransactions(types.Transactions{signedTx})
		return nil
	}
	utils.GetLogInstance().Error("puzzle-play: Unable to call enter method", "error", err)
	return err
}

// CreateTransactionForPayoutMethod generates transaction for payout method and add it into pending tx list.
func (node *Node) CreateTransactionForPayoutMethod(address string, newLevel int, session int, sequence string) error {
	var err error
	toAddress := node.PuzzleContractAddress

	abi, err := abi.JSON(strings.NewReader(contracts.PuzzleABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
		return err
	}
	bytesData, err := abi.Pack("payout", address, newLevel, session, sequence)
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
		return err
	}

	key := node.PuzzleManagerPrivateKey
	if key == nil {
		return fmt.Errorf("PuzzleManagerPrivateKey is nil")
	}
	nonce := node.GetNonceOfAddress(crypto.PubkeyToAddress(key.PublicKey))
	Amount := big.NewInt(0)
	tx := types.NewTransaction(
		nonce,
		toAddress,
		0,
		Amount,
		params.TxGas*10000,
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
