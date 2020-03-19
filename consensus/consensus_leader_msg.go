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

// Constructs the announce message
func (consensus *Consensus) constructAnnounceMessage(priKey *bls.SecretKey) []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_ANNOUNCE,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}
	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg, priKey.GetPublicKey())
	consensusMsg.Payload = consensus.blockHeader

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message, priKey)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the Announce message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// Construct the prepared message, returning prepared message in bytes.
func (consensus *Consensus) constructPreparedMessage(priKey *bls.SecretKey) ([]byte, *bls.Sign) {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_PREPARED,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg, consensus.LeaderPubKey)
	// add block content in prepared message for slow validators to catchup
	consensusMsg.Block = consensus.block

	//// Payload
	buffer := bytes.NewBuffer([]byte{})

	// 96 bytes aggregated signature
	aggSig := bls_cosi.AggregateSig(consensus.Decider.ReadAllSignatures(quorum.Prepare))
	buffer.Write(aggSig.Serialize())

	// Bitmap
	buffer.Write(consensus.prepareBitmap.Bitmap)

	consensusMsg.Payload = buffer.Bytes()
	//// END Payload

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message, priKey)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the Prepared message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage), aggSig
}

// Construct the committed message, returning committed message in bytes.
func (consensus *Consensus) constructCommittedMessage(priKey *bls.SecretKey, pubKey *bls.PublicKey) ([]byte, *bls.Sign) {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_CONSENSUS,
		Type:        msg_pb.MessageType_COMMITTED,
		Request: &msg_pb.Message_Consensus{
			Consensus: &msg_pb.ConsensusRequest{},
		},
	}

	consensusMsg := message.GetConsensus()
	consensus.populateMessageFields(consensusMsg, pubKey)

	//// Payload
	buffer := bytes.NewBuffer([]byte{})

	// 96 bytes aggregated signature
	aggSig := bls_cosi.AggregateSig(consensus.Decider.ReadAllSignatures(quorum.Commit))
	buffer.Write(aggSig.Serialize())

	// Bitmap
	buffer.Write(consensus.commitBitmap.Bitmap)

	consensusMsg.Payload = buffer.Bytes()
	//// END Payload

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(message, priKey)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the Committed message")
	}
	return proto.ConstructConsensusMessage(marshaledMessage), aggSig
}
