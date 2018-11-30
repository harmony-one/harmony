package consensus

import (
	"bytes"
	"encoding/binary"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/log"
	proto_consensus "github.com/harmony-one/harmony/proto/consensus"
)

// Constructs the announce message
func (consensus *Consensus) constructAnnounceMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusID)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeID)
	buffer.Write(twoBytes)

	// n byte of block header
	// TODO(rj,minhdoan): Better to write the size of blockHeader
	buffer.Write(consensus.blockHeader)

	// 64 byte of signature on previous data
	signature := consensus.signMessage(buffer.Bytes())
	buffer.Write(signature)

	consensus.Log.Info("New Announce", "NodeID", consensus.nodeID, "bitmap", consensus.bitmap)
	return proto_consensus.ConstructConsensusMessage(proto_consensus.Announce, buffer.Bytes())
}

// Construct the challenge message, returning challenge message in bytes, challenge scalar and aggregated commmitment point.
func (consensus *Consensus) constructChallengeMessage(msgTypeToSend proto_consensus.MessageType) ([]byte, kyber.Scalar, kyber.Point) {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusID)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeID)
	buffer.Write(twoBytes)

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
	challengeScalar := getChallenge(aggCommitment, bitmap.AggregatePublic, buffer.Bytes()[:36])
	bytes, err := challengeScalar.MarshalBinary()
	if err != nil {
		log.Error("Failed to serialize challenge")
	}
	buffer.Write(bytes)

	// 64 byte of signature on previous data
	signature := consensus.signMessage(buffer.Bytes())
	buffer.Write(signature)

	consensus.Log.Info("New Challenge", "NodeID", consensus.nodeID, "bitmap", consensus.bitmap)
	return proto_consensus.ConstructConsensusMessage(msgTypeToSend, buffer.Bytes()), challengeScalar, aggCommitment
}

// Construct the collective signature message
func (consensus *Consensus) constructCollectiveSigMessage(collectiveSig [64]byte, bitmap []byte) []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusID)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeID)
	buffer.Write(twoBytes)

	// 64 byte collective signature
	buffer.Write(collectiveSig[:])

	// N byte bitmap
	buffer.Write(bitmap)

	// 64 byte of signature on previous data
	signature := consensus.signMessage(buffer.Bytes())
	buffer.Write(signature)

	consensus.Log.Info("New CollectiveSig", "NodeID", consensus.nodeID, "bitmap", consensus.bitmap)
	return proto_consensus.ConstructConsensusMessage(proto_consensus.CollectiveSig, buffer.Bytes())
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
