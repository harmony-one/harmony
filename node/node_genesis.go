package node

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/vm"
)

const (
	// FakeAddressNumber is the number of fake address.
	FakeAddressNumber = 100
	// TotalInitFund is the initial total fund to the faucet.
	TotalInitFund = 9000000
)

// GenesisBlockSetup setups a genesis blockchain.
func (node *Node) GenesisBlockSetup(db ethdb.Database) (*core.BlockChain, error) {
	// Initialize genesis block and blockchain
	genesisAlloc := node.CreateGenesisAllocWithTestingAddresses(FakeAddressNumber)
	contractKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Test contract key string stream that is fixed so that generated test key are deterministic every time"))
	contractAddress := crypto.PubkeyToAddress(contractKey.PublicKey)
	contractFunds := big.NewInt(TotalInitFund)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	genesisAlloc[contractAddress] = core.GenesisAccount{Balance: contractFunds}
	node.ContractKeys = append(node.ContractKeys, contractKey)

	chainConfig := params.TestChainConfig
	chainConfig.ChainID = big.NewInt(int64(node.Consensus.ShardID)) // Use ChainID as piggybacked ShardID
	gspec := core.Genesis{
		Config:  chainConfig,
		Alloc:   genesisAlloc,
		ShardID: uint32(node.Consensus.ShardID),
	}

	// Store genesis block into db.
	gspec.MustCommit(db)
	return core.NewBlockChain(db, nil, gspec.Config, node.Consensus, vm.Config{}, nil)
}

// CreateGenesisAllocWithTestingAddresses create the genesis block allocation that contains deterministically
// generated testing addressess with tokens.
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
		testBankFunds := big.NewInt(1000)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(params.Ether))
		genesisAloc[testBankAddress] = core.GenesisAccount{Balance: testBankFunds}
		node.TestBankKeys = append(node.TestBankKeys, testBankKey)
	}
	return genesisAloc
}
