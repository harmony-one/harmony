package consensus

import (
	"bytes"

	"github.com/harmony-one/bls/ffi/go/bls"
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
	OptionalAggregateSignature *bls.Sign
}

func (consensus *Consensus) construct(p msg_pb.MessageType) (*NetworkMessage, error) {
	// Assume a default of prepare
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        p,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg)

	var aggSig *bls.Sign

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
		sign := consensus.priKey.SignHash(consensusMsg.BlockHash)
		if sign != nil {
			consensusMsg.Payload = sign.Serialize()
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
		consensusMsg.Payload = consensus.blockHeader
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the Prepare message")
		return nil, err
	}

	FBFTMsg, err2 := ParseFBFTMessage(message)

	if err2 != nil {
		utils.Logger().Error().Err(err).Msg("failed to deal with the fbft message")
		return nil, err
	}

	return &NetworkMessage{
		Phase:                      p,
		Bytes:                      proto.ConstructConsensusMessage(marshaledMessage),
		FBFTMsg:                    FBFTMsg,
		OptionalAggregateSignature: aggSig,
	}, nil

}
