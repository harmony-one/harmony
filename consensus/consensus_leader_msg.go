package consensus

import (
	"bytes"

	"github.com/harmony-one/bls/ffi/go/bls"
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/api/proto"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constructs the announce message
func (consensus *Consensus) constructAnnounceMessage() []byte {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_ANNOUNCE

	consensus.populateBasicFields(&message)

	// n byte of block header
	message.Payload = consensus.block // TODO: send only block header in the announce phase.

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(&message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the Announce message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// Construct the prepared message, returning prepared message in bytes.
func (consensus *Consensus) constructPreparedMessage() ([]byte, *bls.Sign) {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_PREPARED

	consensus.populateBasicFields(&message)

	//// Payload
	buffer := bytes.NewBuffer([]byte{})

	// 48 bytes aggregated signature
	aggSig := bls_cosi.AggregateSig(consensus.GetPrepareSigsArray())
	buffer.Write(aggSig.Serialize())

	// Bitmap
	buffer.Write(consensus.prepareBitmap.Bitmap)

	message.Payload = buffer.Bytes()
	//// END Payload

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(&message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the Prepared message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage), aggSig
}

// Construct the committed message, returning committed message in bytes.
func (consensus *Consensus) constructCommittedMessage() ([]byte, *bls.Sign) {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_COMMITTED

	consensus.populateBasicFields(&message)

	//// Payload
	buffer := bytes.NewBuffer([]byte{})

	// 48 bytes aggregated signature
	aggSig := bls_cosi.AggregateSig(consensus.GetCommitSigsArray())
	buffer.Write(aggSig.Serialize())

	// Bitmap
	buffer.Write(consensus.commitBitmap.Bitmap)

	message.Payload = buffer.Bytes()
	//// END Payload

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(&message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the Committed message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage), aggSig
}
