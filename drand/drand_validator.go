package drand

import (
	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// ProcessMessageValidator dispatches messages for the validator to corresponding processors.
func (dRand *DRand) ProcessMessageValidator(payload []byte) {
	message := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, message)

	if err != nil {
		utils.Logger().Error().Interface("dRand", dRand).Err(err).Msg("Failed to unmarshal message payload")
	}

	switch message.Type {
	case msg_pb.MessageType_DRAND_INIT:
		dRand.processInitMessage(message)
	case msg_pb.MessageType_DRAND_COMMIT:
		// do nothing on the COMMIT message, as it is intended to send to leader
	default:
		utils.Logger().Error().
			Interface("dRand", dRand).
			Uint32("msgType", uint32(message.Type)).
			Msg("Unexpected message type")
	}
}

// ProcessMessageValidator dispatches validator's consensus message.
func (dRand *DRand) processInitMessage(message *msg_pb.Message) {
	if message.Type != msg_pb.MessageType_DRAND_INIT {
		utils.Logger().Error().
			Uint32("expected", uint32(msg_pb.MessageType_DRAND_INIT)).
			Uint32("got", uint32(message.Type)).
			Msg("Wrong message type received")
		return
	}

	drandMsg := message.GetDrand()

	// Verify message signature
	err := verifyMessageSig(dRand.leader.ConsensusPubKey, message)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("[DRG] Failed to verify the message signature")
		return
	}
	utils.Logger().Debug().Msg("[DRG] verify the message signature Succeeded")

	// TODO: check the blockHash is the block hash of last block of last epoch.
	blockHash := drandMsg.BlockHash
	copy(dRand.blockHash[:], blockHash[:])

	rand, proof := dRand.vrf(dRand.blockHash)

	msgToSend := dRand.constructCommitMessage(rand, proof)

	// Send the commit message back to leader
	dRand.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(dRand.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))
}
