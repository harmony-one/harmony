package consensus

import (
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

// construct the view change message
func (consensus *Consensus) constructViewChangeMessage() []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_VIEWCHANGE,
		Request: &msg_pb.Message_Viewchange{
			Viewchange: &msg_pb.ViewChangeRequest{},
		},
	}

	vcMsg := message.GetViewchange()
	vcMsg.ConsensusId = consensus.mode.GetConsensusID()
	vcMsg.SeqNum = consensus.seqNum
	// sender address
	vcMsg.SenderPubkey = consensus.PubKey.Serialize()

	// next leader key already updated
	vcMsg.LeaderPubkey = consensus.LeaderPubKey.Serialize()

	preparedMsgs := consensus.pbftLog.GetMessagesByTypeSeqHash(msg_pb.MessageType_PREPARED, consensus.seqNum, consensus.blockHash)

	if len(preparedMsgs) > 1 {
		utils.GetLogInstance().Warn("constructViewChangeMessage got more than 1 prepared message", "seqNum", consensus.seqNum, "consensusID", consensus.consensusID)
	}

	var msgToSign []byte
	if len(preparedMsgs) == 0 {
		msgToSign = NIL // m2 type message
		vcMsg.Payload = []byte{}
	} else {
		// m1 type message
		msgToSign = append(preparedMsgs[0].BlockHash[:], preparedMsgs[0].Payload...)
		vcMsg.Payload = append(msgToSign[:0:0], msgToSign...)
	}

	sign := consensus.priKey.SignHash(msgToSign)
	if sign != nil {
		vcMsg.ViewchangeSig = sign.Serialize()
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.GetLogInstance().Error("constructViewChangeMessage failed to sign and marshal the viewchange message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// new leader construct newview message
func (consensus *Consensus) constructNewViewMessage() []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_NEWVIEW,
		Request: &msg_pb.Message_Viewchange{
			Viewchange: &msg_pb.ViewChangeRequest{},
		},
	}

	vcMsg := message.GetViewchange()
	vcMsg.ConsensusId = consensus.mode.GetConsensusID()
	vcMsg.SeqNum = consensus.seqNum
	// sender address
	vcMsg.SenderPubkey = consensus.PubKey.Serialize()

	// m1 type message
	sig1arr := consensus.GetBhpSigsArray()
	if len(sig1arr) > 0 {
		m1Sig := bls_cosi.AggregateSig(sig1arr)
		vcMsg.M1Aggsigs = m1Sig.Serialize()
		vcMsg.M1Bitmap = consensus.bhpBitmap.Bitmap
		vcMsg.Payload = consensus.m1Payload
	}

	sig2arr := consensus.GetNilSigsArray()
	if len(sig2arr) > 0 {
		m2Sig := bls_cosi.AggregateSig(sig2arr)
		vcMsg.M2Aggsigs = m2Sig.Serialize()
		vcMsg.M2Bitmap = consensus.nilBitmap.Bitmap
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.GetLogInstance().Error("constructNewViewMessage failed to sign and marshal the new view message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}
