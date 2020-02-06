package node

import (
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
)

// ProcessSlashCandidateMessage ..
func (node *Node) processSlashCandidateMessage(msgPayload []byte) {
	if node.NodeConfig.ShardID != shard.BeaconChainShardID {
		return
	}

	candidate := slash.Record{}
	if err := rlp.DecodeBytes(msgPayload, &candidate); err != nil {
		utils.Logger().Error().
			Err(err).
			Msg("unable to decode slash candidate message")
		return
	}
	node.Blockchain().AddPendingSlashingCandidate(&candidate)
}
