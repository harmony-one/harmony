/*
Package proto/discovery implements the discovery ping protocol among nodes.

pingpong.go adds support of ping messages.

ping: from node to peers, sending IP/Port/PubKey info
*/

package discovery

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/harmony-one/harmony/api/proto"
	"github.com/harmony-one/harmony/api/proto/node"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// PingMessageType defines the data structure of the Ping message
type PingMessageType struct {
	Version uint16 // version of the protocol
	NodeVer string // version of the node binary
	Node    node.Info
}

func (p PingMessageType) String() string {
	return fmt.Sprintf("ping:%v/%v=>%v:%v/%v", p.Node.Role, p.Version, p.Node.IP, p.Node.Port, p.Node.PubKey)
}

// NewPingMessage creates a new Ping message based on the p2p.Peer input
func NewPingMessage(peer p2p.Peer, isClient bool) *PingMessageType {
	ping := new(PingMessageType)

	ping.Version = proto.ProtocolVersion
	ping.NodeVer = nodeconfig.GetVersion()
	ping.Node.IP = peer.IP
	ping.Node.Port = peer.Port
	ping.Node.PeerID = peer.PeerID
	if !isClient {
		ping.Node.PubKey = peer.ConsensusPubKey.Serialize()
		ping.Node.Role = node.ValidatorRole
	} else {
		ping.Node.PubKey = nil
		ping.Node.Role = node.ClientRole
	}

	return ping
}

// GetPingMessage deserializes the Ping Message from a list of byte
func GetPingMessage(payload []byte) (*PingMessageType, error) {
	ping := new(PingMessageType)

	r := bytes.NewBuffer(payload)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(ping)

	if err != nil {
		utils.Logger().Error().Err(err).Msg("[GetPingMessage] Decode")
		return nil, fmt.Errorf("Decode Ping Error")
	}

	return ping, nil
}

// ConstructPingMessage contructs ping message from node to leader
func (p PingMessageType) ConstructPingMessage() []byte {
	byteBuffer := bytes.NewBuffer([]byte{byte(proto.Node)})
	byteBuffer.WriteByte(byte(node.PING))

	encoder := gob.NewEncoder(byteBuffer)
	err := encoder.Encode(p)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[ConstructPingMessage] Encode")
		return nil
	}
	return byteBuffer.Bytes()
}
