package consensus

import (
	"bytes"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

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

// Construct the prepared message, returning prepared message in bytes.
func (consensus *Consensus) constructPreparedMessage() ([]byte, *bls.Sign) {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_PREPARED,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg)
	// add block content in prepared message for slow validators to catchup
	consensusMsg.Block = consensus.block

	//// Payload
	buffer := bytes.NewBuffer([]byte{})

	// 96 bytes aggregated signature
	aggSig := bls_cosi.AggregateSig(consensus.GetPrepareSigsArray())
	buffer.Write(aggSig.Serialize())

	// Bitmap
	buffer.Write(consensus.prepareBitmap.Bitmap)

	consensusMsg.Payload = buffer.Bytes()
	//// END Payload

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the Prepared message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage), aggSig
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
	buffer := bytes.NewBuffer([]byte{})

	// 96 bytes aggregated signature
	aggSig := bls_cosi.AggregateSig(consensus.GetCommitSigsArray())
	buffer.Write(aggSig.Serialize())

	// Bitmap
	buffer.Write(consensus.commitBitmap.Bitmap)

	consensusMsg.Payload = buffer.Bytes()
	//// END Payload

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the Committed message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage), aggSig
}
