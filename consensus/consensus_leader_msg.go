package consensus

import (
	"bytes"
	"github.com/dedis/kyber"
	consensus2 "github.com/harmony-one/harmony/consensus/proto"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/log"
	proto_consensus "github.com/harmony-one/harmony/proto/consensus"
)

// Constructs the announce message
func (consensus *Consensus) constructAnnounceMessage() []byte {
	message := consensus2.Message{}

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	// n byte of block header
	message.Payload = consensus.blockHeader

	marshaledMessage, err := message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Announce message", "error", err)
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Announce message", "error", err)
	}
	consensus.Log.Info("New Announce", "NodeID", consensus.nodeID, "bitmap", consensus.bitmap)
	return proto_consensus.ConstructConsensusMessage(proto_consensus.Announce, marshaledMessage)
}

// Construct the challenge message, returning challenge message in bytes, challenge scalar and aggregated commmitment point.
func (consensus *Consensus) constructChallengeMessage(msgTypeToSend proto_consensus.MessageType) ([]byte, kyber.Scalar, kyber.Point) {
	message := consensus2.Message{}

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	//// Payload
	buffer := bytes.NewBuffer([]byte{})

	commitmentsMap := consensus.commitments // msgType == Challenge
	bitmap := consensus.bitmap
	if msgTypeToSend == proto_consensus.FinalChallenge {
		commitmentsMap = consensus.finalCommitments
		bitmap = consensus.finalBitmap
	}

	// 33 byte aggregated commit
	commitments := make([]kyber.Point, 0)
	for _, val := range *commitmentsMap {
		commitments = append(commitments, val)
	}
	aggCommitment, aggCommitmentBytes := getAggregatedCommit(commitments)
	buffer.Write(aggCommitmentBytes)

	// 33 byte aggregated key
	buffer.Write(getAggregatedKey(bitmap))

	// 32 byte challenge
	challengeScalar := getChallenge(aggCommitment, bitmap.AggregatePublic, message.BlockHash)
	bytes, err := challengeScalar.MarshalBinary()
	if err != nil {
		log.Error("Failed to serialize challenge")
	}
	buffer.Write(bytes)

	message.Payload = buffer.Bytes()
	//// END Payload

	marshaledMessage, err := message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Challenge message", "error", err)
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Challenge message", "error", err)
	}
	consensus.Log.Info("New Challenge", "NodeID", consensus.nodeID, "bitmap", consensus.bitmap)
	return proto_consensus.ConstructConsensusMessage(msgTypeToSend, marshaledMessage), challengeScalar, aggCommitment
}

// Construct the collective signature message
func (consensus *Consensus) constructCollectiveSigMessage(collectiveSig [64]byte, bitmap []byte) []byte {
	message := consensus2.Message{}

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	//// Payload
	buffer := bytes.NewBuffer([]byte{})

	// 64 byte collective signature
	buffer.Write(collectiveSig[:])

	// N byte bitmap
	buffer.Write(bitmap)

	message.Payload = buffer.Bytes()
	//// END Payload

	marshaledMessage, err := message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Challenge message", "error", err)
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = message.XXX_Marshal([]byte{}, true)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Challenge message", "error", err)
	}
	consensus.Log.Info("New CollectiveSig", "NodeID", consensus.nodeID, "bitmap", consensus.bitmap)
	return proto_consensus.ConstructConsensusMessage(proto_consensus.CollectiveSig, marshaledMessage)
}

func getAggregatedCommit(commitments []kyber.Point) (commitment kyber.Point, bytes []byte) {
	aggCommitment := crypto.AggregateCommitmentsOnly(crypto.Ed25519Curve, commitments)
	bytes, err := aggCommitment.MarshalBinary()
	if err != nil {
		panic("Failed to deserialize the aggregated commitment")
	}
	return aggCommitment, append(bytes[:], byte(0))
}

func getAggregatedKey(bitmap *crypto.Mask) []byte {
	bytes, err := bitmap.AggregatePublic.MarshalBinary()
	if err != nil {
		panic("Failed to deserialize the aggregated key")
	}
	return append(bytes[:], byte(0))
}

func getChallenge(aggCommitment, aggKey kyber.Point, message []byte) kyber.Scalar {
	challenge, err := crypto.Challenge(crypto.Ed25519Curve, aggCommitment, aggKey, message)
	if err != nil {
		log.Error("Failed to generate challenge")
	}
	return challenge
}
