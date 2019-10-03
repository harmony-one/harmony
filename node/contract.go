package node

import (
	"crypto/ecdsa"
	"math/big"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/params"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/contracts"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants related to smart contract.
const (
	FaucetContractFund = 80000000
)

// BuiltInSC is the type of built-in smart contract in blockchain
type builtInSC uint

// List of smart contract type built-in
const (
	scFaucet builtInSC = iota
)

// GetNonceOfAddress returns nonce of an address.
func (node *Node) GetNonceOfAddress(address common.Address) uint64 {
	state, err := node.Blockchain().State()
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to get chain state")
		return 0
	}
	return state.GetNonce(address)
}

// GetBalanceOfAddress returns balance of an address.
func (node *Node) GetBalanceOfAddress(address common.Address) (*big.Int, error) {
	state, err := node.Blockchain().State()
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to get chain state")
		return nil, err
	}
	balance := big.NewInt(0)
	balance.SetBytes(state.GetBalance(address).Bytes())
	return balance, nil
}

// AddFaucetContractToPendingTransactions adds the faucet contract the genesis block.
func (node *Node) AddFaucetContractToPendingTransactions() {
	// Add a contract deployment transactionv
	priKey := node.ContractDeployerKey
	dataEnc := common.FromHex(contracts.FaucetBin)
	// Unsigned transaction to avoid the case of transaction address.

	contractFunds := big.NewInt(FaucetContractFund)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(denominations.One))
	mycontracttx, _ := types.SignTx(
		types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*10, nil, dataEnc),
		types.HomesteadSigner{},
		priKey)
	node.ContractAddresses = append(node.ContractAddresses, crypto.CreateAddress(crypto.PubkeyToAddress(priKey.PublicKey), uint64(0)))
	node.addPendingTransactions(types.Transactions{mycontracttx})
}

// CallFaucetContract invokes the faucet contract to give the walletAddress initial money
func (node *Node) CallFaucetContract(address common.Address) common.Hash {
	if node.NodeConfig.GetNetworkType() == nodeconfig.Mainnet {
		return common.Hash{}
	}
	// Temporary code to workaround explorer issue for searching new addresses (https://github.com/harmony-one/harmony/issues/503)
	nonce := atomic.AddUint64(&node.ContractDeployerCurrentNonce, 1)
	tx, _ := types.SignTx(types.NewTransaction(nonce-1, address, node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, nil), types.HomesteadSigner{}, node.ContractDeployerKey)
	utils.Logger().Info().Str("Address", common2.MustAddressToBech32(address)).Msg("Sending placeholder token to ")
	node.addPendingTransactions(types.Transactions{tx})
	// END Temporary code

	nonce = atomic.AddUint64(&node.ContractDeployerCurrentNonce, 1)
	return node.callGetFreeTokenWithNonce(address, nonce-1)
}

func (node *Node) callGetFreeToken(address common.Address) common.Hash {
	nonce := atomic.AddUint64(&node.ContractDeployerCurrentNonce, 1)
	return node.callGetFreeTokenWithNonce(address, nonce-1)
}

func (node *Node) callGetFreeTokenWithNonce(address common.Address, nonce uint64) common.Hash {
	abi, err := abi.JSON(strings.NewReader(contracts.FaucetABI))
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to generate faucet contract's ABI")
		return common.Hash{}
	}
	bytesData, err := abi.Pack("request", address)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to generate ABI function bytes data")
		return common.Hash{}
	}
	if len(node.ContractAddresses) == 0 {
		utils.Logger().Error().Err(err).Msg("Failed to find the contract address")
		return common.Hash{}
	}
	tx, _ := types.SignTx(types.NewTransaction(nonce, node.ContractAddresses[0], node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, bytesData), types.HomesteadSigner{}, node.ContractDeployerKey)
	utils.Logger().Info().Str("Address", common2.MustAddressToBech32(address)).Msg("Sending Free Token to ")

	node.addPendingTransactions(types.Transactions{tx})
	return tx.Hash()
}

// AddContractKeyAndAddress is used to add smart contract related information when node restart and resume with previous state
// It supports three kinds of on-chain smart contracts for now.
func (node *Node) AddContractKeyAndAddress(t builtInSC) {
	switch t {
	case scFaucet:
		// faucet contract
		contractDeployerKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Test contract key string stream that is fixed so that generated test key are deterministic every time"))
		node.ContractDeployerKey = contractDeployerKey
		node.ContractAddresses = append(node.ContractAddresses, crypto.CreateAddress(crypto.PubkeyToAddress(contractDeployerKey.PublicKey), uint64(0)))
	default:
		utils.Logger().Error().Interface("unknown SC", t).Msg("AddContractKeyAndAddress")
	}
}
