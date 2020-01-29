package consensus

import (
	"bytes"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

// NetworkMessage is a message intended to be
// created only by the leader for distribution to
// all the other quorum members.
type NetworkMessage struct {
	Phase                      msg_pb.MessageType
	Bytes                      []byte
	FBFTMsg                    *FBFTMessage
	OptionalAggregateSignature *bls.Sign
}

func (consensus *Consensus) construct(p msg_pb.MessageType) *NetworkMessage {
	// Assume a default of prepare
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_PREPARE,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	// If not prepared then set it to be whatever it is
	if p != msg_pb.MessageType_PREPARE {
		message.Type = p
	}

	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg)

	// Do the signing, 96 byte of bls signature
	switch p {
	case msg_pb.MessageType_PREPARED:
		consensusMsg.Block = consensus.block
		// Payload
		buffer := bytes.Buffer{}
		// 96 bytes aggregated signature
		aggSig := bls_cosi.AggregateSig(consensus.Decider.ReadAllSignatures(quorum.Prepare))
		// TODO(Edgar) Finish refactoring with this API
		// aggSig := consensus.Decider.AggregateVotes(quorum.Announce)
		buffer.Write(aggSig.Serialize())
		// Bitmap
		buffer.Write(consensus.prepareBitmap.Bitmap)
		consensusMsg.Payload = buffer.Bytes()

	case msg_pb.MessageType_PREPARE:
		sign := consensus.priKey.SignHash(consensusMsg.BlockHash)
		if sign != nil {
			consensusMsg.Payload = sign.Serialize()
		}

	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the Prepare message")
	}

	FBFTMsg, err2 := ParseFBFTMessage(message)

	if err2 != nil {
		utils.Logger().Error().Err(err).Msg("failed to deal with the fbft message")
	}

	return &NetworkMessage{
		Phase:                      p,
		Bytes:                      proto.ConstructConsensusMessage(marshaledMessage),
		FBFTMsg:                    FBFTMsg,
		OptionalAggregateSignature: nil,
	}

}

// Constructs the announce message
func (consensus *Consensus) constructAnnounceMessage() []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_ANNOUNCE,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}
	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg)
	consensusMsg.Payload = consensus.blockHeader

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the Announce message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// Construct the committed message, returning committed message in bytes.
func (consensus *Consensus) constructCommittedMessage() ([]byte, *bls.Sign) {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_COMMITTED,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg)

	//// Payload
	buffer := bytes.Buffer{}

	// 96 bytes aggregated signature
	aggSig := bls_cosi.AggregateSig(consensus.Decider.ReadAllSignatures(quorum.Commit))
	buffer.Write(aggSig.Serialize())

	// Bitmap
	buffer.Write(consensus.commitBitmap.Bitmap)

	consensusMsg.Payload = buffer.Bytes()
	// END Payload

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the Committed message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage), aggSig
}
