package availability

import (
	"github.com/ethereum/go-ethereum/common"
	engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/shard"
)

// Apply ..
func Apply(bc engine.ChainReader, state *state.DB) error {
	header := bc.CurrentHeader()
	if epoch := header.Epoch(); bc.Config().IsStaking(epoch) {
		if header.ShardID() == shard.BeaconChainShardID {
			superCommittee, err := bc.ReadShardState(header.Epoch())
			processed := make(map[common.Address]struct{})

			if err != nil {
				return err
			}

			for j := range superCommittee.Shards {
				shard := superCommittee.Shards[j]
				for j := range shard.Slots {
					slot := shard.Slots[j]
					if slot.EffectiveStake != nil { // For external validator
						_, ok := processed[slot.EcdsaAddress]
						if !ok {
							processed[slot.EcdsaAddress] = struct{}{}
						}
					}
				}
			}

			if err := IncrementValidatorSigningCounts(
				bc, header, header.ShardID(), state, processed,
			); err != nil {
				return err
			}

			// // kick out the inactive validators so they won't come up in the auction as possible
			// // candidates in the following call to SuperCommitteeForNextEpoch
			if shard.Schedule.IsLastBlock(header.Number().Uint64()) {
				if err := SetInactiveUnavailableValidators(
					bc, state, processed,
				); err != nil {
					return err
				}
			}
		} else {
			// TODO Handle shard chain
		}
	}

	return nil
}
