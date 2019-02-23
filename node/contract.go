package node

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils/contract"
	"golang.org/x/crypto/sha3"
)

// Constants related to smart contract.
const (
	FaucetContractBinary      = "0x6080604052678ac7230489e8000060015560028054600160a060020a031916331790556101aa806100316000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a5780634ddd108a1461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a723058203e799228fee2fa7c5d15e71c04267a0cc2687c5eff3b48b98f21f355e1064ab30029"
	FaucetContractFund        = 8000000
	FaucetFreeMoneyMethodCall = "0x27c78c42000000000000000000000000"
	StakingContractBinary     = "0x608060405234801561001057600080fd5b506103f7806100206000396000f3fe608060405260043610610067576000357c01000000000000000000000000000000000000000000000000000000009004806317437a2c1461006c5780632e1a7d4d146100975780638da5cb5b146100e6578063b69ef8a81461013d578063d0e30db014610168575b600080fd5b34801561007857600080fd5b50610081610186565b6040518082815260200191505060405180910390f35b3480156100a357600080fd5b506100d0600480360360208110156100ba57600080fd5b81019080803590602001909291905050506101a5565b6040518082815260200191505060405180910390f35b3480156100f257600080fd5b506100fb6102cd565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561014957600080fd5b506101526102f3565b6040518082815260200191505060405180910390f35b610170610339565b6040518082815260200191505060405180910390f35b60003073ffffffffffffffffffffffffffffffffffffffff1631905090565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054821115156102c757816000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825403925050819055503373ffffffffffffffffffffffffffffffffffffffff166108fc839081150290604051600060405180830381858888f19350505050158015610280573d6000803e3d6000fd5b506000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506102c8565b5b919050565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905090565b6000346000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055506000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490509056fea165627a7a723058204acf95662eb95006df1e0b8ba32316211039c7872bc6eb99d12689c1624143d80029"
)

// AddStakingContractToPendingTransactions adds the deposit smart contract the genesis block.
func (node *Node) AddStakingContractToPendingTransactions() {
	// Add a contract deployment transaction
	//Generate contract key and associate funds with the smart contract
	priKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Deposit Smart Contract Key"))
	contractAddress := crypto.PubkeyToAddress(priKey.PublicKey)
	//Initially the smart contract should have minimal funds.
	contractFunds := big.NewInt(0)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	dataEnc := common.FromHex(StakingContractBinary)
	// Unsigned transaction to avoid the case of transaction address.
	mycontracttx, _ := types.SignTx(types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, priKey)
	//node.StakingContractAddress = crypto.CreateAddress(contractAddress, uint64(0))
	node.StakingContractAddress = node.generateDeployedStakingContractAddress(mycontracttx, contractAddress)
	node.addPendingTransactions(types.Transactions{mycontracttx})
}

// In order to get the deployed contract address of a contract, we need to find the nonce of the address that created it.
// (Refer: https://solidity.readthedocs.io/en/v0.5.3/introduction-to-smart-contracts.html#index-8)
// Then we can (re)create the deployed address. Trivially, this is 0 for us.
// The deployed contract address can also be obtained via the receipt of the contract creating transaction.
func (node *Node) generateDeployedStakingContractAddress(mycontracttx *types.Transaction, contractAddress common.Address) common.Address {
	//Ideally we send the transaction to

	//Correct Way 1:
	//node.SendTx(mycontracttx)
	//receipts := node.worker.GetCurrentReceipts()
	//deployedcontractaddress = recepits[len(receipts)-1].ContractAddress //get the address from the receipt

	//Correct Way 2:
	//nonce := GetNonce(contractAddress)
	//deployedAddress := crypto.CreateAddress(contractAddress, uint64(nonce))
	//deployedcontractaddress = recepits[len(receipts)-1].ContractAddress //get the address from the receipt
	nonce := 0
	return crypto.CreateAddress(contractAddress, uint64(nonce))
}

// CreateStakingWithdrawTransaction creates a new withdraw stake transaction
func (node *Node) CreateStakingWithdrawTransaction(stake string) (*types.Transaction, error) {
	//These should be read from somewhere.
	DepositContractPriKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Deposit Smart Contract Key")) //DepositContractPriKey is pk for contract
	DepositContractAddress := crypto.PubkeyToAddress(DepositContractPriKey.PublicKey)                             //DepositContractAddress is the address for the contract
	state, err := node.blockchain.State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
	}
	nonce := state.GetNonce(crypto.PubkeyToAddress(DepositContractPriKey.PublicKey))

	withdrawFnSignature := []byte("withdraw(uint256)")
	hash := sha3.NewLegacyKeccak256()
	hash.Write(withdrawFnSignature)
	methodID := hash.Sum(nil)[:4]

	withdraw := stake
	withdrawstake := new(big.Int)
	withdrawstake.SetString(withdraw, 10)
	paddedAmount := common.LeftPadBytes(withdrawstake.Bytes(), 32)

	var dataEnc []byte
	dataEnc = append(dataEnc, methodID...)
	dataEnc = append(dataEnc, paddedAmount...)

	tx, err := types.SignTx(types.NewTransaction(nonce, DepositContractAddress, node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, node.AccountKey)
	return tx, err
}

func (node *Node) getDeployedStakingContract() common.Address {
	return node.StakingContractAddress
}

// AddFaucetContractToPendingTransactions adds the faucet contract the genesis block.
func (node *Node) AddFaucetContractToPendingTransactions() {
	// Add a contract deployment transactionv
	priKey := node.ContractKeys[0]
	dataEnc := common.FromHex(FaucetContractBinary)
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
func (node *Node) CallFaucetContract(walletAddress common.Address) common.Hash {
	return node.createSendingMoneyTransaction(walletAddress)
}

func (node *Node) createSendingMoneyTransaction(walletAddress common.Address) common.Hash {
	state, err := node.blockchain.State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
	}
	nonce := state.GetNonce(crypto.PubkeyToAddress(node.ContractKeys[0].PublicKey))
	contractData := FaucetFreeMoneyMethodCall + hex.EncodeToString(walletAddress.Bytes())
	dataEnc := common.FromHex(contractData)
	tx, _ := types.SignTx(types.NewTransaction(nonce, node.ContractAddresses[0], node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, node.ContractKeys[0])

	node.addPendingTransactions(types.Transactions{tx})
	return tx.Hash()
}

// DepositToFakeAccounts invokes the faucet contract to give the walletAddress initial money
func (node *Node) DepositToFakeAccounts() {
	for _, deployAccount := range contract.FakeAccounts {
		address := common.HexToAddress(deployAccount.Address)
		node.createSendingMoneyTransaction(address)
	}
}
