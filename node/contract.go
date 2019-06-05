package node

import (
	"crypto/ecdsa"
	"math/big"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/contracts"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	contract_constants "github.com/harmony-one/harmony/internal/utils/contract"
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
	scStaking
)

// AddStakingContractToPendingTransactions adds the deposit smart contract the genesis block.
func (node *Node) AddStakingContractToPendingTransactions() {
	// Add a contract deployment transaction
	//Generate contract key and associate funds with the smart contract
	priKey := contract_constants.GenesisBeaconAccountPriKey
	contractAddress := crypto.PubkeyToAddress(priKey.PublicKey)
	//Initially the smart contract should have minimal funds.
	contractFunds := big.NewInt(0)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(denominations.One))
	dataEnc := common.FromHex(contracts.StakeLockContractBin)
	// Unsigned transaction to avoid the case of transaction address.
	mycontracttx, _ := types.SignTx(types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*100, nil, dataEnc), types.HomesteadSigner{}, priKey)
	//node.StakingContractAddress = crypto.CreateAddress(contractAddress, uint64(0))
	node.StakingContractAddress = node.generateDeployedStakingContractAddress(contractAddress)
	node.addPendingTransactions(types.Transactions{mycontracttx})
}

// In order to get the deployed contract address of a contract, we need to find the nonce of the address that created it.
// (Refer: https://solidity.readthedocs.io/en/v0.5.3/introduction-to-smart-contracts.html#index-8)
// Then we can (re)create the deployed address. Trivially, this is 0 for us.
// The deployed contract address can also be obtained via the receipt of the contract creating transaction.
func (node *Node) generateDeployedStakingContractAddress(contractAddress common.Address) common.Address {
	//Correct Way 1:
	//node.SendTx(mycontracttx)
	//receipts := node.worker.GetCurrentReceipts()
	//deployedcontractaddress = receipts[len(receipts)-1].ContractAddress //get the address from the receipt

	//Correct Way 2:
	//nonce := GetNonce(contractAddress)
	//deployedAddress := crypto.CreateAddress(contractAddress, uint64(nonce))
	nonce := 0
	return crypto.CreateAddress(contractAddress, uint64(nonce))
}

// QueryStakeInfo queries the stake info from the stake contract.
func (node *Node) QueryStakeInfo() *structs.StakeInfoReturnValue {
	abi, err := abi.JSON(strings.NewReader(contracts.StakeLockContractABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
		return nil
	}
	bytesData, err := abi.Pack("listLockedAddresses")
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
		return nil
	}

	priKey := contract_constants.GenesisBeaconAccountPriKey
	deployerAddress := crypto.PubkeyToAddress(priKey.PublicKey)

	state, err := node.Blockchain().State()

	if err != nil {
		utils.GetLogInstance().Error("Failed to get blockchain state", "error", err)
		return nil
	}

	stakingContractAddress := crypto.CreateAddress(deployerAddress, uint64(0))
	tx := types.NewTransaction(
		state.GetNonce(deployerAddress),
		stakingContractAddress,
		node.NodeConfig.ShardID,
		nil,
		math.MaxUint64,
		nil,
		bytesData,
	)
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, priKey)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign contract call tx", "error", err)
		return nil
	}
	output, err := node.ContractCaller.CallContract(signedTx)

	if err != nil {
		utils.GetLogInstance().Error("Failed to call staking contract", "error", err)
		return nil
	}

	ret := &structs.StakeInfoReturnValue{}

	err = abi.Unpack(ret, "listLockedAddresses", output)

	if err != nil {
		utils.GetLogInstance().Error("Failed to unpack stake info", "error", err)
		return nil
	}
	return ret
}

func (node *Node) getDeployedStakingContract() common.Address {
	return node.StakingContractAddress
}

// GetNonceOfAddress returns nonce of an address.
func (node *Node) GetNonceOfAddress(address common.Address) uint64 {
	state, err := node.Blockchain().State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
		return 0
	}
	return state.GetNonce(address)
}

// GetBalanceOfAddress returns balance of an address.
func (node *Node) GetBalanceOfAddress(address common.Address) (*big.Int, error) {
	state, err := node.Blockchain().State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
		return nil, err
	}
	return state.GetBalance(address), nil
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
	// Temporary code to workaround explorer issue for searching new addresses (https://github.com/harmony-one/harmony/issues/503)
	nonce := atomic.AddUint64(&node.ContractDeployerCurrentNonce, 1)
	tx, _ := types.SignTx(types.NewTransaction(nonce-1, address, node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, nil), types.HomesteadSigner{}, node.ContractDeployerKey)
	utils.GetLogInstance().Info("Sending placeholder token to ", "Address", address.Hex())
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
		utils.GetLogInstance().Error("Failed to generate faucet contract's ABI", "error", err)
		return common.Hash{}
	}
	bytesData, err := abi.Pack("request", address)
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
		return common.Hash{}
	}
	if len(node.ContractAddresses) == 0 {
		utils.GetLogInstance().Error("Failed to find the contract address")
		return common.Hash{}
	}
	tx, _ := types.SignTx(types.NewTransaction(nonce, node.ContractAddresses[0], node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, bytesData), types.HomesteadSigner{}, node.ContractDeployerKey)
	utils.GetLogInstance().Info("Sending Free Token to ", "Address", address.Hex())

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
	case scStaking:
		// staking contract
		node.CurrentStakes = make(map[common.Address]*structs.StakeInfo)
		stakingPrivKey := contract_constants.GenesisBeaconAccountPriKey
		node.StakingContractAddress = crypto.CreateAddress(crypto.PubkeyToAddress(stakingPrivKey.PublicKey), uint64(0))
	default:
		utils.GetLogInstance().Error("AddContractKeyAndAddress", "unknown SC", t)
	}
}
