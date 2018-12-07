package host

import (
	"bytes"
	"encoding/binary"
	"net"
	"runtime"
	"time"

	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
)

// SendMessage is to connect a socket given a port and send the given message.
// TODO(minhdoan, rj): need to check if a peer is reachable or not.
func SendMessage(host Host, p p2p.Peer, message []byte) {
	// Construct normal p2p message
	content := ConstructP2pMessage(byte(0), message)
	go send(host, p, content)
}

// BroadcastMessage sends the message to a list of peers
func BroadcastMessage(h Host, peers []p2p.Peer, msg []byte) {
	if len(peers) == 0 {
		return
	}
	// Construct broadcast p2p message
	content := ConstructP2pMessage(byte(17), msg)
	length := len(content)

	log.Info("Start Broadcasting", "gomaxprocs", runtime.GOMAXPROCS(0), "Size", length)
	start := time.Now()
	for _, peer := range peers {
		peerCopy := peer
		go send(h, peerCopy, content)
	}
	log.Info("Broadcasting Done", "time spent(s)", time.Since(start).Seconds())

	// Keep track of block propagation time
	// Assume 1M message is for block propagation
	if length > 1000000 {
		log.Debug("NET: START BLOCK PROPAGATION", "Size", length)
	}
}

// ConstructP2pMessage constructs the p2p message as [messageType, contentSize, content]
func ConstructP2pMessage(msgType byte, content []byte) []byte {

	firstByte := byte(17)        // messageType 0x11
	sizeBytes := make([]byte, 4) // contentSize

	binary.BigEndian.PutUint32(sizeBytes, uint32(len(content)))

	byteBuffer := bytes.NewBuffer([]byte{})
	byteBuffer.WriteByte(firstByte)
	byteBuffer.Write(sizeBytes)
	byteBuffer.Write(content)
	return byteBuffer.Bytes()
}

// BroadcastMessageFromLeader sends the message to a list of peers from a leader.
func BroadcastMessageFromLeader(h Host, peers []p2p.Peer, msg []byte) {
	// TODO(minhdoan): Enable back for multicast.
	peers = SelectMyPeers(peers, 1, MaxBroadCast)
	BroadcastMessage(h, peers, msg)
	log.Info("Done sending from leader")
}

// BroadcastMessageFromValidator sends the message to a list of peers from a validator.
func BroadcastMessageFromValidator(h Host, selfPeer p2p.Peer, peers []p2p.Peer, msg []byte) {
	peers = SelectMyPeers(peers, selfPeer.ValidatorID*MaxBroadCast+1, (selfPeer.ValidatorID+1)*MaxBroadCast)
	BroadcastMessage(h, peers, msg)
	log.Info("Done sending from validator")
}

// MaxBroadCast is the maximum number of neighbors to broadcast
const MaxBroadCast = 20

// SelectMyPeers chooses a list of peers based on the range of ValidatorID
// This is a quick hack of the current p2p networking model
func SelectMyPeers(peers []p2p.Peer, min int, max int) []p2p.Peer {
	res := []p2p.Peer{}
	for _, peer := range peers {
		if peer.ValidatorID >= min && peer.ValidatorID <= max {
			res = append(res, peer)
		}
	}
	return res
}

// Send a message to another node with given port.
func send(h Host, peer p2p.Peer, message []byte) {
	// Add attack code here.
	//attack.GetInstance().Run()
	backoff := p2p.NewExpBackoff(250*time.Millisecond, 10*time.Second, 2)

	for trial := 0; trial < 10; trial++ {
		var err error
		h.SendMessage(peer, message)
		if err == nil {
			if trial > 0 {
				log.Warn("retry send", "rety", trial)
			}
			return
		}
		log.Info("sleeping before trying to send again",
			"duration", backoff.Cur, "addr", net.JoinHostPort(peer.IP, peer.Port))
		backoff.Sleep()
	}
	log.Error("gave up sending a message", "addr", net.JoinHostPort(peer.IP, peer.Port))
}

// DialWithSocketClient joins host port and establishes connection
func DialWithSocketClient(ip, port string) (conn net.Conn, err error) {
	//log.Printf("Sending message to ip %s and port %s\n", ip, port)
	addr := net.JoinHostPort(ip, port)
	conn, err = net.Dial("tcp", addr)
	return
}
