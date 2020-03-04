package consensus

import (
	"encoding/binary"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

// construct the view change message
func (consensus *Consensus) constructViewChangeMessage(pubKey *bls.PublicKey, priKey *bls.SecretKey) []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_VIEWCHANGE,
		Request: &msg_pb.Message_Viewchange{
			Viewchange: &msg_pb.ViewChangeRequest{},
		},
	}

	vcMsg := message.GetViewchange()
	vcMsg.ViewId = consensus.current.ViewID()
	vcMsg.BlockNum = consensus.blockNum
	vcMsg.ShardId = consensus.ShardID
	// sender address
	vcMsg.SenderPubkey = pubKey.Serialize()

	// next leader key already updated
	vcMsg.LeaderPubkey = consensus.LeaderPubKey.Serialize()

	preparedMsgs := consensus.FBFTLog.GetMessagesByTypeSeqHash(
		msg_pb.MessageType_PREPARED, consensus.blockNum, consensus.blockHash,
	)
	preparedMsg := consensus.FBFTLog.FindMessageByMaxViewID(preparedMsgs)

	var msgToSign []byte
	if preparedMsg == nil {
		msgToSign = NIL // m2 type message
		vcMsg.Payload = []byte{}
	} else {
		// m1 type message
		msgToSign = append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
		vcMsg.Payload = append(msgToSign[:0:0], msgToSign...)
	}

	utils.Logger().Debug().
		Hex("m1Payload", vcMsg.Payload).
		Str("pubKey", consensus.PubKey.SerializeToHexStr()).
		Msg("[constructViewChangeMessage]")

	sign := priKey.SignHash(msgToSign)
	if sign != nil {
		vcMsg.ViewchangeSig = sign.Serialize()
	} else {
		utils.Logger().Error().Msg("unable to serialize m1/m2 view change message signature")
	}

	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, consensus.current.ViewID())
	sign1 := priKey.SignHash(viewIDBytes)
	if sign1 != nil {
		vcMsg.ViewidSig = sign1.Serialize()
	} else {
		utils.Logger().Error().Msg("unable to serialize viewID signature")
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message, priKey)
	if err != nil {
		utils.Logger().Error().Err(err).
			Msg("[constructViewChangeMessage] failed to sign and marshal the viewchange message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// new leader construct newview message
func (consensus *Consensus) constructNewViewMessage(viewID uint64, pubKey *bls.PublicKey, priKey *bls.SecretKey) []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_NEWVIEW,
		Request: &msg_pb.Message_Viewchange{
			Viewchange: &msg_pb.ViewChangeRequest{},
		},
	}

	vcMsg := message.GetViewchange()
	vcMsg.ViewId = consensus.current.ViewID()
	vcMsg.BlockNum = consensus.blockNum
	vcMsg.ShardId = consensus.ShardID
	// sender address
	vcMsg.SenderPubkey = pubKey.Serialize()
	vcMsg.Payload = consensus.m1Payload

	sig2arr := consensus.GetNilSigsArray(viewID)
	utils.Logger().Debug().Int("len", len(sig2arr)).Msg("[constructNewViewMessage] M2 (NIL) type signatures")
	if len(sig2arr) > 0 {
		m2Sig := bls_cosi.AggregateSig(sig2arr)
		vcMsg.M2Aggsigs = m2Sig.Serialize()
		vcMsg.M2Bitmap = consensus.nilBitmap[viewID].Bitmap
	}

	sig3arr := consensus.GetViewIDSigsArray(viewID)
	consensus.getLogger().Debug().Int("len", len(sig3arr)).Msg("[constructNewViewMessage] M3 (ViewID) type signatures")
	// even we check here for safty, m3 type signatures must >= 2f+1
	if len(sig3arr) > 0 {
		m3Sig := bls_cosi.AggregateSig(sig3arr)
		vcMsg.M3Aggsigs = m3Sig.Serialize()
		vcMsg.M3Bitmap = consensus.viewIDBitmap[viewID].Bitmap
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message, priKey)
	if err != nil {
		utils.Logger().Error().Err(err).
			Msg("[constructNewViewMessage] failed to sign and marshal the new view message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}
