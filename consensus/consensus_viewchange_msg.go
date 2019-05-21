package consensus

import (
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
)

// construct the view change message
func (consensus *Consensus) constructViewChangeMessage() []byte {
	message := &msg_pb.Message{
		ReceiverType: msg_pb.ReceiverType_LEADER,
		ServiceType:  msg_pb.ServiceType_CONSENSUS,
		Type:         msg_pb.MessageType_VIEWCHANGE,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := message.GetConsensus()
	consensusMsg.ConsensusId = consensus.consensusID
	consensusMsg.SeqNum = consensus.seqNum
	// sender address
	consensusMsg.SenderPubkey = consensus.PubKey.Serialize()

	preparedMsgs := consensus.pbftLog.GetMessagesByTypeSeqViewHash(msg_pb.MessageType_PREPARED, consensus.seqNum, consensus.consensusID, consensus.blockHash)

	if len(preparedMsgs) > 0 {
		utils.GetLogInstance().Warn("constructViewChangeMessage got more than 1 prepared message", "seqNum", consensus.seqNum, "consensusID", consensus.consensusID)
	}

	if len(preparedMsgs) == 0 {
		consensusMsg.BlockHash = []byte{}
		consensusMsg.Payload = []byte{}
	} else {
		consensusMsg.BlockHash = preparedMsgs[0].BlockHash[:]
		consensusMsg.Payload = preparedMsgs[0].Payload
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the Prepared message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// new leader construct newview message
func (consensus *Consensus) constructNewViewMessage() []byte {
	message := &msg_pb.Message{
		ReceiverType: msg_pb.ReceiverType_VALIDATOR,
		ServiceType:  msg_pb.ServiceType_CONSENSUS,
		Type:         msg_pb.MessageType_NEWVIEW,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := message.GetConsensus()
	consensusMsg.ConsensusId = consensus.consensusID
	consensusMsg.SeqNum = consensus.seqNum
	// sender address
	consensusMsg.SenderPubkey = consensus.PubKey.Serialize()

	// TODO payload should include two parts: viewchange msg with prepared sig and the one without prepared sig

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the Prepared message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)

}
