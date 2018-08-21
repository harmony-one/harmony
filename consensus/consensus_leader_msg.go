package consensus

import (
	"bytes"
	"encoding/binary"
	"github.com/dedis/kyber"
	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/log"
	proto_consensus "github.com/simple-rules/harmony-benchmark/proto/consensus"
)

// Constructs the announce message
func (consensus *Consensus) constructAnnounceMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// n byte of block header
	buffer.Write(consensus.blockHeader)

	// 64 byte of signature on previous data
	signature := consensus.signMessage(buffer.Bytes())
	buffer.Write(signature)

	return proto_consensus.ConstructConsensusMessage(proto_consensus.ANNOUNCE, buffer.Bytes())
}

// Construct the challenge message
func (consensus *Consensus) constructChallengeMessage() []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// 33 byte aggregated commit
	commitments := make([]kyber.Point, 0)
	for _, val := range consensus.commitments {
		commitments = append(commitments, val)
	}
	aggCommitment, aggCommitmentBytes := getAggregatedCommit(commitments)
	buffer.Write(aggCommitmentBytes)

	// 33 byte aggregated key
	buffer.Write(getAggregatedKey(consensus.bitmap))

	// 32 byte challenge
	buffer.Write(getChallenge(aggCommitment, consensus.bitmap.AggregatePublic, buffer.Bytes()[:36])) // message contains consensus id and block hash for now.
	consensus.aggregatedCommitment = aggCommitment

	// 64 byte of signature on previous data
	signature := consensus.signMessage(buffer.Bytes())
	buffer.Write(signature)

	return proto_consensus.ConstructConsensusMessage(proto_consensus.CHALLENGE, buffer.Bytes())
}

// Construct the collective signature message
func (consensus *Consensus) constructCollectiveSigMessage(collectiveSig [64]byte, bitmap []byte) []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte leader id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// 64 byte collective signature
	buffer.Write(collectiveSig[:])

	// N byte bitmap
	buffer.Write(bitmap)

	// 64 byte of signature on previous data
	signature := consensus.signMessage(buffer.Bytes())
	buffer.Write(signature)

	return proto_consensus.ConstructConsensusMessage(proto_consensus.COLLECTIVE_SIG, buffer.Bytes())
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

func getChallenge(aggCommitment, aggKey kyber.Point, message []byte) []byte {
	challenge, err := crypto.Challenge(crypto.Ed25519Curve, aggCommitment, aggKey, message)
	if err != nil {
		log.Error("Failed to generate challenge")
	}
	bytes, err := challenge.MarshalBinary()
	if err != nil {
		log.Error("Failed to serialize challenge")
	}
	return bytes
}
