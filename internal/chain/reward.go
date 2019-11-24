package chain

import (
	"fmt"
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
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/pkg/errors"
)

var (
	// BlockReward is the block reward, to be split evenly among block signers.
	BlockReward = numeric.NewDecFromBigInt(
		new(big.Int).Mul(big.NewInt(24), big.NewInt(denominations.One)),
	)
	// BlockRewardStakedCase is the baseline block reward in staked case -
	BlockRewardStakedCase = numeric.NewDecFromBigInt(new(big.Int).Mul(
		big.NewInt(18), big.NewInt(denominations.One),
	))

	errPayoutNotEqualBlockReward = errors.New("total payout not equal to blockreward")
)

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
	if parentHeader.Number().Cmp(common.Big0) == 0 {
		// Parent is an epoch block,
		// which is not signed in the usual manner therefore rewards nothing.
		return shard.SlotList{}, shard.SlotList{}, shard.SlotList{}, nil
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

	committerKeys := []*bls.PublicKey{}
	for _, member := range parentCommittee.Slots {
		committerKey := new(bls.PublicKey)
		err := member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			return nil, nil, nil, ctxerror.New(
				"cannot convert BLS public key",
				"blsPublicKey",
				member.BlsPublicKey,
			).WithCause(err)
		}
		committerKeys = append(committerKeys, committerKey)
	}
	mask, err := bls2.NewMask(committerKeys, nil)
	if err != nil {
		return nil, nil, nil, ctxerror.New(
			"cannot create group sig mask",
		).WithCause(err)
	}
	if err := mask.SetMask(header.LastCommitBitmap()); err != nil {
		return nil, nil, nil, ctxerror.New(
			"cannot set group sig mask bits",
		).WithCause(err)
	}

	payable, missing := shard.SlotList{}, shard.SlotList{}

	for idx, member := range parentCommittee.Slots {
		switch signed, err := mask.IndexEnabled(idx); true {
		case err != nil:
			return nil, nil, nil, ctxerror.New("cannot check for committer bit",
				"committerIndex", idx,
			).WithCause(err)
		case signed:
			payable = append(payable, member)
		default:
			missing = append(missing, member)
		}
	}
	return parentCommittee.Slots, payable, missing, nil
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
		// Epoch block has no parent to reward.
		return nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() != shard.BeaconChainShardID {
		return nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() == shard.BeaconChainShardID {

		// Take care of my own beacon chain committee, _ is missing, for slashing
		members, payable, _, err := ballotResultBeaconchain(bc, header)
		if err != nil {
			return err
		}

		votingPower := votepower.Compute(members)

		for beaconMember := range payable {
			voter := votingPower.Voters[payable[beaconMember].BlsPublicKey]
			if !voter.IsHarmonyNode {
				due := BlockRewardStakedCase.Mul(voter.EffectivePercent)
				state.AddBalance(voter.EarningAccount, due.RoundInt())
			}
		}

		// Handle rewards for shardchain
		if cxLinks := header.CrossLinks(); len(cxLinks) != 0 {
			crossLinks := types.CrossLinks{}
			err := rlp.DecodeBytes(cxLinks, &crossLinks)
			if err != nil {
				fmt.Println("cross-link-error", err)
				return err
			}

			w := sync.WaitGroup{}

			type slotPayable struct {
				effective numeric.Dec
				payee     common.Address
				nonce     int
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

					if subComm == nil {
						fmt.Println("oops", cxLink.ShardID(), subCommittee.JSON())
					}

					// Assume index is 1-1 for these []s
					stakers, publicKeys := subComm.Slots.OnlyStaked()
					mask, err := bls2.NewMask(publicKeys, nil)
					if err != nil {
						slotError(err, payable)
						return
					}
					commitBitmap := cxLink.Header().LastCommitBitmap()

					if err := mask.SetMask(commitBitmap); err != nil {
						slotError(ctxerror.New(
							"cannot set group sig mask bits",
						).WithCause(err), payable)
						return
					}

					totalAmount := numeric.ZeroDec()

					for j := range stakers {
						totalAmount = totalAmount.Add(*stakers[j].StakeWithDelegationApplied)
					}

					for j := range stakers {
						switch signed, err := mask.IndexEnabled(j); true {
						case err != nil:
							slotError(ctxerror.New(
								"cannot check for committer bit", "committerIndex", j,
							).WithCause(err), payable)
							return
						case signed:
							go func(signersDue numeric.Dec, addr common.Address, j int) {
								payable <- slotPayable{
									effective: signersDue,
									payee:     addr,
									nonce:     i * (i + j),
									oops:      nil,
								}
							}((*stakers[j].StakeWithDelegationApplied).Quo(
								totalAmount,
							).Mul(BlockRewardStakedCase), stakers[j].EcdsaAddress, j)
						}
					}
				}(i)
			}

			w.Wait()
			resultsHandle := []slotPayable{}

			for payThem := range payable {
				resultsHandle = append(resultsHandle, payThem)
			}

			// Check if any errors
			for payThem := range resultsHandle {
				if err := resultsHandle[payThem].oops; err != nil {
					return err
				}
			}
			// Enforce order
			sort.SliceStable(resultsHandle,
				func(i, j int) bool { return resultsHandle[i].nonce < resultsHandle[j].nonce },
			)

			// Finally do the pay
			for payThem := range resultsHandle {
				state.AddBalance(
					resultsHandle[payThem].payee, resultsHandle[payThem].effective.TruncateInt(),
				)
			}
		}
	}

	// wholePie := BlockRewardStakedCase
	// totalAmount := numeric.ZeroDec()

	// // Legacy logic
	// if bc.Config().IsStaking(header.Epoch()) == false {
	// 	wholePie = BlockReward
	// 	payable := []struct {
	// 		string
	// 		common.Address
	// 		*big.Int
	// 	}{}

	// totalAmount = rewarder.Award(
	// 	wholePie, accounts, func(receipient common.Address, amount *big.Int) {
	// 		payable = append(payable, struct {
	// 			string
	// 			common.Address
	// 			*big.Int
	// 		}{common2.MustAddressToBech32(receipient), receipient, amount},
	// 		)
	// 	},
	// )

	// if totalAmount.Equal(BlockReward) == false {
	// 	utils.Logger().Error().
	// 		Int64("block-reward", BlockReward.Int64()).
	// 		Int64("total-amount-paid-out", totalAmount.Int64()).
	// 		Msg("Total paid out was not equal to block-reward")
	// 	return errors.Wrapf(
	// 		errPayoutNotEqualBlockReward, "payout "+totalAmount.String(),
	// 	)
	// }
	// signers := make([]string, len(payable))

	// for i := range payable {
	// 	signers[i] = payable[i].string
	// 	state.AddBalance(payable[i].Address, payable[i].Int)
	// }

	// header.Logger(utils.Logger()).Debug().
	// 	Int("NumAccounts", len(accounts)).
	// 	Str("TotalAmount", totalAmount.String()).
	// 	Strs("Signers", signers).
	// 	Msg("[Block Reward] Successfully paid out block reward")
	// } else {
	// 	// TODO Beaconchain is still a shard, so need take care of my own committee (same logic %staked
	// 	// vote payout)

	// }

	return nil
}
