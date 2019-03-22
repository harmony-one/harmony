package node

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/contracts"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	contract_constants "github.com/harmony-one/harmony/internal/utils/contract"
)

// Constants related to smart contract.
const (
	FaucetContractFund = 8000000
)

// AddStakingContractToPendingTransactions adds the deposit smart contract the genesis block.
func (node *Node) AddStakingContractToPendingTransactions() {
	// Add a contract deployment transaction
	//Generate contract key and associate funds with the smart contract
	priKey := contract_constants.GenesisBeaconAccountPriKey
	contractAddress := crypto.PubkeyToAddress(priKey.PublicKey)
	//Initially the smart contract should have minimal funds.
	contractFunds := big.NewInt(0)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
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
	//deployedcontractaddress = recepits[len(receipts)-1].ContractAddress //get the address from the receipt

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
	}
	bytesData, err := abi.Pack("listLockedAddresses")
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
	}

	priKey := contract_constants.GenesisBeaconAccountPriKey
	deployerAddress := crypto.PubkeyToAddress(priKey.PublicKey)

	state, err := node.blockchain.State()

	stakingContractAddress := crypto.CreateAddress(deployerAddress, uint64(0))
	tx := types.NewTransaction(
		state.GetNonce(deployerAddress),
		stakingContractAddress,
		0,
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
	state, err := node.blockchain.State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
	}
	return state.GetNonce(address)
}

// GetBalanceOfAddress returns balance of an address.
func (node *Node) GetBalanceOfAddress(address common.Address) uint64 {
	state, err := node.blockchain.State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
	}
	return state.GetBalance(address).Uint64()
}

// AddFaucetContractToPendingTransactions adds the faucet contract the genesis block.
func (node *Node) AddFaucetContractToPendingTransactions() {
	// Add a contract deployment transactionv
	priKey := node.ContractDeployerKey
	dataEnc := common.FromHex(contracts.FaucetBin)
	// Unsigned transaction to avoid the case of transaction address.

	contractFunds := big.NewInt(FaucetContractFund)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	mycontracttx, _ := types.SignTx(
		types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*10, nil, dataEnc),
		types.HomesteadSigner{},
		priKey)
	node.ContractAddresses = append(node.ContractAddresses, crypto.CreateAddress(crypto.PubkeyToAddress(priKey.PublicKey), uint64(0)))
	node.addPendingTransactions(types.Transactions{mycontracttx})
}

// CallFaucetContract invokes the faucet contract to give the walletAddress initial money
func (node *Node) CallFaucetContract(address common.Address) common.Hash {
	return node.callGetFreeToken(address)
}

func (node *Node) callGetFreeToken(address common.Address) common.Hash {
	nonce := node.GetNonceOfAddress(crypto.PubkeyToAddress(node.ContractDeployerKey.PublicKey))
	return node.callGetFreeTokenWithNonce(address, nonce)
}

func (node *Node) callGetFreeTokenWithNonce(address common.Address, nonce uint64) common.Hash {
	abi, err := abi.JSON(strings.NewReader(contracts.FaucetABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate faucet contract's ABI", "error", err)
	}
	bytesData, err := abi.Pack("request", address)
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
	}
	tx, _ := types.SignTx(types.NewTransaction(nonce, node.ContractAddresses[0], node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, bytesData), types.HomesteadSigner{}, node.ContractDeployerKey)
	utils.GetLogInstance().Info("Sending Free Token to ", "Address", address.Hex())

	node.addPendingTransactions(types.Transactions{tx})
	return tx.Hash()
}
