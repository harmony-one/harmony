package consensus

import (
	"encoding/binary"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
)

// construct the view change message
func (consensus *Consensus) constructViewChangeMessage(priKey *bls.PrivateKeyWrapper, nextLeaderPubKey *bls.PublicKeyWrapper) []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_VIEWCHANGE,
		Request: &msg_pb.Message_Viewchange{
			Viewchange: &msg_pb.ViewChangeRequest{
				ViewId:       consensus.GetViewChangingID(),
				BlockNum:     consensus.blockNum,
				ShardId:      consensus.ShardID,
				SenderPubkey: priKey.Pub.Bytes[:],
				LeaderPubkey: nextLeaderPubKey.Bytes[:],
			},
		},
	}

	preparedMsgs := consensus.FBFTLog.GetMessagesByTypeSeq(
		msg_pb.MessageType_PREPARED, consensus.blockNum,
	)
	preparedMsg := consensus.FBFTLog.FindMessageByMaxViewID(preparedMsgs)

	var encodedBlock []byte
	if preparedMsg != nil {
		block := consensus.FBFTLog.GetBlockByHash(preparedMsg.BlockHash)
		consensus.getLogger().Debug().
			Interface("Block", block).
			Interface("preparedMsg", preparedMsg).
			Msg("[constructViewChangeMessage] found prepared msg")
		if block != nil {
			if err := consensus.BlockVerifier(block); err == nil {
				tmpEncoded, err := rlp.EncodeToBytes(block)
				if err != nil {
					consensus.getLogger().Err(err).Msg("[constructViewChangeMessage] Failed encoding block")
				}
				encodedBlock = tmpEncoded
			} else {
				consensus.getLogger().Err(err).Msg("[constructViewChangeMessage] Failed validating prepared block")
			}
		}
	}

	vcMsg := message.GetViewchange()
	var msgToSign []byte
	if len(encodedBlock) == 0 {
		msgToSign = NIL // m2 type message
		vcMsg.Payload = []byte{}
	} else {
		// m1 type message
		msgToSign = append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
		vcMsg.Payload = append(msgToSign[:0:0], msgToSign...)
		vcMsg.PreparedBlock = encodedBlock
	}

	consensus.getLogger().Debug().
		Hex("m1Payload", vcMsg.Payload).
		Str("pubKey", consensus.GetPublicKeys().SerializeToHexStr()).
		Msg("[constructViewChangeMessage]")

	sign := priKey.Pri.SignHash(msgToSign)
	if sign != nil {
		vcMsg.ViewchangeSig = sign.Serialize()
	} else {
		consensus.getLogger().Error().Msg("unable to serialize m1/m2 view change message signature")
	}

	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, consensus.GetViewChangingID())
	sign1 := priKey.Pri.SignHash(viewIDBytes)
	if sign1 != nil {
		vcMsg.ViewidSig = sign1.Serialize()
	} else {
		consensus.getLogger().Error().Msg("unable to serialize viewID signature")
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message, priKey.Pri)
	if err != nil {
		consensus.getLogger().Err(err).
			Msg("[constructViewChangeMessage] failed to sign and marshal the viewchange message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// new leader construct newview message
func (consensus *Consensus) constructNewViewMessage(viewID uint64, priKey *bls.PrivateKeyWrapper) []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_NEWVIEW,
		Request: &msg_pb.Message_Viewchange{
			Viewchange: &msg_pb.ViewChangeRequest{
				ViewId:       consensus.GetViewChangingID(),
				BlockNum:     consensus.blockNum,
				ShardId:      consensus.ShardID,
				SenderPubkey: priKey.Pub.Bytes[:],
			},
		},
	}

	vcMsg := message.GetViewchange()
	vcMsg.Payload = consensus.m1Payload
	if len(consensus.m1Payload) != 0 {
		block := consensus.FBFTLog.GetBlockByHash(consensus.blockHash)
		if block != nil {
			encodedBlock, err := rlp.EncodeToBytes(block)
			if err != nil {
				consensus.getLogger().Err(err).Msg("[constructNewViewMessage] Failed encoding prepared block")
			}
			if len(encodedBlock) != 0 {
				vcMsg.PreparedBlock = encodedBlock
			}
		}
	}

	sig2arr := consensus.GetNilSigsArray(viewID)
	consensus.getLogger().Debug().Int("len", len(sig2arr)).Msg("[constructNewViewMessage] M2 (NIL) type signatures")
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

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message, priKey.Pri)
	if err != nil {
		consensus.getLogger().Err(err).
			Msg("[constructNewViewMessage] failed to sign and marshal the new view message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}
