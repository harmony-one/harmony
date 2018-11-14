package beaconchain

import (
	"fmt"
	"net"
	"os"

	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto"
	proto_identity "github.com/simple-rules/harmony-benchmark/proto/identity"
)

//BeaconChainHandler handles registration of new Identities
// This could have been its seperate package like consensus, but am avoiding creating a lot of packages.
func (IDC *BeaconChain) BeaconChainHandler(conn net.Conn) {
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

// TODO(alok): You removed pow package.
// func (IDC *IdentityChain) registerIdentity(msgPayload []byte) {
// 	payload, err := proto_identity.GetIdentityMessagePayload(msgPayload)
// 	if err != nil {
// 		IDC.log.Error("identity payload not read")
// 	} else {
// 		fmt.Println("identity payload read")
// 	}
// 	fmt.Println("we are now registering identities")
// 	offset := 0
// 	proof := payload[offset : offset+32]
// 	offset = offset + 32
// 	Node := node.DeserializeWaitNode(payload[offset:])
// 	req := IDC.PowMap[Node.Self]
// 	ok, err := pow.Check(req, string(proof), []byte(""))
// 	fmt.Println(err)
// 	if ok {
// 		fmt.Println("Proof of work accepted")
// 		IDC.PendingIdentities = append(IDC.PendingIdentities, Node)
// 		fmt.Println(len(IDC.PendingIdentities)) //Fix why IDC does not have log working.
// 	} else {
// 		fmt.Println("identity proof of work not accepted")
// 	}
// }

// func (IDC *IdentityChain) acceptNewConnection(msgPayload []byte) {

// 	identityPayload, err := proto_identity.GetIdentityMessagePayload(msgPayload)
// 	if err != nil {
// 		fmt.Println("There was a error in reading the identity payload")
// 	} else {
// 		fmt.Println("accepted new connection")
// 	}
// 	fmt.Println("Sleeping for 2 secs ...")
// 	time.Sleep(2 * time.Second)
// 	Node := node.DeserializeWaitNode(identityPayload)
// 	buffer := bytes.NewBuffer([]byte{})
// 	src := rand.NewSource(time.Now().UnixNano())
// 	rnd := rand.New(src)
// 	challengeNonce := int((rnd.Int31()))
// 	req := pow.NewRequest(5, []byte(strconv.Itoa(challengeNonce)))
// 	IDC.PowMap[Node.Self] = req
// 	buffer.Write([]byte(req))
// 	// 32 byte block hash
// 	// buffer.Write(prevBlockHash)
// 	// The message is missing previous BlockHash, this is because we don't actively maintain a identitychain
// 	// This canbe included in the fulfill request.
// 	// Message should be encrypted and then signed to follow PKE.
// 	//IDC should accept node publickey, encrypt the nonce and blockhash
// 	// Then sign the message by own private key and send the message back.
// 	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.Register, buffer.Bytes())
// 	p2p.SendMessage(Node.Self, msgToSend)
// }
