package consensus

import (
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
		if alreadyCastBallot := consensus.Decider.ReadBallot(
			quorum.Commit, recvMsg.SenderPubkey.Bytes,
		); alreadyCastBallot != nil {
			firstPubKey, err := bls.BytesToBLSPublicKey(alreadyCastBallot.SignerPubKey[:])
			if err != nil {
				return false
			}
			if recvMsg.SenderPubkey.Object.IsEqual(firstPubKey) {
				for _, blk := range consensus.FBFTLog.GetBlocksByNumber(recvMsg.BlockNum) {
					firstSignedBlock := blk.Header()
					areHeightsEqual := firstSignedBlock.Number().Uint64() == recvMsg.BlockNum
					areViewIDsEqual := firstSignedBlock.ViewID().Uint64() == recvMsg.ViewID
					areHeadersEqual := firstSignedBlock.Hash() == recvMsg.BlockHash

					// If signer already firstSignedBlock, and the block height is the same
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

						curHeader := consensus.ChainReader.CurrentHeader()
						committee, err := consensus.ChainReader.ReadShardState(curHeader.Epoch())
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

						addr, err := subComm.AddressForBLSKey(recvMsg.SenderPubkey.Bytes)
						if err != nil {
							consensus.getLogger().Err(err).Str("msg", recvMsg.String()).
								Msg("could not find address for bls key")
							return true
						}

						leaderAddr, err := subComm.AddressForBLSKey(consensus.LeaderPubKey.Bytes)
						if err != nil {
							consensus.getLogger().Err(err).Str("msg", recvMsg.String()).
								Msg("could not find address for leader bls key")
							return true
						}

						go func(reporter common.Address) {
							evid := slash.Evidence{
								ConflictingVotes: slash.ConflictingVotes{
									FirstVote: slash.Vote{
										alreadyCastBallot.SignerPubKey,
										alreadyCastBallot.BlockHeaderHash,
										alreadyCastBallot.Signature,
									},
									SecondVote: slash.Vote{
										recvMsg.SenderPubkey.Bytes,
										recvMsg.BlockHash,
										common.Hex2Bytes(doubleSign.SerializeToHexStr()),
									}},
								Moment: slash.Moment{
									Epoch:   curHeader.Epoch(),
									ShardID: consensus.ShardID,
								},
								Offender: *addr,
							}
							proof := slash.Record{
								Evidence: evid,
								Reporter: reporter,
							}
							consensus.SlashChan <- proof
						}(*leaderAddr)
						return true
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
	num, hash := consensus.blockNum, recvMsg.BlockHash
	suspicious := !consensus.FBFTLog.HasMatchingAnnounce(num, hash) ||
		!consensus.FBFTLog.HasMatchingPrepared(num, hash)
	if suspicious {
		consensus.getLogger().Debug().
			Str("message", recvMsg.String()).
			Uint64("block-on-consensus", num).
			Msg("possible double signer")
		return true
	}
	return false
}
