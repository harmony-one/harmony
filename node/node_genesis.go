package node

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	common2 "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
)

const (
	// GenesisONEToken is the initial total number of ONE in the genesis block for mainnet.
	GenesisONEToken = 12600000000
	// ContractDeployerInitFund is the initial fund for the contract deployer account in testnet/devnet.
	ContractDeployerInitFund = 10000000000
	// InitFreeFund is the initial fund for permissioned accounts for testnet/devnet/
	InitFreeFund = 100
)

var (
	// GenesisFund is the initial total number of ONE (in Nano) in the genesis block for mainnet.
	GenesisFund = new(big.Int).Mul(big.NewInt(GenesisONEToken), big.NewInt(denominations.One))
)

// genesisInitializer is a shardchain.DBInitializer adapter.
type genesisInitializer struct {
	node *Node
}

// InitChainDB sets up a new genesis block in the database for the given shard.
func (gi *genesisInitializer) InitChainDB(db ethdb.Database, shardID uint32) error {
	shardState, _ := committee.WithStakingEnabled.Compute(
		big.NewInt(core.GenesisEpoch), nil,
	)
	if shardState == nil {
		return errors.New("failed to create genesis shard state")
	}
	if shardID != shard.BeaconChainShardID {
		// store only the local shard for shard chains
		subComm, err := shardState.FindCommitteeByID(shardID)
		if err != nil {
			return errors.New("cannot find local shard in genesis")
		}
		shardState = &shard.State{nil, []shard.Committee{*subComm}}
	}
	gi.node.SetupGenesisBlock(db, shardID, shardState)
	return nil
}

// SetupGenesisBlock sets up a genesis blockchain.
func (node *Node) SetupGenesisBlock(db ethdb.Database, shardID uint32, myShardState *shard.State) {
	utils.Logger().Info().Interface("shardID", shardID).Msg("setting up a brand new chain database")
	if shardID == node.NodeConfig.ShardID {
		node.isFirstTime = true
	}

	// Initialize genesis block and blockchain

	genesisAlloc := make(core.GenesisAlloc)
	chainConfig := params.ChainConfig{}
	gasLimit := params.GenesisGasLimit

	netType := node.NodeConfig.GetNetworkType()

	switch netType {
	case nodeconfig.Mainnet:
		chainConfig = *params.MainnetChainConfig
		if shardID == 0 {
			foundationAddress := common.HexToAddress("0xE25ABC3f7C3d5fB7FB81EAFd421FF1621A61107c")
			genesisAlloc[foundationAddress] = core.GenesisAccount{Balance: GenesisFund}
		}
	case nodeconfig.Pangaea:
		chainConfig = *params.PangaeaChainConfig
	case nodeconfig.Partner:
		chainConfig = *params.PartnerChainConfig
	case nodeconfig.Stressnet:
		chainConfig = *params.StressnetChainConfig
	default: // all other types share testnet config
		chainConfig = *params.TestChainConfig
	}

	// All non-mainnet chains get test accounts
	if netType != nodeconfig.Mainnet {
		gasLimit = params.TestGenesisGasLimit
		// Smart contract deployer account used to deploy initial smart contract
		contractDeployerKey, _ := ecdsa.GenerateKey(
			crypto.S256(),
			strings.NewReader("Test contract key string stream that is fixed so that generated test key are deterministic every time"),
		)
		contractDeployerAddress := crypto.PubkeyToAddress(contractDeployerKey.PublicKey)
		contractDeployerFunds := big.NewInt(ContractDeployerInitFund)
		contractDeployerFunds = contractDeployerFunds.Mul(
			contractDeployerFunds, big.NewInt(denominations.One),
		)
		genesisAlloc[contractDeployerAddress] = core.GenesisAccount{Balance: contractDeployerFunds}
		node.ContractDeployerKey = contractDeployerKey
	}

	gspec := core.Genesis{
		Config:         &chainConfig,
		Factory:        blockfactory.NewFactory(&chainConfig),
		Alloc:          genesisAlloc,
		ShardID:        shardID,
		GasLimit:       gasLimit,
		ShardStateHash: myShardState.Hash(),
		ShardState:     *myShardState.DeepCopy(),
		Timestamp:      1561734000, // GMT: Friday, June 28, 2019 3:00:00 PM. PST: Friday, June 28, 2019 8:00:00 AM
		ExtraData:      []byte("Harmony for One and All. Open Consensus for 10B."),
	}

	// Store genesis block into db.
	gspec.MustCommit(db)
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
