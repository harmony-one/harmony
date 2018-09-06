package identitychain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/pow"
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
	case proto.IDENTITY:
		actionType := proto_identity.IdentityMessageType(msgType)
		switch actionType {
		case proto_identity.IDENTITY:
			idMsgType, err := proto_identity.GetIdentityMessageType(msgPayload)
			if err != nil {
				fmt.Println("Error finding the identity message type")
			}
			switch idMsgType {
			case proto_identity.REGISTER:
				IDC.registerIdentity(msgPayload)
			case proto_identity.ANNOUNCE:
				IDC.acceptNewConnection(msgPayload)
			}

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
	fmt.Println("we are now registering identities")
	//reconstruct the challenge and check whether its correct
	offset := 0
	proof := payload[offset : offset+32]
	offset = offset + 32
	Node := node.DeserializeWaitNode(payload[offset:])
	id := int(IDC.PowMap[Node.Self])
	req := pow.NewRequest(5, []byte(strconv.Itoa(id)))
	ok, err := pow.Check(req, string(proof), []byte(""))
	fmt.Println(err)
	if ok {
		fmt.Println("Proof of work accepted")
		IDC.PendingIdentities = append(IDC.PendingIdentities, Node)
		fmt.Println(len(IDC.PendingIdentities)) //Fix why IDC does not have log working.
	} else {
		fmt.Println("identity proof of work not accepted")
	}
}

func (IDC *IdentityChain) acceptNewConnection(msgPayload []byte) {

	identityPayload, err := proto_identity.GetIdentityMessagePayload(msgPayload)
	if err != nil {
		fmt.Println("There was a error in reading the identity payload")
	} else {
		fmt.Println("accepted new connection")
	}
	fmt.Println("Sleeping for 2 secs ...")
	time.Sleep(5 * time.Second)
	Node := node.DeserializeWaitNode(identityPayload)
	buffer := bytes.NewBuffer([]byte{})
	src := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(src)
	challengeNonce := uint32(rnd.Int31())
	//challengeNonce := uint32(rand.Intn(1000)) //fix so that different nonce is sent everytime.
	fmt.Println("Challenge Nonce Sent:")
	fmt.Println(challengeNonce)
	IDC.PowMap[Node.Self] = challengeNonce
	// 4 byte length of challengeNonce

	fourBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fourBytes, challengeNonce)
	buffer.Write(fourBytes)
	// 32 byte block hash
	// buffer.Write(prevBlockHash)
	// The message is missing previous BlockHash, this is because we don't actively maintain a identitychain
	// This canbe included in the fulfill request.

	// Message should be encrypted and then signed to follow PKE.
	//IDC should accept node publickey, encrypt the nonce and blockhash
	// Then sign the message by own private key and send the message back.
	fmt.Println("Done sleeping. Ready or not here i come!")
	msgToSend := proto_identity.ConstructIdentityMessage(proto_identity.REGISTER, buffer.Bytes())
	p2p.SendMessage(Node.Self, msgToSend)
}
