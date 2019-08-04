package node

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	// GenesisFund is the initial total fund in the genesis block for mainnet.
	GenesisFund = 12600000000
	// TestAccountNumber is the number of test accounts for testnet/devnet/
	TestAccountNumber = 100
	// ContractDeployerInitFund is the initial fund for the contract deployer account in testnet/devnet.
	ContractDeployerInitFund = 100000000
	// InitFreeFund is the initial fund for permissioned accounts for testnet/devnet/
	InitFreeFund = 100
)

// genesisInitializer is a shardchain.DBInitializer adapter.
type genesisInitializer struct {
	node *Node
}

// InitChainDB sets up a new genesis block in the database for the given shard.
func (gi *genesisInitializer) InitChainDB(db ethdb.Database, shardID uint32) error {
	shardState := core.GetInitShardState()
	if shardID != 0 {
		// store only the local shard for shard chains
		c := shardState.FindCommitteeByID(shardID)
		if c == nil {
			return errors.New("cannot find local shard in genesis")
		}
		shardState = types.ShardState{*c}
	}
	gi.node.SetupGenesisBlock(db, shardID, shardState)
	return nil
}

// SetupGenesisBlock sets up a genesis blockchain.
func (node *Node) SetupGenesisBlock(db ethdb.Database, shardID uint32, myShardState types.ShardState) {
	utils.Logger().Info().Interface("shardID", shardID).Msg("setting up a brand new chain database")
	if shardID == node.NodeConfig.ShardID {
		node.isFirstTime = true
	}

	// Initialize genesis block and blockchain

	genesisAlloc := make(core.GenesisAlloc)
	chainConfig := params.ChainConfig{}

	switch node.NodeConfig.GetNetworkType() {
	case nodeconfig.Mainnet:
		chainConfig = *params.MainnetChainConfig
		if shardID == 0 {
			foundationAddress := common.HexToAddress("0xE25ABC3f7C3d5fB7FB81EAFd421FF1621A61107c")
			genesisFunds := big.NewInt(GenesisFund)
			genesisFunds = genesisFunds.Mul(genesisFunds, big.NewInt(denominations.One))
			genesisAlloc[foundationAddress] = core.GenesisAccount{Balance: genesisFunds}
		}
	case nodeconfig.Testnet:
		fallthrough
	case nodeconfig.Localnet:
		fallthrough
	case nodeconfig.Devnet:
		chainConfig = *params.TestnetChainConfig
		// Tests account for txgen to use
		node.AddTestingAddresses(genesisAlloc, TestAccountNumber)

		// Smart contract deployer account used to deploy initial smart contract
		contractDeployerKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Test contract key string stream that is fixed so that generated test key are deterministic every time"))
		contractDeployerAddress := crypto.PubkeyToAddress(contractDeployerKey.PublicKey)
		contractDeployerFunds := big.NewInt(ContractDeployerInitFund)
		contractDeployerFunds = contractDeployerFunds.Mul(contractDeployerFunds, big.NewInt(denominations.One))
		genesisAlloc[contractDeployerAddress] = core.GenesisAccount{Balance: contractDeployerFunds}
		node.ContractDeployerKey = contractDeployerKey
	}

	// Initialize shard state
	// TODO: add ShardID into chainconfig and change ChainID to NetworkID
	chainConfig.ChainID = big.NewInt(int64(shardID)) // Use ChainID as piggybacked ShardID

	gspec := core.Genesis{
		Config:         &chainConfig,
		Alloc:          genesisAlloc,
		ShardID:        shardID,
		GasLimit:       params.GenesisGasLimit,
		ShardStateHash: myShardState.Hash(),
		ShardState:     myShardState.DeepCopy(),
		Timestamp:      1561734000, // GMT: Friday, June 28, 2019 3:00:00 PM. PST: Friday, June 28, 2019 8:00:00 AM
		ExtraData:      []byte("Harmony for One and All. Open Consensus for 10B."),
	}

	// Store genesis block into db.
	gspec.MustCommit(db)
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

// AddTestingAddresses create the genesis block allocation that contains deterministically
// generated testing addresses with tokens. This is mostly used for generated simulated transactions in txgen.
func (node *Node) AddTestingAddresses(gAlloc core.GenesisAlloc, numAddress int) {
	for _, testBankKey := range node.TestBankKeys {
		testBankAddress := crypto.PubkeyToAddress(testBankKey.PublicKey)
		testBankFunds := big.NewInt(InitFreeFund)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(denominations.One))
		gAlloc[testBankAddress] = core.GenesisAccount{Balance: testBankFunds}
	}
}

// AddNodeAddressesToGenesisAlloc adds to the genesis block allocation the accounts used for network validators/nodes,
// including the account used by the nodes of the initial beacon chain and later new nodes.
func AddNodeAddressesToGenesisAlloc(genesisAlloc core.GenesisAlloc) {
	for _, account := range genesis.HarmonyAccounts {
		testBankFunds := big.NewInt(InitFreeFund)
		testBankFunds = testBankFunds.Mul(testBankFunds, big.NewInt(denominations.One))
		address := common2.ParseAddr(account.Address)
		genesisAlloc[address] = core.GenesisAccount{Balance: testBankFunds}
	}
}
