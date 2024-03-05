package consensus

import (
	"bytes"
	"errors"

	protobuf "github.com/golang/protobuf/proto"

	"github.com/harmony-one/harmony/crypto/bls"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/internal/utils"
)

// NetworkMessage is a message intended to be
// created only for distribution to
// all the other quorum members.
type NetworkMessage struct {
	MessageType                msg_pb.MessageType
	Bytes                      []byte
	FBFTMsg                    *FBFTMessage
	OptionalAggregateSignature *bls_core.Sign
}

// Populates the common basic fields for all consensus message.
func (consensus *Consensus) populateMessageFields(
	request *msg_pb.ConsensusRequest, blockHash []byte,
) *msg_pb.ConsensusRequest {
	request.ViewId = consensus.getCurBlockViewID()
	request.BlockNum = consensus.getBlockNum()
	request.ShardId = consensus.ShardID
	// 32 byte block hash
	request.BlockHash = blockHash
	return request
}

// Populates the common basic fields for the consensus message and senders bitmap.
func (consensus *Consensus) populateMessageFieldsAndSendersBitmap(
	request *msg_pb.ConsensusRequest, blockHash []byte, bitmap []byte,
) *msg_pb.ConsensusRequest {
	consensus.populateMessageFields(request, blockHash)
	// sender address
	request.SenderPubkeyBitmap = bitmap
	return request
}

// Populates the common basic fields for the consensus message and single sender.
func (consensus *Consensus) populateMessageFieldsAndSender(
	request *msg_pb.ConsensusRequest, blockHash []byte, pubKey bls.SerializedPublicKey,
) *msg_pb.ConsensusRequest {
	consensus.populateMessageFields(request, blockHash)
	// sender address
	request.SenderPubkey = pubKey[:]
	return request
}

// construct is the single creation point of messages intended for the wire.
func (consensus *Consensus) construct(
	p msg_pb.MessageType, payloadForSign []byte, priKeys []*bls.PrivateKeyWrapper,
) (*NetworkMessage, error) {
	if len(priKeys) == 0 {
		return nil, errors.New("no elected bls keys provided")
	}
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        p,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}
	var (
		consensusMsg *msg_pb.ConsensusRequest
		aggSig       *bls_core.Sign
	)

	if len(priKeys) == 1 {
		consensusMsg = consensus.populateMessageFieldsAndSender(
			message.GetConsensus(), consensus.blockHash[:], priKeys[0].Pub.Bytes,
		)
	} else {
		// TODO: use a persistent bitmap to report bitmap
		mask := bls.NewMask(consensus.decider.Participants())
		for _, key := range priKeys {
			mask.SetKey(key.Pub.Bytes, true)
		}
		consensusMsg = consensus.populateMessageFieldsAndSendersBitmap(
			message.GetConsensus(), consensus.blockHash[:], mask.Bitmap,
		)
	}

	// Do the signing, 96 byte of bls signature
	needMsgSig := true
	switch p {
	case msg_pb.MessageType_ANNOUNCE:
		consensusMsg.Block = consensus.block
		consensusMsg.Payload = consensus.blockHash[:]
	case msg_pb.MessageType_PREPARE:
		needMsgSig = false
		sig := bls_core.Sign{}
		for _, priKey := range priKeys {
			if s := priKey.Pri.SignHash(consensusMsg.BlockHash); s != nil {
				sig.Add(s)
			}
		}
		consensusMsg.Payload = sig.Serialize()
	case msg_pb.MessageType_COMMIT:
		needMsgSig = false
		sig := bls_core.Sign{}
		for _, priKey := range priKeys {
			if s := priKey.Pri.SignHash(payloadForSign); s != nil {
				sig.Add(s)
			}
		}
		consensusMsg.Payload = sig.Serialize()
	case msg_pb.MessageType_PREPARED:
		consensusMsg.Block = consensus.block
		consensusMsg.Payload = consensus.constructQuorumSigAndBitmap(quorum.Prepare)
	case msg_pb.MessageType_COMMITTED:
		consensusMsg.Payload = consensus.constructQuorumSigAndBitmap(quorum.Commit)
	}

	var marshaledMessage []byte
	var err error
	if needMsgSig {
		// The message that needs signing only needs to be signed with a single key
		marshaledMessage, err = consensus.signAndMarshalConsensusMessage(message, priKeys[0].Pri)
	} else {
		// Skip message (potentially multi-sig) signing for validator consensus messages (prepare and commit)
		// as signature is already signed on the block data.
		marshaledMessage, err = protobuf.Marshal(message)
	}
	if err != nil {
		utils.Logger().Error().Err(err).
			Str("phase", p.String()).
			Msg("Failed to sign and marshal consensus message")
		return nil, err
	}

	FBFTMsg, err2 := consensus.parseFBFTMessage(message)

	if err2 != nil {
		utils.Logger().Error().Err(err).
			Str("phase", p.String()).
			Msg("failed to deal with the FBFT message")
		return nil, err
	}

	return &NetworkMessage{
		MessageType:                p,
		Bytes:                      proto.ConstructConsensusMessage(marshaledMessage),
		FBFTMsg:                    FBFTMsg,
		OptionalAggregateSignature: aggSig,
	}, nil
}

// constructQuorumSigAndBitmap constructs the aggregated sig and bitmap as
// a byte slice in format of: [[aggregated sig], [sig bitmap]]
func (consensus *Consensus) constructQuorumSigAndBitmap(p quorum.Phase) []byte {
	buffer := bytes.Buffer{}
	// 96 bytes aggregated signature
	aggSig := consensus.decider.AggregateVotes(p)
	buffer.Write(aggSig.Serialize())
	// Bitmap
	if p == quorum.Prepare {
		buffer.Write(consensus.prepareBitmap.Bitmap)
	} else if p == quorum.Commit {
		buffer.Write(consensus.commitBitmap.Bitmap)
	} else {
		utils.Logger().Error().
			Str("phase", p.String()).
			Msg("[constructQuorumSigAndBitmap] Invalid phase is supplied.")
		return []byte{}
	}
	return buffer.Bytes()
}
