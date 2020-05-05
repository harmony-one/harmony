package relay

import (
	"errors"

	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
)

// BroadCaster ..
type BroadCaster struct {
	config *nodeconfig.ConfigType
	host   *p2p.Host
}

// NewBroadCaster ..
func NewBroadCaster(
	configUsed *nodeconfig.ConfigType,
	host *p2p.Host,
) *BroadCaster {
	return &BroadCaster{
		config: configUsed,
		host:   host,
	}
}

func (c *BroadCaster) newBlock(
	newBlock *types.Block, groups []nodeconfig.GroupID,
) error {

	blockData, err := rlp.EncodeToBytes(newBlock)
	if err != nil {
		return err
	}

	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_BROADCASTED_NEW_BLOCK,
		Request: &msg_pb.Message_NewBlock{
			NewBlock: &msg_pb.LeaderBroadCastedBlockRequest{
				Block: blockData,
			},
		},
	}

	marshaledMessage, err := protobuf.Marshal(message)

	if err != nil {
		return err
	}

	return c.host.SendMessageToGroups(
		groups, p2p.ConstructMessage(
			proto.ConstructConsensusMessage(marshaledMessage),
		),
	)
}

var (
	errBlockToBroadCastWrong = errors.New("wrong shard id")
	beaconChainClientGroup   = []nodeconfig.GroupID{
		nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID),
	}
)

// AcceptedBlockForShardGroup ..
func (c *BroadCaster) AcceptedBlockForShardGroup(
	shardID uint32, blk *types.Block,
) error {
	grps := []nodeconfig.GroupID{c.config.GetShardGroupID()}
	return c.newBlock(blk, grps)
}

// NewBeaconChainBlockForClient ..
func (c *BroadCaster) NewBeaconChainBlockForClient(
	newBlock *types.Block,
) error {
	// HACK need to think through the groups/topics later, its not a client
	if newBlock.Header().ShardID() != shard.BeaconChainShardID {
		return errBlockToBroadCastWrong
	}

	return c.newBlock(newBlock, beaconChainClientGroup)
}

// BroadcastSlash ..
func (c *BroadCaster) NewSlashRecord(witness *slash.Record) error {
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
