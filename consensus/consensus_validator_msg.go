package consensus

import (
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
)

// Construct the prepare message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructPrepareMessage() []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_PREPARE,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg)

	// 96 byte of bls signature
	sign := consensus.priKey.SignHash(consensusMsg.BlockHash)
	if sign != nil {
		consensusMsg.Payload = sign.Serialize()
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the Prepare message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// Construct the commit message which contains the signature on the multi-sig of prepare phase.
func (consensus *Consensus) constructCommitMessage(commitPayload []byte) []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_COMMIT,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg)

	// 96 byte of bls signature
	sign := consensus.priKey.SignHash(commitPayload)
	if sign != nil {
		consensusMsg.Payload = sign.Serialize()
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the Commit message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}
