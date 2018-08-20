package consensus

import (
	"bytes"
	"encoding/binary"
	"github.com/dedis/kyber"
	"github.com/simple-rules/harmony-benchmark/crypto"
	proto_consensus "github.com/simple-rules/harmony-benchmark/proto/consensus"
)

// Construct the commit message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructCommitMessage(msgType proto_consensus.MessageType) (secret kyber.Scalar, commitMsg []byte) {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:])

	// 2 byte validator id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// 32 byte of commit (TODO: figure out why it's different than Zilliqa's ECPoint which takes 33 bytes: https://crypto.stackexchange.com/questions/51703/how-to-convert-from-curve25519-33-byte-to-32-byte-representation)
	secret, commitment := crypto.Commit(crypto.Ed25519Curve)
	commitment.MarshalTo(buffer)

	// 64 byte of signature on previous data
	signature := consensus.signMessage(buffer.Bytes())
	buffer.Write(signature)

	return secret, proto_consensus.ConstructConsensusMessage(msgType, buffer.Bytes())
}

// Construct the response message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructResponseMessage(response kyber.Scalar) []byte {
	buffer := bytes.NewBuffer([]byte{})

	// 4 byte consensus id
	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
	buffer.Write(fourBytes)

	// 32 byte block hash
	buffer.Write(consensus.blockHash[:32])

	// 2 byte validator id
	twoBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
	buffer.Write(twoBytes)

	// 32 byte of response
	response.MarshalTo(buffer)

	// 64 byte of signature on previous data
	signature := consensus.signMessage(buffer.Bytes())
	buffer.Write(signature)

	return proto_consensus.ConstructConsensusMessage(proto_consensus.RESPONSE, buffer.Bytes())
}
