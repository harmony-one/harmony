package beaconchain

import (
	"net"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/proto"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
)

// BeaconChainHandler handles registration of new Identities
func (bc *BeaconChain) BeaconChainHandler(conn net.Conn) {
	content, err := p2p.ReadMessageContent(conn)
	if err != nil {
		bc.log.Error("Read p2p data failed")
		return
	}
	bc.log.Info("received connection", "connectionIp", conn.RemoteAddr())
	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		bc.log.Error("Read message category failed", "err", err)
		return
	}
	msgType, err := proto.GetMessageType(content)
	if err != nil {
		bc.log.Error("Read action type failed")
		return
	}
	msgPayload, err := proto.GetMessagePayload(content)
	if err != nil {
		bc.log.Error("Read message payload failed")
		return
	}
	identityMsgPayload, err := proto_identity.GetIdentityMessagePayload(msgPayload)
	if err != nil {
		bc.log.Error("Read message payload failed")
		return
	}
	switch msgCategory {
	case proto.Identity:
		actionType := proto_identity.IDMessageType(msgType)
		switch actionType {
		case proto_identity.Identity:
			bc.log.Info("Message category is of the type identity protocol, which is correct!")
			idMsgType, err := proto_identity.GetIdentityMessageType(msgPayload)
			if err != nil {
				bc.log.Error("Error finding the identity message type")
			}
			switch idMsgType {
			case proto_identity.Register:
				bc.log.Info("Identity Message Type is of the type Register")
				bc.AcceptConnections(identityMsgPayload)
			default:
				panic("Unrecognized identity message type")
			}
		default:
			panic("Unrecognized message category")
		}

	}
}
