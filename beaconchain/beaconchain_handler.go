package beaconchain

import (
	"fmt"
	"net"
	"os"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/proto"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
)

//BeaconChainHandler handles registration of new Identities
// This could have been its seperate package like consensus, but am avoiding creating a lot of packages.
func (IDC *BeaconChain) BeaconChainHandler(conn net.Conn) {
	content, err := p2p.ReadMessageContent(conn)
	if err != nil {
		IDC.log.Error("Read p2p data failed")
		return
	} else {
		IDC.log.Info("received connection")
	}
	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		IDC.log.Error("Read message category failed", "err", err)
		return
	}
	if msgCategory != proto.Identity {
		IDC.log.Error("Identity Chain Recieved incorrect protocol message")
		os.Exit(1)
	} else {
		fmt.Println("Message category is correct")
	}
	msgType, err := proto.GetMessageType(content)
	if err != nil {
		IDC.log.Error("Read action type failed")
		return
	}
	msgPayload, err := proto.GetMessagePayload(content)
	if err != nil {
		IDC.log.Error("Read message payload failed")
		return
	}
	switch msgCategory {
	case proto.Identity:
		actionType := proto_identity.IdentityMessageType(msgType)
		switch actionType {
		case proto_identity.Identity:
			idMsgType, err := proto_identity.GetIdentityMessageType(msgPayload)
			if err != nil {
				fmt.Println("Error finding the identity message type")
			}
			switch idMsgType {
			case proto_identity.Register:
				IDC.AcceptConnections(msgPayload)
			case proto_identity.Acknowledge:
				// IDC.acceptNewConnection(msgPayload)
			}

		}

	}
}
