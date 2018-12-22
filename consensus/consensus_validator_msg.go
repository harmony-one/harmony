package consensus

import (
	"github.com/dedis/kyber"
	consensus2 "github.com/harmony-one/harmony/consensus/proto"
	"github.com/harmony-one/harmony/crypto"
	proto_consensus "github.com/harmony-one/harmony/proto/consensus"
)

// Construct the commit message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructCommitMessage(msgType proto_consensus.MessageType) (secret kyber.Scalar, commitMsg []byte) {
	message := consensus2.Message{}

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	// 32 byte of commit (TODO: figure out why it's different than Zilliqa's ECPoint which takes 33 bytes: https://crypto.stackexchange.com/questions/51703/how-to-convert-from-curve25519-33-byte-to-32-byte-representation)
	secret, commitment := crypto.Commit(crypto.Ed25519Curve)
	bytes, err := commitment.MarshalBinary()
	if err != nil {
		consensus.Log.Debug("Failed to marshal commit", "error", err)
	}
	message.Payload = bytes

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

	return secret, proto_consensus.ConstructConsensusMessage(msgType, marshaledMessage)
}

// Construct the response message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructResponseMessage(msgType proto_consensus.MessageType, response kyber.Scalar) []byte {
	message := consensus2.Message{}

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
	return proto_consensus.ConstructConsensusMessage(msgType, marshaledMessage)
}
