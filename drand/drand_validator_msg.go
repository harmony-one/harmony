package drand

import (
	drand_proto "github.com/harmony-one/harmony/api/drand"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constructs the init message
func (dRand *DRand) constructCommitMessage(vrf [32]byte, proof []byte) []byte {
	message := drand_proto.Message{}
	message.Type = drand_proto.MessageType_COMMIT
	message.SenderId = dRand.nodeID

	message.BlockHash = dRand.blockHash[:]
	message.Payload = append(vrf[:], proof...)
	// Adding the public key into payload so leader can verify the vrf
	// TODO: change the curve to follow the same curve with consensus, so the public key doesn't need to be attached.
	message.Payload = append(message.Payload, (*dRand.vrfPubKey).Serialize()...)
	marshaledMessage, err := dRand.signAndMarshalDRandMessage(&message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the commit message", "error", err)
	}
	return proto.ConstructDRandMessage(marshaledMessage)
}
