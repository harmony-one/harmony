package consensus

import (
	"bytes"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/staking/slash"
)

// Check for double sign and if any, send it out to beacon chain for slashing.
// Returns true when it is a double-sign or there is error, otherwise, false.
func (consensus *Consensus) checkDoubleSign(recvMsg *FBFTMessage) bool {
	if consensus.couldThisBeADoubleSigner(recvMsg) {
		addrSet := map[common.Address]struct{}{}
		for _, pubKey2 := range recvMsg.SenderPubkeys {
			if alreadyCastBallot := consensus.decider.ReadBallot(
				quorum.Commit, pubKey2.Bytes,
			); alreadyCastBallot != nil {
				for _, pubKey1 := range alreadyCastBallot.SignerPubKeys {
					if bytes.Compare(pubKey2.Bytes[:], pubKey1[:]) == 0 {
						for _, blk := range consensus.fBFTLog.GetBlocksByNumber(recvMsg.BlockNum) {
							firstSignedHeader := blk.Header()
							areHeightsEqual := firstSignedHeader.Number().Uint64() == recvMsg.BlockNum
							areViewIDsEqual := firstSignedHeader.ViewID().Uint64() == recvMsg.ViewID
							areHeadersEqual := firstSignedHeader.Hash() == recvMsg.BlockHash

							// If signer already firstSignedHeader, and the block height is the same
							// and the viewID is the same, then we need to verify the block
							// hash, and if block hash is different, then that is a clear
							// case of double signing
							if areHeightsEqual && areViewIDsEqual && !areHeadersEqual {
								var doubleSign bls_core.Sign
								if err := doubleSign.Deserialize(recvMsg.Payload); err != nil {
									consensus.getLogger().Err(err).Str("msg", recvMsg.String()).
										Msg("could not deserialize potential double signer")
									return true
								}

								curHeader := consensus.Blockchain().CurrentHeader()
								committee, err := consensus.Blockchain().ReadShardState(curHeader.Epoch())
								if err != nil {
									consensus.getLogger().Err(err).
										Uint32("shard", consensus.ShardID).
										Uint64("epoch", curHeader.Epoch().Uint64()).
										Msg("could not read shard state")
									return true
								}

								subComm, err := committee.FindCommitteeByID(
									consensus.ShardID,
								)
								if err != nil {
									consensus.getLogger().Err(err).
										Str("msg", recvMsg.String()).
										Msg("could not find subcommittee for bls key")
									return true
								}

								addr, err := subComm.AddressForBLSKey(pubKey2.Bytes)
								if err != nil {
									consensus.getLogger().Err(err).Str("msg", recvMsg.String()).
										Msg("could not find address for bls key")
									return true
								}
								if _, ok := addrSet[*addr]; ok {
									// Address already slashed
									break
								}

								leaderAddr, err := subComm.AddressForBLSKey(consensus.LeaderPubKey.Bytes)
								if err != nil {
									consensus.getLogger().Err(err).Str("msg", recvMsg.String()).
										Msg("could not find address for leader bls key")
									return true
								}

								go func(reporter common.Address) {
									secondKeys := make([]bls.SerializedPublicKey, len(recvMsg.SenderPubkeys))
									for i, pubKey := range recvMsg.SenderPubkeys {
										secondKeys[i] = pubKey.Bytes
									}
									evid := slash.Evidence{
										ConflictingVotes: slash.ConflictingVotes{
											FirstVote: slash.Vote{
												SignerPubKeys:   alreadyCastBallot.SignerPubKeys,
												BlockHeaderHash: alreadyCastBallot.BlockHeaderHash,
												Signature:       alreadyCastBallot.Signature,
											},
											SecondVote: slash.Vote{
												SignerPubKeys:   secondKeys,
												BlockHeaderHash: recvMsg.BlockHash,
												Signature:       common.Hex2Bytes(doubleSign.SerializeToHexStr()),
											}},
										Moment: slash.Moment{
											Epoch:   curHeader.Epoch(),
											ShardID: consensus.ShardID,
											Height:  recvMsg.BlockNum,
											ViewID:  recvMsg.ViewID,
										},
										Offender: *addr,
									}

									sort.SliceStable(evid.ConflictingVotes.FirstVote.SignerPubKeys, func(i, j int) bool {
										return bytes.Compare(
											evid.ConflictingVotes.FirstVote.SignerPubKeys[i][:],
											evid.ConflictingVotes.FirstVote.SignerPubKeys[j][:]) < 0
									})
									sort.SliceStable(evid.ConflictingVotes.SecondVote.SignerPubKeys, func(i, j int) bool {
										return bytes.Compare(
											evid.ConflictingVotes.SecondVote.SignerPubKeys[i][:],
											evid.ConflictingVotes.SecondVote.SignerPubKeys[j][:]) < 0
									})
									proof := slash.Record{
										Evidence: evid,
										Reporter: reporter,
									}
									consensus.SlashChan <- proof
								}(*leaderAddr)
								addrSet[*addr] = struct{}{}
								break
							}
						}
					}
				}
			}
		}

		return true
	}
	return false
}

func (consensus *Consensus) couldThisBeADoubleSigner(
	recvMsg *FBFTMessage,
) bool {
	num, hash := consensus.BlockNum(), recvMsg.BlockHash
	suspicious := !consensus.fBFTLog.HasMatchingAnnounce(num, hash) ||
		!consensus.fBFTLog.HasMatchingPrepared(num, hash)
	if suspicious {
		consensus.getLogger().Debug().
			Str("message", recvMsg.String()).
			Uint64("block-on-consensus", num).
			Msg("possible double signer")
		return true
	}
	return false
}
