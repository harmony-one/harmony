package node

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/utils/contract"
)

const (
	// FakeAddressNumber is the number of fake address.
	FakeAddressNumber = 100
	// TotalInitFund is the initial total fund for the contract deployer.
	TotalInitFund = 9000000
	// InitFreeFundInEther is the initial fund for sample accounts.
	InitFreeFundInEther = 1000
)

// GenesisBlockSetup setups a genesis blockchain.
func (node *Node) GenesisBlockSetup(db ethdb.Database, isArchival bool) (*core.BlockChain, error) {
	// Initialize genesis block and blockchain
	// Tests account for txgen to use
	genesisAlloc := node.CreateGenesisAllocWithTestingAddresses(FakeAddressNumber)

	// Smart contract deployer account used to deploy protocol-level smart contract
	contractDeployerKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Test contract key string stream that is fixed so that generated test key are deterministic every time"))
	contractDeployerAddress := crypto.PubkeyToAddress(contractDeployerKey.PublicKey)
	contractDeployerFunds := big.NewInt(TotalInitFund)
	contractDeployerFunds = contractDeployerFunds.Mul(contractDeployerFunds, big.NewInt(params.Ether))
	genesisAlloc[contractDeployerAddress] = core.GenesisAccount{Balance: contractDeployerFunds}
	node.ContractDeployerKey = contractDeployerKey

	// Accounts used by validator/nodes to stake and participate in the network.
	AddNodeAddressesToGenesisAlloc(genesisAlloc)

	chainConfig := params.TestChainConfig
	chainConfig.ChainID = big.NewInt(int64(node.Consensus.ShardID)) // Use ChainID as piggybacked ShardID
	gspec := core.Genesis{
		Config:         chainConfig,
		Alloc:          genesisAlloc,
		ShardID:        uint32(node.Consensus.ShardID),
		ShardStateHash: core.GetInitShardState().Hash(),
	}

	// Store genesis block into db.
	gspec.MustCommit(db)
	cacheConfig := core.CacheConfig{}
	if isArchival {
		cacheConfig = core.CacheConfig{Disabled: true, TrieNodeLimit: 256 * 1024 * 1024, TrieTimeLimit: 30 * time.Second}
	}
	return core.NewBlockChain(db, &cacheConfig, gspec.Config, node.Consensus, vm.Config{}, nil)
}

// CreateGenesisAllocWithTestingAddresses create the genesis block allocation that contains deterministically
// generated testing addressess with tokens. This is mostly used for generated simulated transactions in txgen.
// TODO: Remove it later when moving to production.
func (node *Node) CreateGenesisAllocWithTestingAddresses(numAddress int) core.GenesisAlloc {
	rand.Seed(0)
	len := 1000000
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(rand.Intn(100))
	}
	reader := strings.NewReader(string(bytes))
	genesisAloc := make(core.GenesisAlloc)
	for i := 0; i < numAddress; i++ {
		testBankKey, _ := ecdsa.GenerateKey(crypto.S256(), reader)
		testBankAddress := crypto.PubkeyToAddress(testBankKey.PublicKey)
		testBankFunds := big.NewInt(InitFreeFundInEther)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(params.Ether))
		genesisAloc[testBankAddress] = core.GenesisAccount{Balance: testBankFunds}
		node.TestBankKeys = append(node.TestBankKeys, testBankKey)
	}
	return genesisAloc
}

// AddNodeAddressesToGenesisAlloc adds to the genesis block allocation the accounts used for network validators/nodes,
// including the account used by the nodes of the initial beacon chain and later new nodes.
func AddNodeAddressesToGenesisAlloc(genesisAlloc core.GenesisAlloc) {
	for _, account := range contract.GenesisAccounts {
		testBankFunds := big.NewInt(InitFreeFundInEther)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(params.Ether))
		address := common.HexToAddress(account.Address)
		genesisAlloc[address] = core.GenesisAccount{Balance: testBankFunds}
	}
	for _, account := range contract.NewNodeAccounts {
		testBankFunds := big.NewInt(InitFreeFundInEther)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(params.Ether))
		address := common.HexToAddress(account.Address)
		genesisAlloc[address] = core.GenesisAccount{Balance: testBankFunds}
	}
	for _, account := range contract.DemoAccounts {
		testBankFunds := big.NewInt(InitFreeFundInEther)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(params.Ether))
		address := common.HexToAddress(account.Address)
		genesisAlloc[address] = core.GenesisAccount{Balance: testBankFunds}
	}
}
