package identitychain

import (
	"fmt"
	"net"
	"os"

	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto"
	proto_identity "github.com/simple-rules/harmony-benchmark/proto/identity"
	"github.com/simple-rules/harmony-benchmark/waitnode"
)

//IdentityChainHandler handles registration of new Identities
// This could have been its seperate package like consensus, but am avoiding creating a lot of packages.
func (IDC *IdentityChain) IdentityChainHandler(conn net.Conn) {
	content, err := p2p.ReadMessageContent(conn)
	if err != nil {
		IDC.log.Error("Read p2p data failed")
		return
	}
	msgCategory, err := proto.GetMessageCategory(content)
	if err != nil {
		IDC.log.Error("Read message category failed", "err", err)
		return
	}
	if msgCategory != proto.IDENTITY {
		IDC.log.Error("Identity Chain Recieved incorrect protocol message")
		os.Exit(1)
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
	case proto.IDENTITY:
		actionType := proto_identity.IdentityMessageType(msgType)
		switch actionType {
		case proto_identity.IDENTITY:
			identityPayload, err := proto_identity.GetIdentityMessagePayload(msgPayload)
			if err != nil {
				IDC.log.Error("identity payload not read")
			} else {
				fmt.Println("identity payload read")
			}
			NewWaitNode := waitnode.DeserializeWaitNode(identityPayload)
			IDC.PendingIdentities = append(IDC.PendingIdentities, NewWaitNode)
			fmt.Println(len(IDC.PendingIdentities))
		}

	}
}
