package drand

import (
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constructs the init message
func (dRand *DRand) constructCommitMessage(vrf [32]byte, proof []byte) []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_DRAND,
		Type:        msg_pb.MessageType_DRAND_COMMIT,
		Request: &msg_pb.Message_Drand{
			Drand: &msg_pb.DrandRequest{},
		},
	}

	drandMsg := message.GetDrand()
	drandMsg.SenderPubkey = dRand.pubKey.Serialize()
	drandMsg.BlockHash = dRand.blockHash[:]
	drandMsg.ShardId = dRand.ShardID
	drandMsg.Payload = append(vrf[:], proof...)
	// Adding the public key into payload so leader can verify the vrf
	// TODO: change the curve to follow the same curve with consensus, so the public key doesn't need to be attached.
	drandMsg.Payload = append(drandMsg.Payload, (*dRand.vrfPubKey).Serialize()...)
	marshaledMessage, err := dRand.signAndMarshalDRandMessage(message)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to sign and marshal the commit message")
	}
	return proto.ConstructDRandMessage(marshaledMessage)
}
