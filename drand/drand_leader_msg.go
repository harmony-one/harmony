package drand

import (
	"github.com/harmony-one/harmony/api/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constructs the init message
func (dRand *DRand) constructInitMessage() []byte {
	message := &msg_pb.Message{
		ServiceType: msg_pb.ServiceType_DRAND,
		Type:        msg_pb.MessageType_DRAND_INIT,
		Request: &msg_pb.Message_Drand{
			Drand: &msg_pb.DrandRequest{},
		},
	}

	drandMsg := message.GetDrand()
	drandMsg.SenderPubkey = dRand.pubKey.Serialize()
	drandMsg.BlockHash = dRand.blockHash[:]
	// Don't need the payload in init message
	marshaledMessage, err := dRand.signAndMarshalDRandMessage(message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the init message", "error", err)
	}
	return proto.ConstructDRandMessage(marshaledMessage)
}
