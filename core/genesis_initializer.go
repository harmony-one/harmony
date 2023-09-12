package core

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/ethdb"
	nodeconfig "github.com/servprotocolorg/harmony/internal/configs/node"
	"github.com/servprotocolorg/harmony/internal/utils"
	"github.com/servprotocolorg/harmony/shard"
	"github.com/servprotocolorg/harmony/shard/committee"
)

// GenesisInitializer is a shardchain.DBInitializer adapter.
type GenesisInitializer struct {
	NetworkType nodeconfig.NetworkType
}

// InitChainDB sets up a new genesis block in the database for the given shard.
func (gi *GenesisInitializer) InitChainDB(db ethdb.Database, shardID uint32) error {
	shardState, _ := committee.WithStakingEnabled.Compute(
		big.NewInt(GenesisEpoch), nil,
	)
	if shardState == nil {
		return errors.New("failed to create genesis shard state")
	}
	switch shardID {
	case shard.BeaconChainShardID:
	default:
		// store only the local shard for shard chains
		subComm, err := shardState.FindCommitteeByID(shardID)
		if err != nil {
			return errors.New("cannot find local shard in genesis")
		}
		shardState = &shard.State{Shards: []shard.Committee{*subComm}}
	}
	gi.setupGenesisBlock(db, shardID, shardState)
	return nil
}

// SetupGenesisBlock sets up a genesis blockchain.
func (gi *GenesisInitializer) setupGenesisBlock(db ethdb.Database, shardID uint32, myShardState *shard.State) {
	utils.Logger().Info().Interface("shardID", shardID).Msg("setting up a brand new chain database")
	gspec := NewGenesisSpec(gi.NetworkType, shardID)
	gspec.ShardStateHash = myShardState.Hash()
	gspec.ShardState = *myShardState.DeepCopy()
	// Store genesis block into db.
	gspec.MustCommit(db)
}
