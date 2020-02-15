package node

import (
	"fmt"

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
	candidates := []slash.Record{}
	if err := rlp.DecodeBytes(msgPayload, candidates); err != nil {
		fmt.Println("couldn't decode the payload correctly.", err.Error())
		utils.Logger().Error().
			Err(err).
			Msg("unable to decode slash candidate message")
		return
	}

	node.Blockchain().AddPendingSlashingCandidates(candidates)
}
