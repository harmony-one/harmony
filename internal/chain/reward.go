package chain

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
)

// BlockReward is the block reward, to be split evenly among block signers.
var BlockReward = new(big.Int).Mul(big.NewInt(24), big.NewInt(denominations.One))

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func AccumulateRewards(
	bc engine.ChainReader, state *state.DB, header *block.Header,
) error {
	blockNum := header.Number.Uint64()
	if blockNum == 0 {
		// Epoch block has no parent to reward.
		return nil
	}
	// TODO ek – retrieving by parent number (blockNum - 1) doesn't work,
	//  while it is okay with hash.  Sounds like DB inconsistency.
	//  Figure out why.
	parentHeader := bc.GetHeaderByHash(header.ParentHash)
	if parentHeader == nil {
		return ctxerror.New("cannot find parent block header in DB",
			"parentHash", header.ParentHash)
	}
	if parentHeader.Number.Cmp(common.Big0) == 0 {
		// Parent is an epoch block,
		// which is not signed in the usual manner therefore rewards nothing.
		return nil
	}
	parentShardState, err := bc.ReadShardState(parentHeader.Epoch)
	if err != nil {
		return ctxerror.New("cannot read shard state",
			"epoch", parentHeader.Epoch,
		).WithCause(err)
	}
	parentCommittee := parentShardState.FindCommitteeByID(parentHeader.ShardID)
	if parentCommittee == nil {
		return ctxerror.New("cannot find shard in the shard state",
			"parentBlockNumber", parentHeader.Number,
			"shardID", parentHeader.ShardID,
		)
	}
	var committerKeys []*bls.PublicKey
	for _, member := range parentCommittee.NodeList {
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
	if err := mask.SetMask(header.LastCommitBitmap); err != nil {
		return ctxerror.New("cannot set group sig mask bits").WithCause(err)
	}
	totalAmount := big.NewInt(0)
	var accounts []common.Address
	signers := []string{}
	for idx, member := range parentCommittee.NodeList {
		if signed, err := mask.IndexEnabled(idx); err != nil {
			return ctxerror.New("cannot check for committer bit",
				"committerIndex", idx,
			).WithCause(err)
		} else if signed {
			accounts = append(accounts, member.EcdsaAddress)
		}
	}
	numAccounts := big.NewInt(int64(len(accounts)))
	last := new(big.Int)
	for i, account := range accounts {
		cur := new(big.Int)
		cur.Mul(BlockReward, big.NewInt(int64(i+1))).Div(cur, numAccounts)
		diff := new(big.Int).Sub(cur, last)
		signers = append(signers, common2.MustAddressToBech32(account))
		state.AddBalance(account, diff)
		totalAmount = new(big.Int).Add(totalAmount, diff)
		last = cur
	}
	header.Logger(utils.Logger()).Debug().
		Str("NumAccounts", numAccounts.String()).
		Str("TotalAmount", totalAmount.String()).
		Strs("Signers", signers).
		Msg("[Block Reward] Successfully paid out block reward")
	return nil
}
