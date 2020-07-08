package consensus

import (
	"bytes"

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
	Phase                      msg_pb.MessageType
	Bytes                      []byte
	FBFTMsg                    *FBFTMessage
	OptionalAggregateSignature *bls_core.Sign
}

// Populates the common basic fields for all consensus message.
func (consensus *Consensus) populateMessageFields(
	request *msg_pb.ConsensusRequest, blockHash []byte, pubKey bls.SerializedPublicKey,
) *msg_pb.ConsensusRequest {
	request.ViewId = consensus.viewID
	request.BlockNum = consensus.blockNum
	request.ShardId = consensus.ShardID
	// 32 byte block hash
	request.BlockHash = blockHash
	// sender address
	request.SenderPubkey = pubKey[:]
	return request
}

// construct is the single creation point of messages intended for the wire.
func (consensus *Consensus) construct(
	p msg_pb.MessageType, payloadForSign []byte, priKey *bls.PrivateKeyWrapper,
) (*NetworkMessage, error) {
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

	consensusMsg = consensus.populateMessageFields(
		message.GetConsensus(), consensus.blockHash[:], priKey.Pub.Bytes,
	)

	// Do the signing, 96 byte of bls signature
	switch p {
	case msg_pb.MessageType_PREPARED:
		consensusMsg.Block = consensus.block
		// Payload
		buffer := bytes.Buffer{}
		// 96 bytes aggregated signature
		aggSig = consensus.Decider.AggregateVotes(quorum.Prepare)
		buffer.Write(aggSig.Serialize())
		// Bitmap
		buffer.Write(consensus.prepareBitmap.Bitmap)
		consensusMsg.Payload = buffer.Bytes()
	case msg_pb.MessageType_PREPARE:
		if s := priKey.Pri.SignHash(consensusMsg.BlockHash); s != nil {
			consensusMsg.Payload = s.Serialize()
		}
	case msg_pb.MessageType_COMMIT:
		if s := priKey.Pri.SignHash(payloadForSign); s != nil {
			consensusMsg.Payload = s.Serialize()
		}
	case msg_pb.MessageType_COMMITTED:
		buffer := bytes.Buffer{}
		// 96 bytes aggregated signature
		aggSig = consensus.Decider.AggregateVotes(quorum.Commit)
		buffer.Write(aggSig.Serialize())
		// Bitmap
		buffer.Write(consensus.commitBitmap.Bitmap)
		consensusMsg.Payload = buffer.Bytes()
	case msg_pb.MessageType_ANNOUNCE:
		consensusMsg.Payload = consensus.blockHash[:]
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message, priKey.Pri)
	if err != nil {
		utils.Logger().Error().Err(err).
			Str("phase", p.String()).
			Msg("Failed to sign and marshal consensus message")
		return nil, err
	}

	FBFTMsg, err2 := ParseFBFTMessage(message)

	if err2 != nil {
		utils.Logger().Error().Err(err).
			Str("phase", p.String()).
			Msg("failed to deal with the FBFT message")
		return nil, err
	}

	return &NetworkMessage{
		Phase:                      p,
		Bytes:                      proto.ConstructConsensusMessage(marshaledMessage),
		FBFTMsg:                    FBFTMsg,
		OptionalAggregateSignature: aggSig,
	}, nil
}
