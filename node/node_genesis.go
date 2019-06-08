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
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/genesis"
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
	shardState := core.GetInitShardState()
	if shardID != 0 {
		// store only the local shard
		c := shardState.FindCommitteeByID(shardID)
		if c == nil {
			return errors.New("cannot find local shard in genesis")
		}
		shardState = types.ShardState{*c}
	}
	if err := rawdb.WriteShardState(db, common.Big0, shardState); err != nil {
		return ctxerror.New("cannot store epoch shard state").WithCause(err)
	}
	if err := gi.node.SetupGenesisBlock(db, shardID); err != nil {
		return ctxerror.New("cannot setup genesis block").WithCause(err)
	}
	return nil
}

// SetupGenesisBlock sets up a genesis blockchain.
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
	contractDeployerFunds = contractDeployerFunds.Mul(contractDeployerFunds, big.NewInt(denominations.One))
	genesisAlloc[contractDeployerAddress] = core.GenesisAccount{Balance: contractDeployerFunds}
	node.ContractDeployerKey = contractDeployerKey

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
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(denominations.One))
		genesisAloc[testBankAddress] = core.GenesisAccount{Balance: testBankFunds}
	}
	return genesisAloc
}

// AddNodeAddressesToGenesisAlloc adds to the genesis block allocation the accounts used for network validators/nodes,
// including the account used by the nodes of the initial beacon chain and later new nodes.
func AddNodeAddressesToGenesisAlloc(genesisAlloc core.GenesisAlloc) {
	for _, account := range genesis.GenesisAccounts {
		testBankFunds := big.NewInt(InitFreeFundInEther)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(denominations.One))
		address := common.HexToAddress(account.Address)
		genesisAlloc[address] = core.GenesisAccount{Balance: testBankFunds}
	}
}
