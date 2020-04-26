package relay

import (
	"errors"
	"fmt"

	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
)

// TxnCaster ..
type TxnCaster interface {
	NewStakingTransaction(stakingTx *staking.StakingTransaction) error
	NewTransaction(tx *types.Transaction) error
}

// BlockCaster ..
type BlockCaster interface {
	NewShardChainBlock(newBlock *types.Block) error
	NewBeaconChainBlock(newBlock *types.Block) error
}

// BroadCaster ..
type BroadCaster interface {
	TxnCaster
	BlockCaster
	NewSlashRecord(witness *slash.Record) error
}

type caster struct {
	config *nodeconfig.ConfigType
	host   p2p.Host
}

// NewBroadCaster ..
func NewBroadCaster(
	configUsed *nodeconfig.ConfigType,
	host p2p.Host,
) BroadCaster {
	return &caster{
		config: configUsed,
		host:   host,
	}
}

const (
	// NumTryBroadCast is the number of times trying to broadcast
	NumTryBroadCast = 3
)

// TODO: make this batch more transactions
func (c *caster) tryBroadcast(tx *types.Transaction) {
	msg := proto_node.ConstructTransactionListMessageAccount(types.Transactions{tx})

	shardGroupID := nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(tx.ShardID()))
	utils.Logger().Info().Str("shardGroupID", string(shardGroupID)).Msg("tryBroadcast")

	for attempt := 0; attempt < NumTryBroadCast; attempt++ {
		if err := c.host.SendMessageToGroups([]nodeconfig.GroupID{shardGroupID},
			p2p.ConstructMessage(msg)); err != nil && attempt < NumTryBroadCast {
			utils.Logger().Error().Int("attempt", attempt).Msg("Error when trying to broadcast tx")
		} else {
			break
		}
	}
}

func (c *caster) tryBroadcastStaking(stakingTx *staking.StakingTransaction) {
	msg := proto_node.ConstructStakingTransactionListMessageAccount(
		staking.StakingTransactions{stakingTx},
	)

	shardGroupID := nodeconfig.NewGroupIDByShardID(
		nodeconfig.ShardID(shard.BeaconChainShardID),
	) // broadcast to beacon chain
	utils.Logger().Info().
		Str("shardGroupID", string(shardGroupID)).
		Msg("tryBroadcastStaking")

	for attempt := 0; attempt < NumTryBroadCast; attempt++ {
		if err := c.host.SendMessageToGroups([]nodeconfig.GroupID{shardGroupID},
			p2p.ConstructMessage(msg)); err != nil && attempt < NumTryBroadCast {
			utils.Logger().Error().
				Int("attempt", attempt).
				Msg("Error when trying to broadcast staking tx")
		} else {
			break
		}
	}
}

func (c *caster) newBlock(
	newBlock *types.Block, groups []nodeconfig.GroupID,
) error {
	msg := p2p.ConstructMessage(
		proto_node.ConstructBlocksSyncMessage([]*types.Block{newBlock}),
	)
	if err := c.host.SendMessageToGroups(groups, msg); err != nil {
		return err
	}
	return nil
}

var (
	errBlockToBroadCastWrong = errors.New("wrong shard id")
)

func (c *caster) NewBeaconChainBlock(newBlock *types.Block) error {
	// HACK need to think through the groups/topics later, its not a client
	if newBlock.Header().ShardID() != shard.BeaconChainShardID {
		return errBlockToBroadCastWrong
	}

	groups := []nodeconfig.GroupID{
		nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID),
		// c.config.GetClientGroupID(),
	}

	fmt.Println("beaconchain broadcast", groups, newBlock.String())

	return c.newBlock(newBlock, groups)
}

func (c *caster) NewShardChainBlock(newBlock *types.Block) error {
	shardID := newBlock.Header().ShardID()
	if shardID == shard.BeaconChainShardID ||
		c.config.ShardID == shard.BeaconChainShardID {
		return errBlockToBroadCastWrong
	}

	groups := []nodeconfig.GroupID{
		nodeconfig.NewClientGroupIDByShardID(c.config.ShardID),
	}

	fmt.Println("shardChain broadcast", groups, newBlock.String())
	return c.newBlock(newBlock, groups)
}

// BroadcastSlash ..
func (c *caster) NewSlashRecord(witness *slash.Record) error {
	if err := c.host.SendMessageToGroups(
		[]nodeconfig.GroupID{c.config.GetBeaconGroupID()},
		p2p.ConstructMessage(
			proto_node.ConstructSlashMessage(slash.Records{*witness})),
	); err != nil {
		utils.Logger().Err(err).
			RawJSON("record", []byte(witness.String())).
			Msg("could not send slash record to beaconchain")
		return err
	}
	utils.Logger().Info().Msg("broadcast the double sign record")
	return nil
}

func (c *caster) NewStakingTransaction(
	stakingTx *staking.StakingTransaction,
) error {
	// TODO make this give back err
	c.tryBroadcastStaking(stakingTx)
	return nil
}

func (c *caster) NewTransaction(
	tx *types.Transaction,
) error {
	c.tryBroadcast(tx)
	return nil
}
