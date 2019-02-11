package drand

import (
	drand_proto "github.com/harmony-one/harmony/api/drand"
	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constructs the init message
func (drand *DRand) constructInitMessage() []byte {
	message := drand_proto.Message{}
	message.Type = drand_proto.MessageType_INIT

	copy(message.BlockHash, drand.blockHash[:])
	// Don't need the payload in init message
	marshaledMessage, err := drand.signAndMarshalDRandMessage(&message)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign and marshal the init message", "error", err)
	}
	return proto.ConstructDRandMessage(marshaledMessage)
}
