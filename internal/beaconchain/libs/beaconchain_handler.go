package beaconchain

import (
	"github.com/harmony-one/harmony/api/proto"
	proto_identity "github.com/harmony-one/harmony/api/proto/identity"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// BeaconChainHandler handles registration of new Identities
func (bc *BeaconChain) BeaconChainHandler(s p2p.Stream) {
	content, err := p2p.ReadMessageContent(s)
	if err != nil {
		utils.GetLogInstance().Error("Read p2p data failed")
		return
	}
	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		utils.GetLogInstance().Error("Read message category failed", "err", err)
		return
	}
	msgType, err := proto.GetMessageType(content)
	if err != nil {
		utils.GetLogInstance().Error("Read action type failed")
		return
	}
	msgPayload, err := proto.GetMessagePayload(content)
	if err != nil {
		utils.GetLogInstance().Error("Read message payload failed")
		return
	}
	identityMsgPayload, err := proto_identity.GetIdentityMessagePayload(msgPayload)
	if err != nil {
		utils.GetLogInstance().Error("Read message payload failed")
		return
	}
	switch msgCategory {
	case proto.Identity:
		actionType := proto_identity.IDMessageType(msgType)
		switch actionType {
		case proto_identity.Identity:
			utils.GetLogInstance().Info("Message category is of the type identity protocol, which is correct!")
			idMsgType, err := proto_identity.GetIdentityMessageType(msgPayload)
			if err != nil {
				utils.GetLogInstance().Error("Error finding the identity message type")
			}
			switch idMsgType {
			case proto_identity.Register:
				utils.GetLogInstance().Info("Identity Message Type is of the type Register")
				bc.AcceptConnections(identityMsgPayload)
			default:
				utils.GetLogInstance().Error("Unrecognized identity message type", "type", idMsgType)
			}
		default:
			utils.GetLogInstance().Error("Unrecognized message category", "actionType", actionType)
		}

	}
}
