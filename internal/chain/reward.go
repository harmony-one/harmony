package chain

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
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
	BlockReward = numeric.NewDecFromBigInt(
		new(big.Int).Mul(big.NewInt(24), big.NewInt(denominations.One)),
	)
	// BlockRewardStakedCase is the baseline block reward in staked case -
	BlockRewardStakedCase = numeric.NewDecFromBigInt(new(big.Int).Mul(
		big.NewInt(18), big.NewInt(denominations.One),
	))

	errPayoutNotEqualBlockReward = errors.New("total payout not equal to blockreward")
)

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func AccumulateRewards(
	bc engine.ChainReader, state *state.DB, header *block.Header,
	rewarder reward.Distributor, slasher slash.Slasher,
	beaconChain engine.ChainReader,
) error {

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() != shard.BeaconChainShardID {
		return nil
	}

	if bc.Config().IsStaking(header.Epoch()) &&
		bc.CurrentHeader().ShardID() == shard.BeaconChainShardID {

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
				common.Address
				numeric.Dec
			}

			payable := make(chan slotPayable)

			for i := range crossLinks {
				w.Add(1)

				go func(i int) {
					defer w.Done()
					cxLink := crossLinks[i]
					subCommittee := shard.State{}
					if err := rlp.DecodeBytes(
						cxLink.ChainHeader.ShardState(), &subCommittee,
					); err != nil {
						//
						// return err
					}

					subComm := subCommittee.FindCommitteeByID(cxLink.ShardID())

					if subComm == nil {
						fmt.Println("oops", cxLink.ShardID(), subCommittee.JSON())
					}

					// Assume index is 1-1 for these []s
					stakers, publicKeys := subComm.Slots.OnlyStaked()
					mask, err := bls2.NewMask(publicKeys, nil)
					if err != nil {
						// return err
					}
					commitBitmap := cxLink.Header().LastCommitBitmap()

					if err := mask.SetMask(commitBitmap); err != nil {
						// return ctxerror.New("cannot set group sig mask bits").WithCause(err)
					}

					totalAmount := numeric.ZeroDec()

					for j := range stakers {
						totalAmount = totalAmount.Add(*stakers[j].StakeWithDelegationApplied)
					}

					for j := range stakers {
						switch signed, err := mask.IndexEnabled(j); true {
						case err != nil:
							// return ctxerror.New(
							// 	"cannot check for committer bit", "committerIndex", j,
							// ).WithCause(err)
						case signed:
							go func(signersDue numeric.Dec, addr common.Address) {
								payable <- slotPayable{addr, signersDue}
							}((*stakers[j].StakeWithDelegationApplied).Quo(
								totalAmount,
							).Mul(BlockRewardStakedCase), stakers[j].EcdsaAddress)
						}
					}
				}(i)
			}

			w.Wait()
			for payThem := range payable {
				state.AddBalance(payThem.Address, payThem.TruncateInt())
			}

		}
	}

	blockNum := header.Number().Uint64()
	if blockNum == 0 {
		// Epoch block has no parent to reward.
		return nil
	}
	// TODO ek – retrieving by parent number (blockNum - 1) doesn't work,
	//  while it is okay with hash.  Sounds like DB inconsistency.
	//  Figure out why.
	parentHeader := bc.GetHeaderByHash(header.ParentHash())
	if parentHeader == nil {
		return ctxerror.New("cannot find parent block header in DB",
			"parentHash", header.ParentHash())
	}
	if parentHeader.Number().Cmp(common.Big0) == 0 {
		// Parent is an epoch block,
		// which is not signed in the usual manner therefore rewards nothing.
		return nil
	}
	parentShardState, err := bc.ReadShardState(parentHeader.Epoch())
	if err != nil {
		return ctxerror.New("cannot read shard state",
			"epoch", parentHeader.Epoch(),
		).WithCause(err)
	}
	parentCommittee := parentShardState.FindCommitteeByID(parentHeader.ShardID())
	if parentCommittee == nil {
		return ctxerror.New("cannot find shard in the shard state",
			"parentBlockNumber", parentHeader.Number(),
			"shardID", parentHeader.ShardID(),
		)
	}
	var committerKeys []*bls.PublicKey
	for _, member := range parentCommittee.Slots {
		committerKey := new(bls.PublicKey)
		err := member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			return ctxerror.New("cannot convert BLS public key",
				"blsPublicKey", member.BlsPublicKey).WithCause(err)
		}
		committerKeys = append(committerKeys, committerKey)
	}
	mask, err := bls2.NewMask(committerKeys, nil)
	if err != nil {
		return ctxerror.New("cannot create group sig mask").WithCause(err)
	}
	if err := mask.SetMask(header.LastCommitBitmap()); err != nil {
		return ctxerror.New("cannot set group sig mask bits").WithCause(err)
	}

	accounts := []common.Address{}
	missing := shard.SlotList{}

	for idx, member := range parentCommittee.Slots {
		switch signed, err := mask.IndexEnabled(idx); true {
		case err != nil:
			return ctxerror.New("cannot check for committer bit",
				"committerIndex", idx,
			).WithCause(err)
		case signed:
			accounts = append(accounts, member.EcdsaAddress)
		default:
			missing = append(missing, member)
		}
	}

	// do it quickly, // TODO come back to this(slashing)
	// w := sync.WaitGroup{}
	// for i := range missing {
	// 	w.Add(1)
	// 	go func(member int) {
	// 		defer w.Done()
	// 		// Slash if missing block was long enough
	// 		if slasher.ShouldSlash(missing[member].BlsPublicKey) {
	// 			// TODO Logic
	// 		}
	// 	}(i)
	// }

	// w.Wait()

	wholePie := BlockRewardStakedCase
	totalAmount := numeric.ZeroDec()

	// Legacy logic
	if bc.Config().IsStaking(header.Epoch()) == false {
		wholePie = BlockReward
		payable := []struct {
			string
			common.Address
			*big.Int
		}{}

		totalAmount = rewarder.Award(
			wholePie, accounts, func(receipient common.Address, amount *big.Int) {
				payable = append(payable, struct {
					string
					common.Address
					*big.Int
				}{common2.MustAddressToBech32(receipient), receipient, amount},
				)
			},
		)

		if totalAmount.Equal(BlockReward) == false {
			utils.Logger().Error().
				Int64("block-reward", BlockReward.Int64()).
				Int64("total-amount-paid-out", totalAmount.Int64()).
				Msg("Total paid out was not equal to block-reward")
			return errors.Wrapf(
				errPayoutNotEqualBlockReward, "payout "+totalAmount.String(),
			)
		}
		signers := make([]string, len(payable))

		for i := range payable {
			signers[i] = payable[i].string
			state.AddBalance(payable[i].Address, payable[i].Int)
		}

		header.Logger(utils.Logger()).Debug().
			Int("NumAccounts", len(accounts)).
			Str("TotalAmount", totalAmount.String()).
			Strs("Signers", signers).
			Msg("[Block Reward] Successfully paid out block reward")
	} else {
		// TODO Beaconchain is still a shard, so need take care of my own committee (same logic %staked
		// vote payout)

	}

	return nil
}
