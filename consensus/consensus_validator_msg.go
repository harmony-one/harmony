package consensus

import (
	protobuf "github.com/golang/protobuf/proto"
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/internal/utils"
)

// Construct the prepare message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructPrepareMessage() []byte {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_PREPARE

	consensus.populateBasicFields(&message)

	// 48 byte of bls signature
	sign := consensus.priKey.SignHash(message.BlockHash)
	if sign != nil {
		message.Payload = sign.Serialize()
	}

	err := consensus.signConsensusMessage(&message)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to sign the Prepare message", "error", err)
	}

	marshaledMessage, err := protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to marshal Prepare message", "error", err)
	}

	return proto.ConstructConsensusMessage(marshaledMessage)
}

// Construct the commit message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructCommitMessage(multiSigAndBitmap []byte) []byte {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_COMMIT

	consensus.populateBasicFields(&message)

	// 48 byte of bls signature
	sign := consensus.priKey.SignHash(multiSigAndBitmap)
	if sign != nil {
		message.Payload = sign.Serialize()
	}

	err := consensus.signConsensusMessage(&message)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to sign the Commit message", "error", err)
	}

	marshaledMessage, err := protobuf.Marshal(&message)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to marshal Commit message", "error", err)
	}

	return proto.ConstructConsensusMessage(marshaledMessage)
}
