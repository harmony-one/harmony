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
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "dRand", dRand)
	}

	if message.Type == msg_pb.MessageType_DRAND_INIT {
		dRand.processInitMessage(message)
	} else {
		utils.GetLogInstance().Error("Unexpected message type", "msgType", message.Type, "dRand", dRand)
	}
}

// ProcessMessageValidator dispatches validator's consensus message.
func (dRand *DRand) processInitMessage(message *msg_pb.Message) {
	if message.Type != msg_pb.MessageType_DRAND_INIT {
		utils.GetLogInstance().Error("Wrong message type received", "expected", msg_pb.MessageType_DRAND_INIT, "got", message.Type)
		return
	}

	drandMsg := message.GetDrand()

	// Verify message signature
	err := verifyMessageSig(dRand.leader.ConsensusPubKey, message)
	if err != nil {
		utils.GetLogInstance().Warn("[DRG] Failed to verify the message signature", "Error", err)
		return
	}
	utils.GetLogInstance().Debug("[DRG] verify the message signature Succeeded")

	// TODO: check the blockHash is the block hash of last block of last epoch.
	blockHash := drandMsg.BlockHash
	copy(dRand.blockHash[:], blockHash[:])

	rand, proof := dRand.vrf(dRand.blockHash)

	msgToSend := dRand.constructCommitMessage(rand, proof)

	// Send the commit message back to leader
	dRand.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, host.ConstructP2pMessage(byte(17), msgToSend))
}
