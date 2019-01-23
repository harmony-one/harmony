package consensus

import (
	"github.com/dedis/kyber"
	protobuf "github.com/golang/protobuf/proto"
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/crypto"
)

// Construct the commit message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructCommitMessage(msgType consensus_proto.MessageType) (secret kyber.Scalar, commitMsg []byte) {
	message := consensus_proto.Message{}
	message.Type = msgType

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	// 32 byte of commit
	secret, commitment := crypto.Commit(crypto.Ed25519Curve)
	bytes, err := commitment.MarshalBinary()
	if err != nil {
		consensus.Log.Debug("Failed to marshal commit", "error", err)
	}
	message.Payload = bytes

	marshaledMessage, err := protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Announce message", "error", err)
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Announce message", "error", err)
	}

	return secret, proto.ConstructConsensusMessage(marshaledMessage)
}

// Construct the response message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructResponseMessage(msgType consensus_proto.MessageType, response kyber.Scalar) []byte {
	message := consensus_proto.Message{}
	message.Type = msgType

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	bytes, err := response.MarshalBinary()
	if err != nil {
		consensus.Log.Debug("Failed to marshal response", "error", err)
	}
	message.Payload = bytes

	marshaledMessage, err := protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Announce message", "error", err)
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Announce message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}
