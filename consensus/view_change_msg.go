package consensus

import (
	"encoding/binary"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/rlp"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/harmony/multibls"
	"github.com/pkg/errors"
)

// construct the view change message
func (consensus *Consensus) constructViewChangeMessage(priKey *bls.PrivateKeyWrapper) []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_VIEWCHANGE,
		Request: &msg_pb.Message_Viewchange{
			Viewchange: &msg_pb.ViewChangeRequest{
				ViewId:       consensus.getViewChangingID(),
				BlockNum:     consensus.getBlockNum(),
				ShardId:      consensus.ShardID,
				SenderPubkey: priKey.Pub.Bytes[:],
				LeaderPubkey: consensus.LeaderPubKey.Bytes[:],
			},
		},
	}

	preparedMsgs := consensus.FBFTLog.GetMessagesByTypeSeq(
		msg_pb.MessageType_PREPARED, consensus.getBlockNum(),
	)
	preparedMsg := consensus.FBFTLog.FindMessageByMaxViewID(preparedMsgs)

	var encodedBlock []byte
	if preparedMsg != nil {
		block := consensus.FBFTLog.GetBlockByHash(preparedMsg.BlockHash)
		consensus.getLogger().Info().
			Interface("Block", block).
			Interface("preparedMsg", preparedMsg).
			Msg("[constructViewChangeMessage] found prepared msg")
		if block != nil {
			if err := consensus.verifyBlock(block); err == nil {
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

	consensus.getLogger().Info().
		Hex("m1Payload", vcMsg.Payload).
		Str("NextLeader", consensus.LeaderPubKey.Bytes.Hex()).
		Str("SenderPubKey", priKey.Pub.Bytes.Hex()).
		Msg("[constructViewChangeMessage]")

	sign := priKey.Pri.SignHash(msgToSign)
	if sign != nil {
		vcMsg.ViewchangeSig = sign.Serialize()
	} else {
		consensus.getLogger().Error().Msg("unable to serialize m1/m2 view change message signature")
	}

	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, consensus.getViewChangingID())
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
				ViewId:       viewID,
				BlockNum:     consensus.getBlockNum(),
				ShardId:      consensus.ShardID,
				SenderPubkey: priKey.Pub.Bytes[:],
			},
		},
	}

	vcMsg := message.GetViewchange()
	vcMsg.Payload, vcMsg.PreparedBlock = consensus.vc.GetPreparedBlock(consensus.FBFTLog)
	vcMsg.M2Aggsigs, vcMsg.M2Bitmap = consensus.vc.GetM2Bitmap(viewID)
	vcMsg.M3Aggsigs, vcMsg.M3Bitmap = consensus.vc.GetM3Bitmap(viewID)
	if vcMsg.M3Bitmap == nil || vcMsg.M3Aggsigs == nil {
		return nil
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message, priKey.Pri)
	if err != nil {
		consensus.getLogger().Err(err).
			Msg("[constructNewViewMessage] failed to sign and marshal the new view message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

var (
	errNilMessage           = errors.New("Nil protobuf message")
	errIncorrectMessageType = errors.New("Incorrect message type")
)

// ParseViewChangeMessage parses view change message into FBFTMessage structure
func ParseViewChangeMessage(msg *msg_pb.Message) (*FBFTMessage, error) {
	if msg == nil {
		return nil, errNilMessage
	}
	vcMsg := msg.GetViewchange()
	FBFTMsg := FBFTMessage{
		BlockNum:    vcMsg.BlockNum,
		ViewID:      vcMsg.ViewId,
		MessageType: msg.GetType(),
		Payload:     make([]byte, len(vcMsg.Payload)),
		Block:       make([]byte, len(vcMsg.PreparedBlock)),
	}

	if FBFTMsg.MessageType != msg_pb.MessageType_VIEWCHANGE {
		return nil, errIncorrectMessageType
	}

	copy(FBFTMsg.Block[:], vcMsg.PreparedBlock[:])
	copy(FBFTMsg.Payload[:], vcMsg.Payload[:])

	pubKey, err := bls_cosi.BytesToBLSPublicKey(vcMsg.SenderPubkey)
	if err != nil {
		return nil, err
	}
	leaderKey, err := bls_cosi.BytesToBLSPublicKey(vcMsg.LeaderPubkey)
	if err != nil {
		return nil, err
	}

	vcSig := bls_core.Sign{}
	err = vcSig.Deserialize(vcMsg.ViewchangeSig)
	if err != nil {
		return nil, err
	}
	FBFTMsg.ViewchangeSig = &vcSig

	vcSig1 := bls_core.Sign{}
	err = vcSig1.Deserialize(vcMsg.ViewidSig)
	if err != nil {
		return nil, err
	}
	FBFTMsg.ViewidSig = &vcSig1

	FBFTMsg.SenderPubkeys = []*bls.PublicKeyWrapper{{Object: pubKey}}
	copy(FBFTMsg.SenderPubkeys[0].Bytes[:], vcMsg.SenderPubkey[:])
	FBFTMsg.LeaderPubkey = &bls.PublicKeyWrapper{Object: leaderKey}
	copy(FBFTMsg.LeaderPubkey.Bytes[:], vcMsg.LeaderPubkey[:])

	return &FBFTMsg, nil
}

// ParseNewViewMessage parses new view message into FBFTMessage structure
func ParseNewViewMessage(msg *msg_pb.Message, members multibls.PublicKeys) (*FBFTMessage, error) {
	if msg == nil {
		return nil, errNilMessage
	}
	vcMsg := msg.GetViewchange()
	FBFTMsg := FBFTMessage{
		BlockNum:    vcMsg.BlockNum,
		ViewID:      vcMsg.ViewId,
		MessageType: msg.GetType(),
		Payload:     make([]byte, len(vcMsg.Payload)),
		Block:       make([]byte, len(vcMsg.PreparedBlock)),
	}

	if FBFTMsg.MessageType != msg_pb.MessageType_NEWVIEW {
		return nil, errIncorrectMessageType
	}

	copy(FBFTMsg.Payload[:], vcMsg.Payload[:])
	copy(FBFTMsg.Block[:], vcMsg.PreparedBlock[:])

	pubKey, err := bls_cosi.BytesToBLSPublicKey(vcMsg.SenderPubkey)
	if err != nil {
		return nil, err
	}

	FBFTMsg.SenderPubkeys = []*bls.PublicKeyWrapper{{Object: pubKey}}
	copy(FBFTMsg.SenderPubkeys[0].Bytes[:], vcMsg.SenderPubkey[:])

	if len(vcMsg.M3Aggsigs) > 0 {
		m3Sig := bls_core.Sign{}
		err = m3Sig.Deserialize(vcMsg.M3Aggsigs)
		if err != nil {
			return nil, err
		}
		m3mask, err := bls_cosi.NewMask(members, nil)
		if err != nil {
			return nil, err
		}
		m3mask.SetMask(vcMsg.M3Bitmap)
		FBFTMsg.M3AggSig = &m3Sig
		FBFTMsg.M3Bitmap = m3mask
	}

	if len(vcMsg.M2Aggsigs) > 0 {
		m2Sig := bls_core.Sign{}
		err = m2Sig.Deserialize(vcMsg.M2Aggsigs)
		if err != nil {
			return nil, err
		}
		m2mask, err := bls_cosi.NewMask(members, nil)
		if err != nil {
			return nil, err
		}
		m2mask.SetMask(vcMsg.M2Bitmap)
		FBFTMsg.M2AggSig = &m2Sig
		FBFTMsg.M2Bitmap = m2mask
	}

	return &FBFTMsg, nil
}
