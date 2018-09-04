package identitychain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"os"

	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/proto"
	proto_identity "github.com/simple-rules/harmony-benchmark/proto/identity"
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
		actionType := proto_identity.MessageType(msgType)
		switch actionType {
		case proto_identity.REGISTER:
			IDC.registerIdentity(msgPayload)
		case proto_identity.ANNOUNCE:
			IDC.acceptNewConnection(msgPayload)
		}

	}
}

func (IDC *IdentityChain) registerIdentity(msgPayload []byte) {
	payload, err := proto_identity.GetIdentityMessagePayload(msgPayload)
	if err != nil {
		IDC.log.Error("identity payload not read")
	} else {
		fmt.Println("identity payload read")
	}
	//reconstruct the challenge and check whether its correct
	offset := 0
	//proof := payload[offset : offset+64]

	offset = offset + 64
	Node := node.DeserializeNode(payload[offset:])
	fmt.Println(Node.Self)
	os.Exit(1)
	// id := []byte(IDC.PowMap[Node.Self])
	// req := pow.NewRequest(5, id)
	// ok, _ := pow.Check(req, string(proof), []byte("This is blockchash data"))
	// if ok {
	// 	IDC.PendingIdentities = append(IDC.PendingIdentities, Node)
	// 	fmt.Println(len(IDC.PendingIdentities)) //Fix why IDC does not have log working.
	// } else {
	// 	fmt.Println("identity proof of work not accepted")
	// }
}

func (IDC *IdentityChain) acceptNewConnection(msgPayload []byte) {

	identityPayload, err := proto_identity.GetIdentityMessagePayload(msgPayload)
	if err != nil {
		fmt.Println("There was a error in reading the identity payload")
	}
	Node := node.DeserializeNode(identityPayload)
	buffer := bytes.NewBuffer([]byte{})
	challengeNonce := uint32(rand.Intn(1000))
	IDC.PowMap[Node.Self] = challengeNonce

	// 64 byte length of challenge
	fourBytes := make([]byte, 64)
	binary.BigEndian.PutUint32(fourBytes, challengeNonce)
	buffer.Write(fourBytes)

	sendMsgPayload := buffer.Bytes()

	// 32 byte block hash
	// buffer.Write(prevBlockHash)
	// The message is missing previous BlockHash, this is because we don't actively maintain a identitychain
	// This canbe included in the fulfill request.

	// Message should be encrypted and then signed to follow PKE.
	//IDC should accept node publickey, encrypt the nonce and blockhash
	// Then sign the message by own private key and send the message back.

	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.REGISTER, sendMsgPayload)
	p2p.SendMessage(Node.Self, msgToSend)
}
