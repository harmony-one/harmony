package node

import (
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/harmony-one/harmony/webhooks"
)

// ProcessSlashCandidateMessage ..
func (node *Node) processSlashCandidateMessage(msgPayload []byte) {
	if node.NodeConfig.ShardID != shard.BeaconChainShardID {
		return
	}
	candidates := slash.Records{}

	if err := rlp.DecodeBytes(msgPayload, &candidates); err != nil {
		utils.Logger().Error().
			Err(err).Msg("unable to decode slash candidates message")
		return
	}

	if err := node.Blockchain().AddPendingSlashingCandidates(
		candidates,
	); err != nil {
		utils.Logger().Error().
			Err(err).Msg("unable to add slash candidates to pending ")
	}
}

func (node *Node) handleSlashChan() {
	for doubleSign := range node.Consensus.SlashChan {
		utils.Logger().Info().
			RawJSON("double-sign-candidate", []byte(doubleSign.String())).
			Msg("double sign notified by consensus leader")
		// no point to broadcast the slash if we aren't even in the right epoch yet
		if !node.Blockchain().Config().IsStaking(
			node.Blockchain().CurrentHeader().Epoch(),
		) {
			return
		}
		if hooks := node.NodeConfig.WebHooks.Hooks; hooks != nil {
			if s := hooks.Slashing; s != nil {
				url := s.OnNoticeDoubleSign
				go func() { webhooks.DoPost(url, &doubleSign) }()
			}
		}
		if node.NodeConfig.ShardID != shard.BeaconChainShardID {
			go node.BroadcastSlash(&doubleSign)
		} else {
			records := slash.Records{doubleSign}
			if err := node.Blockchain().AddPendingSlashingCandidates(
				records,
			); err != nil {
				utils.Logger().Err(err).Msg("could not add new slash to ending slashes")
			}
		}
	}

}
