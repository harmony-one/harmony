package consensus

import (
	protobuf "github.com/golang/protobuf/proto"
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/internal/utils"
)

// Construct the prepare message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructPrepareMessage() []byte {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_PREPARE

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	// 48 byte of bls signature
	sign := consensus.priKey.SignHash(message.BlockHash)
	if sign != nil {
		message.Payload = sign.Serialize()
	}

	marshaledMessage, err := protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to marshal Prepare message", "error", err)
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to marshal Prepare message", "error", err)
	}

	return proto.ConstructConsensusMessage(marshaledMessage)
}

// Construct the commit message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructCommitMessage() []byte {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_COMMIT

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	// 48 byte of bls signature
	// TODO: sign on the prepared message hash, rather than the block hash
	sign := consensus.priKey.SignHash(message.BlockHash)
	if sign != nil {
		message.Payload = sign.Serialize()
	}

	marshaledMessage, err := protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to marshal Commit message", "error", err)
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to marshal Commit message", "error", err)
	}

	return proto.ConstructConsensusMessage(marshaledMessage)
}
