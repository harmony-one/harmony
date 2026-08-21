package consensus

import (
	"bytes"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/crypto/bls"
	bls_core "github.com/harmony-one/harmony/crypto/bls/core"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
)

// priorCommitBallots inspects the commit ballots already on record for the senders of
// recvMsg and reports what they say about it.
//
// The first return is true when any sender has already cast a commit ballot, which makes
// recvMsg a repeat that the caller does not count. The second holds those ballots that
// name a block other than the one recvMsg votes for. Since onCommit admits a message
// only for the height and viewID this node is currently deciding, and a stored ballot
// was admitted under the same height and viewID, such a ballot and recvMsg are two votes
// cast by one signer for two blocks at a single height, which is what a double sign is.
//
// It is nil whenever every prior ballot names the same block, so an ordinary repeat
// costs one comparison per sender and no allocation. A validator holding several keys
// casts one message covering all of them and the same ballot is filed under each, so a
// ballot already collected is not collected again.
func (consensus *Consensus) priorCommitBallots(
	recvMsg *FBFTMessage,
) (bool, []*votepower.Ballot) {
	var (
		seen        bool
		conflicting []*votepower.Ballot
	)
	for _, signer := range recvMsg.SenderPubkeys {
		signed := consensus.decider().ReadBallot(quorum.Commit, signer.Bytes)
		if signed == nil {
			continue
		}
		seen = true
		if signed.BlockHeaderHash == recvMsg.BlockHash {
			continue
		}
		alreadyHeld := false
		for _, held := range conflicting {
			if held == signed {
				alreadyHeld = true
				break
			}
		}
		if !alreadyHeld {
			conflicting = append(conflicting, signed)
		}
	}
	if seen {
		consensus.getLogger().Debug().
			Uint64("blockNum", recvMsg.BlockNum).
			Str("blockHash", recvMsg.BlockHash.Hex()).
			Msg("[OnCommit] Already Received commit message from the validator")
	}
	return seen, conflicting
}

// reportDoubleSign turns each conflicting pair of commit ballots into a slash record and
// hands it to the node, one record per offending validator.
//
// A record is emitted only for evidence that stands on its own: the incoming signature
// verifies, both ballots carry the same signer keys, and those keys resolve to a single
// validator in the committee for the epoch. The beacon chain re-checks all of this
// before a record reaches a block, and holding to the same conditions here keeps the
// pending set to records that can be acted on.
func (consensus *Consensus) reportDoubleSign(
	recvMsg *FBFTMessage, conflicting []*votepower.Ballot,
) {
	if len(conflicting) == 0 {
		return
	}
	bc := consensus.Blockchain()
	if bc == nil {
		return
	}
	// slash.Verify reads the evidence epoch from the height the evidence names, and
	// rebuilds the commit payload it verifies both ballots against from that epoch.
	// Deriving it here the same way keeps the two in agreement at an epoch boundary,
	// where the chain head still belongs to the epoch before the block being voted on.
	evidenceEpoch := shard.Schedule.CalcEpochNumber(recvMsg.BlockNum)
	if !bc.Config().IsDoubleSignSlash(evidenceEpoch) {
		return
	}

	leaderKey := consensus.getLeaderPubKey()
	if leaderKey == nil {
		return
	}
	secondVote, ok := consensus.voteFromCommitMessage(recvMsg, evidenceEpoch)
	if !ok {
		return
	}

	shardState, err := bc.ReadShardState(evidenceEpoch)
	if err != nil {
		consensus.getLogger().Err(err).
			Uint64("epoch", evidenceEpoch.Uint64()).
			Msg("[DoubleSign] could not read shard state for the evidence epoch")
		return
	}
	subComm, err := shardState.FindCommitteeByID(consensus.ShardID)
	if err != nil {
		consensus.getLogger().Err(err).
			Uint32("shard", consensus.ShardID).
			Msg("[DoubleSign] could not find subcommittee for the evidence epoch")
		return
	}
	reporter, err := subComm.AddressForBLSKey(leaderKey.Bytes)
	if err != nil {
		consensus.getLogger().Err(err).
			Msg("[DoubleSign] could not find address for the leader bls key")
		return
	}

	reported := map[common.Address]struct{}{}
	for _, ballot := range conflicting {
		firstVote := slash.Vote{
			SignerPubKeys:   ballot.SignerPubKeys,
			BlockHeaderHash: ballot.BlockHeaderHash,
			Signature:       ballot.Signature,
		}
		offenders := sharedSignerKeys(firstVote.SignerPubKeys, secondVote.SignerPubKeys)
		// Both ballots must be signed by the same set of keys for the pair to describe
		// one validator voting twice, which is the shape verification accepts.
		if len(offenders) == 0 ||
			len(offenders) != len(firstVote.SignerPubKeys) ||
			len(offenders) != len(secondVote.SignerPubKeys) {
			continue
		}
		addr, err := subComm.AddressForBLSKey(offenders[0])
		if err != nil {
			consensus.getLogger().Err(err).
				Msg("[DoubleSign] could not find address for the signer bls key")
			continue
		}
		if _, done := reported[*addr]; done {
			continue
		}
		// A record names its reporter as the party that witnessed the offence, so the
		// two have to be different validators for the record to hold.
		if *addr == *reporter {
			consensus.getLogger().Warn().
				Str("offender", addr.Hex()).
				Msg("[DoubleSign] offender is the reporting leader, leaving it to another witness")
			continue
		}
		reported[*addr] = struct{}{}

		record := slash.Record{
			Evidence: slash.Evidence{
				ConflictingVotes: slash.ConflictingVotes{
					FirstVote:  sortedVote(firstVote),
					SecondVote: sortedVote(secondVote),
				},
				Moment: slash.Moment{
					Epoch:   evidenceEpoch,
					ShardID: consensus.ShardID,
					Height:  recvMsg.BlockNum,
					ViewID:  recvMsg.ViewID,
				},
				Offender: *addr,
			},
			Reporter: *reporter,
		}
		consensus.getLogger().Warn().
			Str("offender", addr.Hex()).
			Uint64("height", recvMsg.BlockNum).
			Uint64("view-id", recvMsg.ViewID).
			Str("first-hash", ballot.BlockHeaderHash.Hex()).
			Str("second-hash", recvMsg.BlockHash.Hex()).
			Msg("[DoubleSign] two commit ballots at one height, reporting for slash")
		consensus.submitSlashRecord(record)
	}
}

// voteFromCommitMessage renders recvMsg as a slash ballot once its signature verifies
// against the commit payload for the evidence epoch.
//
// The payload is rebuilt from the fields of the message rather than looked up from a
// stored block, because the block a conflicting ballot names is one this node has no
// reason to hold, and slash.Verify rebuilds it from the same fields.
func (consensus *Consensus) voteFromCommitMessage(
	recvMsg *FBFTMessage, evidenceEpoch *big.Int,
) (slash.Vote, bool) {
	var sign bls_core.Sign
	if err := sign.Deserialize(recvMsg.Payload); err != nil {
		consensus.getLogger().Err(err).
			Msg("[DoubleSign] could not deserialize the commit signature")
		return slash.Vote{}, false
	}
	aggregated := &bls_core.PublicKey{}
	keys := make([]bls.SerializedPublicKey, 0, len(recvMsg.SenderPubkeys))
	for _, pubKey := range recvMsg.SenderPubkeys {
		if pubKey == nil || pubKey.Object == nil {
			return slash.Vote{}, false
		}
		aggregated.Add(pubKey.Object)
		keys = append(keys, pubKey.Bytes)
	}
	if len(keys) == 0 {
		return slash.Vote{}, false
	}
	commitPayload := signature.ConstructCommitPayload(
		consensus.Blockchain().Config(),
		evidenceEpoch, recvMsg.BlockHash, recvMsg.BlockNum, recvMsg.ViewID,
	)
	if !sign.VerifyHash(aggregated, commitPayload) {
		consensus.getLogger().Warn().
			Str("recvMsg", recvMsg.String()).
			Msg("[DoubleSign] commit signature does not verify, no evidence to report")
		return slash.Vote{}, false
	}
	return slash.Vote{
		SignerPubKeys:   keys,
		BlockHeaderHash: recvMsg.BlockHash,
		Signature:       recvMsg.Payload,
	}, true
}

// submitSlashRecord hands a record to the node, which gossips it to the beacon chain or
// queues it locally. The send never waits: consensus holds its lock while deciding a
// commit, so a report yields to block production rather than delaying it.
func (consensus *Consensus) submitSlashRecord(record slash.Record) {
	select {
	case consensus.SlashChan <- record:
	default:
		consensus.getLogger().Error().
			RawJSON("record", []byte(record.String())).
			Msg("[DoubleSign] slash channel is full, record not queued")
	}
}

// sharedSignerKeys returns the keys present in both ballots, which are the keys that
// signed each of the two blocks. Order follows first so the result is stable.
func sharedSignerKeys(first, second []bls.SerializedPublicKey) []bls.SerializedPublicKey {
	shared := []bls.SerializedPublicKey{}
	for _, a := range first {
		for _, b := range second {
			if shard.CompareBLSPublicKey(a, b) == 0 {
				shared = append(shared, a)
				break
			}
		}
	}
	return shared
}

// sortedVote returns the vote with its signer keys in canonical order, so that one
// offence witnessed more than once yields records that hash alike and deduplicate.
func sortedVote(v slash.Vote) slash.Vote {
	keys := make([]bls.SerializedPublicKey, len(v.SignerPubKeys))
	copy(keys, v.SignerPubKeys)
	sort.SliceStable(keys, func(i, j int) bool {
		return bytes.Compare(keys[i][:], keys[j][:]) < 0
	})
	v.SignerPubKeys = keys
	return v
}
