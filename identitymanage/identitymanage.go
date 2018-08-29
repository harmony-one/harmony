package identitymanage

// import (
// 	"bytes"
// 	"encoding/binary"
// 	"log"

// 	"github.com/dedis/kyber"
// )

// // Consensus data containing all info related to one round of consensus process
// type Identity struct {
// 	priKey kyber.Scalar
// 	pubKey kyber.Point
// 	Log    log.Logger
// }

// // Construct the response message to send to leader (assumption the consensus data is already verified)
// func (identity *Identity) registerIdentity(msgType proto_consensus.MessageType, response kyber.Scalar) []byte {
// 	buffer := bytes.NewBuffer([]byte{})

// 	// 4 byte consensus id
// 	fourBytes := make([]byte, 4)
// 	binary.BigEndian.PutUint32(fourBytes, consensus.consensusId)
// 	buffer.Write(fourBytes)

// 	// 32 byte block hash
// 	buffer.Write(consensus.blockHash[:32])

// 	// 2 byte validator id
// 	twoBytes := make([]byte, 2)
// 	binary.BigEndian.PutUint16(twoBytes, consensus.nodeId)
// 	buffer.Write(twoBytes)

// 	// 32 byte of response
// 	response.MarshalTo(buffer)

// 	// 64 byte of signature on previous data
// 	signature := consensus.signMessage(buffer.Bytes())
// 	buffer.Write(signature)

// 	return proto_identity.ConstructIdentityMessage(msgType, buffer.Bytes())
// }
