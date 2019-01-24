package consensus

import (
	"bytes"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/api/proto"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
)

// Constructs the announce message
func (consensus *Consensus) constructAnnounceMessage() []byte {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_ANNOUNCE

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	// n byte of block header
	message.Payload = consensus.block

	marshaledMessage, err := protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Announce message", "error", err)
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Announce message", "error", err)
	}
	consensus.Log.Info("New Announce", "NodeID", consensus.nodeID)
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// Construct the prepared message, returning prepared message in bytes.
func (consensus *Consensus) constructPreparedMessage() ([]byte, *bls.Sign) {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_PREPARED

	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	//// Payload
	buffer := bytes.NewBuffer([]byte{})

	// 48 bytes aggregated signature
	aggSig := bls_cosi.AggregateSig(consensus.GetPrepareSigsArray())
	buffer.Write(aggSig.Serialize())

	// Bitmap
	buffer.Write(consensus.prepareBitmap.Bitmap)

	message.Payload = buffer.Bytes()
	//// END Payload

	// TODO: use custom serialization method rather than protobuf
	marshaledMessage, err := protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Prepared message", "error", err)
	}
	// 48 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Prepared message", "error", err)
	}
	consensus.Log.Info("New Prepared Message", "NodeID", consensus.nodeID, "bitmap", consensus.prepareBitmap)
	return proto.ConstructConsensusMessage(marshaledMessage), aggSig
}

// Construct the committed message, returning committed message in bytes.
func (consensus *Consensus) constructCommittedMessage() ([]byte, *bls.Sign) {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_COMMITTED
	// 4 byte consensus id
	message.ConsensusId = consensus.consensusID

	// 32 byte block hash
	message.BlockHash = consensus.blockHash[:]

	// 4 byte sender id
	message.SenderId = uint32(consensus.nodeID)

	//// Payload
	buffer := bytes.NewBuffer([]byte{})

	// 48 bytes aggregated signature
	aggSig := bls_cosi.AggregateSig(consensus.GetCommitSigsArray())
	buffer.Write(aggSig.Serialize())

	// Bitmap
	buffer.Write(consensus.commitBitmap.Bitmap)

	message.Payload = buffer.Bytes()
	//// END Payload

	// TODO: use custom serialization method rather than protobuf
	marshaledMessage, err := protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Committed message", "error", err)
	}
	// 48 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature

	marshaledMessage, err = protobuf.Marshal(&message)
	if err != nil {
		consensus.Log.Debug("Failed to marshal Committed message", "error", err)
	}
	consensus.Log.Info("New Prepared Message", "NodeID", consensus.nodeID, "bitmap", consensus.commitBitmap)
	return proto.ConstructConsensusMessage(marshaledMessage), aggSig
}
