package node

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/utils/contract"
)

const (
	// FakeAddressNumber is the number of fake address.
	FakeAddressNumber = 100
	// TotalInitFund is the initial total fund for the contract deployer.
	TotalInitFund = 1000000100
	// InitFreeFundInEther is the initial fund for sample accounts.
	InitFreeFundInEther = 100
)

// genesisInitializer is a shardchain.DBInitializer adapter.
type genesisInitializer struct {
	node *Node
}

// InitChainDB sets up a new genesis block in the database for the given shard.
func (gi *genesisInitializer) InitChainDB(db ethdb.Database, shardID uint32) error {
	return gi.node.SetupGenesisBlock(db, shardID)
}

// GenesisBlockSetup setups a genesis blockchain.
func (node *Node) SetupGenesisBlock(db ethdb.Database, shardID uint32) error {
	utils.GetLogger().Info("setting up a brand new chain database",
		"shardID", shardID)
	if shardID == node.Consensus.ShardID {
		node.isFirstTime = true
	}

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

	// Add puzzle fund
	puzzleFunds := big.NewInt(TotalInitFund)
	puzzleFunds = puzzleFunds.Mul(puzzleFunds, big.NewInt(params.Ether))
	genesisAlloc[common.HexToAddress(contract.PuzzleAccounts[0].Address)] = core.GenesisAccount{Balance: puzzleFunds}

	if shardID == 0 {
		// Accounts used by validator/nodes to stake and participate in the network.
		AddNodeAddressesToGenesisAlloc(genesisAlloc)
	}

	// TODO: create separate chain config instead of using the same pointer reference
	chainConfig := *params.TestChainConfig
	chainConfig.ChainID = big.NewInt(int64(shardID)) // Use ChainID as piggybacked ShardID
	gspec := core.Genesis{
		Config:  &chainConfig,
		Alloc:   genesisAlloc,
		ShardID: shardID,
	}

	// Store genesis block into db.
	_, err := gspec.Commit(db)

    return err
}

// CreateTestBankKeys deterministically generates testing addresses.
func CreateTestBankKeys(numAddresses int) (keys []*ecdsa.PrivateKey, err error) {
	rand.Seed(0)
    bytes := make([]byte, 1000000)
    for i := range bytes {
    	bytes[i] = byte(rand.Intn(100))
	}
    reader := strings.NewReader(string(bytes))
    for i := 0; i < numAddresses; i++ {
        key, err := ecdsa.GenerateKey(crypto.S256(), reader)
        if err != nil {
        	return nil, err
		}
        keys = append(keys, key)
	}
    return keys, nil
}

// CreateGenesisAllocWithTestingAddresses create the genesis block allocation that contains deterministically
// generated testing addresses with tokens. This is mostly used for generated simulated transactions in txgen.
// TODO: Remove it later when moving to production.
func (node *Node) CreateGenesisAllocWithTestingAddresses(numAddress int) core.GenesisAlloc {
	genesisAloc := make(core.GenesisAlloc)
	for _, testBankKey := range node.TestBankKeys {
		testBankAddress := crypto.PubkeyToAddress(testBankKey.PublicKey)
		testBankFunds := big.NewInt(InitFreeFundInEther)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(params.Ether))
		genesisAloc[testBankAddress] = core.GenesisAccount{Balance: testBankFunds}
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
	//for _, account := range contract.NewNodeAccounts {
	//	testBankFunds := big.NewInt(InitFreeFundInEther)
	//	testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(params.Ether))
	//	address := common.HexToAddress(account.Address)
	//	genesisAlloc[address] = core.GenesisAccount{Balance: testBankFunds}
	//}
	for _, account := range contract.DemoAccounts {
		testBankFunds := big.NewInt(InitFreeFundInEther)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(params.Ether))
		address := common.HexToAddress(account.Address)
		genesisAlloc[address] = core.GenesisAccount{Balance: testBankFunds}
	}
}
