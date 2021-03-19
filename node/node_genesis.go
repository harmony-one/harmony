package node

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
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

	gspec := core.NewGenesisSpec(node.NodeConfig.GetNetworkType(), shardID)
	gspec.ShardStateHash = myShardState.Hash()
	gspec.ShardState = *myShardState.DeepCopy()
	// Store genesis block into db.
	gspec.MustCommit(db)
}
