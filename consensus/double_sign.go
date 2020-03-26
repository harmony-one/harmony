package consensus

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
)

// Check for double sign and if any, send it out to beacon chain for slashing.
func (consensus *Consensus) checkDoubleSign(recvMsg *FBFTMessage) {
	if consensus.couldThisBeADoubleSigner(recvMsg) {
		if alreadyCastBallot := consensus.Decider.ReadBallot(
			quorum.Commit, recvMsg.SenderPubkey,
		); alreadyCastBallot != nil {
			firstPubKey := bls.PublicKey{}
			alreadyCastBallot.SignerPubKey.ToLibBLSPublicKey(&firstPubKey)
			if recvMsg.SenderPubkey.IsEqual(&firstPubKey) {
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
						var doubleSign bls.Sign
						if err := doubleSign.Deserialize(recvMsg.Payload); err != nil {
							consensus.getLogger().Err(err).Str("msg", recvMsg.String()).
								Msg("could not deserialize potential double signer")
							return
						}

						curHeader := consensus.ChainReader.CurrentHeader()
						committee, err := consensus.ChainReader.ReadShardState(curHeader.Epoch())
						if err != nil {
							consensus.getLogger().Err(err).
								Uint32("shard", consensus.ShardID).
								Uint64("epoch", curHeader.Epoch().Uint64()).
								Msg("could not read shard state")
							return
						}
						offender := *shard.FromLibBLSPublicKeyUnsafe(recvMsg.SenderPubkey)
						subComm, err := committee.FindCommitteeByID(
							consensus.ShardID,
						)
						if err != nil {
							consensus.getLogger().Err(err).
								Str("msg", recvMsg.String()).
								Msg("could not find subcommittee for bls key")
							return
						}

						addr, err := subComm.AddressForBLSKey(offender)

						if err != nil {
							consensus.getLogger().Err(err).Str("msg", recvMsg.String()).
								Msg("could not find address for bls key")
							return
						}

						now := big.NewInt(time.Now().UnixNano())

						go func(reporter common.Address) {
							evid := slash.Evidence{
								ConflictingBallots: slash.ConflictingBallots{
									AlreadyCastBallot: *alreadyCastBallot,
									DoubleSignedBallot: votepower.Ballot{
										SignerPubKey:    offender,
										BlockHeaderHash: recvMsg.BlockHash,
										Signature:       common.Hex2Bytes(doubleSign.SerializeToHexStr()),
										Height:          recvMsg.BlockNum,
										ViewID:          recvMsg.ViewID,
									}},
								Moment: slash.Moment{
									Epoch:        curHeader.Epoch(),
									ShardID:      consensus.ShardID,
									TimeUnixNano: now,
								},
							}
							proof := slash.Record{
								Evidence: evid,
								Reporter: reporter,
								Offender: *addr,
							}
							consensus.SlashChan <- proof
						}(consensus.SelfAddresses[consensus.LeaderPubKey.SerializeToHexStr()])
						return
					}
				}
			}
		}
		return
	}
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
