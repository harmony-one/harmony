package newnode

import (
	"fmt"
	"net"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/proto"
	proto_identity "github.com/harmony-one/harmony/proto/identity"
)

// NodeHandler handles a new incoming connection.
func (node *NewNode) NodeHandler(conn net.Conn) {
	defer conn.Close()

	content, err := p2p.ReadMessageContent(conn)

	if err != nil {
		node.log.Error("Read p2p data failed", "err", err, "node", node)
		return
	}

	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		node.log.Error("Read node type failed", "err", err, "node", node)
		return
	}

	msgType, err := proto.GetMessageType(content)
	if err != nil {
		node.log.Error("Read action type failed", "err", err, "node", node)
		return
	}

	msgPayload, err := proto.GetMessagePayload(content)
	if err != nil {
		node.log.Error("Read message payload failed", "err", err, "node", node)
		return
	}
	identityMsgPayload, err := proto_identity.GetIdentityMessagePayload(msgPayload)
	if err != nil {
		node.log.Error("Read message payload failed")
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
			case proto_identity.Acknowledge:
				node.processShardInfo(identityMsgPayload)
			}

		}

	}
}
