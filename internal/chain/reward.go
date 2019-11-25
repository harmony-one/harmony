package chain

import (
	"math/big"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/pkg/errors"
)

var (
	// BlockReward is the block reward, to be split evenly among block signers.
	BlockReward = new(big.Int).Mul(big.NewInt(24), big.NewInt(denominations.One))
	// BlockRewardStakedCase is the baseline block reward in staked case -
	BlockRewardStakedCase = numeric.NewDecFromBigInt(new(big.Int).Mul(
		big.NewInt(18), big.NewInt(denominations.One),
	))

	errPayoutNotEqualBlockReward = errors.New("total payout not equal to blockreward")
)

func blockSigners(
	header *block.Header, parentCommittee *shard.Committee,
) (shard.SlotList, shard.SlotList, error) {
	committerKeys := []*bls.PublicKey{}

	for _, member := range parentCommittee.Slots {
		committerKey := new(bls.PublicKey)
		err := member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			return nil, nil, ctxerror.New(
				"cannot convert BLS public key",
				"blsPublicKey",
				member.BlsPublicKey,
			).WithCause(err)
		}
		committerKeys = append(committerKeys, committerKey)
	}
	mask, err := bls2.NewMask(committerKeys, nil)
	if err != nil {
		return nil, nil, ctxerror.New(
			"cannot create group sig mask",
		).WithCause(err)
	}
	if err := mask.SetMask(header.LastCommitBitmap()); err != nil {
		return nil, nil, ctxerror.New(
			"cannot set group sig mask bits",
		).WithCause(err)
	}

	payable, missing := shard.SlotList{}, shard.SlotList{}

	for idx, member := range parentCommittee.Slots {
		switch signed, err := mask.IndexEnabled(idx); true {
		case err != nil:
			return nil, nil, ctxerror.New("cannot check for committer bit",
				"committerIndex", idx,
			).WithCause(err)
		case signed:
			payable = append(payable, member)
		default:
			missing = append(missing, member)
		}
	}
	return payable, missing, nil
}

func ballotResult(
	bc engine.ChainReader, header *block.Header, shardID uint32,
) (shard.SlotList, shard.SlotList, shard.SlotList, error) {
	// TODO ek – retrieving by parent number (blockNum - 1) doesn't work,
	//  while it is okay with hash.  Sounds like DB inconsistency.
	//  Figure out why.
	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader == nil {
		return nil, nil, nil, ctxerror.New(
			"cannot find parent block header in DB",
			"parentHash", header.ParentHash(),
		)
	}
	parentShardState, err := bc.ReadShardState(parentHeader.Epoch())
	if err != nil {
		return nil, nil, nil, ctxerror.New(
			"cannot read shard state", "epoch", parentHeader.Epoch(),
		).WithCause(err)
	}
	parentCommittee := parentShardState.FindCommitteeByID(shardID)

	if parentCommittee == nil {
		return nil, nil, nil, ctxerror.New(
			"cannot find shard in the shard state",
			"parentBlockNumber", parentHeader.Number(),
			"shardID", parentHeader.ShardID(),
		)
	}

	payable, missing, err := blockSigners(header, parentCommittee)
	return parentCommittee.Slots, payable, missing, err
}

func ballotResultBeaconchain(
	bc engine.ChainReader, header *block.Header,
) (shard.SlotList, shard.SlotList, shard.SlotList, error) {
	return ballotResult(bc, header, shard.BeaconChainShardID)
}

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func AccumulateRewards(
	bc engine.ChainReader, state *state.DB, header *block.Header,
	rewarder reward.Distributor, slasher slash.Slasher,
	beaconChain engine.ChainReader,
) error {

	blockNum := header.Number().Uint64()
	if blockNum == 0 {
		// genesis block has no parent to reward.
		return nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() != shard.BeaconChainShardID {
		return nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() == shard.BeaconChainShardID {

		// Take care of my own beacon chain committee, _ is missing, for slashing
		members, payable, _, err := ballotResultBeaconchain(beaconChain, header)
		if err != nil {
			return err
		}

		votingPower := votepower.Compute(members)

		for beaconMember := range payable {
			// TODO Give out whatever leftover to the last voter/handle
			// what to do about share of those that didn't sign
			voter := votingPower.Voters[payable[beaconMember].BlsPublicKey]
			if !voter.IsHarmonyNode {
				due := BlockRewardStakedCase.Mul(
					voter.EffectivePercent.Quo(votepower.StakersShare),
				)
				state.AddBalance(voter.EarningAccount, due.RoundInt())
			}
		}

		// Handle rewards for shardchain
		if cxLinks := header.CrossLinks(); len(cxLinks) != 0 {
			crossLinks := types.CrossLinks{}
			err := rlp.DecodeBytes(cxLinks, &crossLinks)
			if err != nil {
				return err
			}

			w := sync.WaitGroup{}

			type slotPayable struct {
				effective numeric.Dec
				payee     common.Address
				bucket    int
				index     int
				oops      error
			}

			payable := make(chan slotPayable)

			slotError := func(err error, receive chan slotPayable) {
				s := slotPayable{}
				s.oops = err
				go func() {
					receive <- s
				}()
			}

			for i := range crossLinks {
				w.Add(1)

				go func(i int) {
					defer w.Done()
					cxLink := crossLinks[i]
					subCommittee := shard.State{}
					if err := rlp.DecodeBytes(
						cxLink.ChainHeader.ShardState(), &subCommittee,
					); err != nil {
						slotError(err, payable)
						return
					}

					subComm := subCommittee.FindCommitteeByID(cxLink.ShardID())
					// _ are the missing signers, later for slashing
					payableSigners, _, err := blockSigners(cxLink.Header(), subComm)
					votingPower := votepower.Compute(subComm.Slots)

					if err != nil {
						slotError(err, payable)
						return
					}

					for member := range payableSigners {
						voter := votingPower.Voters[payableSigners[member].BlsPublicKey]
						if !voter.IsHarmonyNode {
							due := BlockRewardStakedCase.Mul(
								voter.EffectivePercent.Quo(votepower.StakersShare),
							)
							to := voter.EarningAccount
							go func(signersDue numeric.Dec, addr common.Address, j int) {
								payable <- slotPayable{
									effective: signersDue,
									payee:     addr,
									bucket:    i,
									index:     j,
									oops:      nil,
								}
							}(due, to, member)
						}
					}
				}(i)
			}

			w.Wait()
			resultsHandle := make([][]slotPayable, len(crossLinks))
			for i := range resultsHandle {
				resultsHandle[i] = []slotPayable{}
			}

			for payThem := range payable {
				bucket := payThem.bucket
				resultsHandle[bucket] = append(resultsHandle[bucket], payThem)
			}

			// Check if any errors and sort each bucket to enforce order
			for bucket := range resultsHandle {
				for payThem := range resultsHandle[bucket] {
					if err := resultsHandle[bucket][payThem].oops; err != nil {
						return err
					}
				}

				sort.SliceStable(resultsHandle[bucket],
					func(i, j int) bool {
						return resultsHandle[bucket][i].index < resultsHandle[bucket][j].index
					},
				)
			}

			// Finally do the pay
			for bucket := range resultsHandle {
				for payThem := range resultsHandle[bucket] {
					state.AddBalance(
						resultsHandle[bucket][payThem].payee,
						resultsHandle[bucket][payThem].effective.TruncateInt(),
					)
				}
			}
		}
		return nil
	}

	payable := []struct {
		string
		common.Address
		*big.Int
	}{}

	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader.Number().Cmp(common.Big0) == 0 {
		// Parent is an epoch block,
		// which is not signed in the usual manner therefore rewards nothing.
		return nil
	}

	_, signers, _, err := ballotResult(bc, header, header.ShardID())

	if err != nil {
		return err
	}

	totalAmount := rewarder.Award(
		BlockReward, signers, func(receipient common.Address, amount *big.Int) {
			payable = append(payable, struct {
				string
				common.Address
				*big.Int
			}{common2.MustAddressToBech32(receipient), receipient, amount},
			)
		},
	)

	if totalAmount.Cmp(BlockReward) != 0 {
		utils.Logger().Error().
			Int64("block-reward", BlockReward.Int64()).
			Int64("total-amount-paid-out", totalAmount.Int64()).
			Msg("Total paid out was not equal to block-reward")
		return errors.Wrapf(
			errPayoutNotEqualBlockReward, "payout "+totalAmount.String(),
		)
	}

	for i := range payable {
		state.AddBalance(payable[i].Address, payable[i].Int)
	}

	header.Logger(utils.Logger()).Debug().
		Int("NumAccounts", len(payable)).
		Str("TotalAmount", totalAmount.String()).
		Msg("[Block Reward] Successfully paid out block reward")

	return nil
}
