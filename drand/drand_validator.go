package drand

import (
	protobuf "github.com/golang/protobuf/proto"
	drand_proto "github.com/harmony-one/harmony/api/drand"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// ProcessMessageValidator dispatches messages for the validator to corresponding processors.
func (dRand *DRand) ProcessMessageValidator(payload []byte) {
	message := drand_proto.Message{}
	err := protobuf.Unmarshal(payload, &message)

	if err != nil {
		utils.GetLogInstance().Error("[DRG] Failed to unmarshal message payload.", "err", err, "dRand", dRand)
	}

	switch message.Type {
	case drand_proto.MessageType_INIT:
		dRand.processInitMessage(message)
	case drand_proto.MessageType_COMMIT:
		// do nothing on the COMMIT message, as it is intended to send to leader
	default:
		utils.GetLogInstance().Error("[DRG] Unexpected message type", "msgType", message.Type, "dRand", dRand)
	}
}

// ProcessMessageValidator dispatches validator's consensus message.
func (dRand *DRand) processInitMessage(message drand_proto.Message) {
	if message.Type != drand_proto.MessageType_INIT {
		utils.GetLogInstance().Error("[DRG] Wrong message type received", "expected", drand_proto.MessageType_INIT, "got", message.Type)
		return
	}

	blockHash := message.BlockHash

	// Verify message signature
	err := verifyMessageSig(dRand.leader.PubKey, message)
	if err != nil {
		utils.GetLogInstance().Warn("[DRG] Failed to verify the message signature", "Error", err, "leader.PubKey", dRand.leader.PubKey)
		return
	}

	// TODO: check the blockHash is the block hash of last block of last epoch.
	copy(dRand.blockHash[:], blockHash[:])

	rand, proof := dRand.vrf(dRand.blockHash)

	msgToSend := dRand.constructCommitMessage(rand, proof)

	// Send the commit message back to leader
	if utils.UseLibP2P {
		dRand.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, host.ConstructP2pMessage(byte(17), msgToSend))
	} else {
		host.SendMessage(dRand.host, dRand.leader, msgToSend, nil)
	}
}
