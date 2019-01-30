package consensus

import (
	consensus_proto "github.com/harmony-one/harmony/api/consensus"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/internal/utils"
)

// Construct the prepare message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructPrepareMessage() []byte {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_PREPARE

	consensus.populateMessageFields(&message)

	// 48 byte of bls signature
	sign := consensus.priKey.SignHash(message.BlockHash)
	if sign != nil {
		message.Payload = sign.Serialize()
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(&message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the Prepare message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}

// Construct the commit message to send to leader (assumption the consensus data is already verified)
func (consensus *Consensus) constructCommitMessage(multiSigAndBitmap []byte) []byte {
	message := consensus_proto.Message{}
	message.Type = consensus_proto.MessageType_COMMIT

	consensus.populateMessageFields(&message)

	// 48 byte of bls signature
	sign := consensus.priKey.SignHash(multiSigAndBitmap)
	if sign != nil {
		message.Payload = sign.Serialize()
	}

	marshaledMessage, err := consensus.signAndMarshalConsensusMessage(&message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the Commit message", "error", err)
	}
	return proto.ConstructConsensusMessage(marshaledMessage)
}
