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
	candidates, e := slash.Records{}, utils.Logger().Error()

	if err := rlp.DecodeBytes(msgPayload, &candidates); err != nil {
		e.Err(err).
			Msg("unable to decode slash candidate message")
		return
	}

	if err := candidates.SanityCheck(); err != nil {
		e.Err(err).
			RawJSON("slash-candidates", []byte(candidates.String())).
			Msg("sanity check failed on incoming candidates")
		return
	}

	node.Blockchain().AddPendingSlashingCandidates(candidates)
}
