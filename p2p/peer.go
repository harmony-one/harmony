package p2p

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"runtime"
	"time"

	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2pv2"

	"github.com/dedis/kyber"
)

// Peer is the object for a p2p peer (node)
type Peer struct {
	IP          string      // IP address of the peer
	Port        string      // Port number of the peer
	PubKey      kyber.Point // Public key of the peer
	Ready       bool        // Ready is true if the peer is ready to join consensus.
	ValidatorID int         // -1 is the default value, means not assigned any validator ID in the shard
	// TODO(minhdoan, rj): use this Ready to not send/broadcast to this peer if it wasn't available.
}

// MaxBroadCast is the maximum number of neighbors to broadcast
const MaxBroadCast = 20

// Version The version number of p2p library
// 1 - Direct socket connection
// 2 - libp2p
const Version = 1

// SendMessage sends the message to the peer
func SendMessage(peer Peer, msg []byte) {
	// Construct normal p2p message
	content := ConstructP2pMessage(byte(0), msg)
	go send(peer.IP, peer.Port, content)
}

// BroadcastMessage sends the message to a list of peers
func BroadcastMessage(peers []Peer, msg []byte) {
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
		go send(peerCopy.IP, peerCopy.Port, content)
	}
	log.Info("Broadcasting Done", "time spent(s)", time.Since(start).Seconds())

	// Keep track of block propagation time
	// Assume 1M message is for block propagation
	if length > 1000000 {
		log.Debug("NET: START BLOCK PROPAGATION", "Size", length)
	}
}

// SelectMyPeers chooses a list of peers based on the range of ValidatorID
// This is a quick hack of the current p2p networking model
func SelectMyPeers(peers []Peer, min int, max int) []Peer {
	res := []Peer{}
	for _, peer := range peers {
		if peer.ValidatorID >= min && peer.ValidatorID <= max {
			res = append(res, peer)
		}
	}
	return res
}

// BroadcastMessageFromLeader sends the message to a list of peers from a leader.
func BroadcastMessageFromLeader(peers []Peer, msg []byte) {
	// TODO(minhdoan): Enable back for multicast.
	peers = SelectMyPeers(peers, 1, MaxBroadCast)
	BroadcastMessage(peers, msg)
}

// BroadcastMessageFromValidator sends the message to a list of peers from a validator.
func BroadcastMessageFromValidator(selfPeer Peer, peers []Peer, msg []byte) {
	peers = SelectMyPeers(peers, selfPeer.ValidatorID*MaxBroadCast+1, (selfPeer.ValidatorID+1)*MaxBroadCast)
	BroadcastMessage(peers, msg)
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

// SocketClient is to connect a socket given a port and send the given message.
// TODO(minhdoan, rj): need to check if a peer is reachable or not.
func sendWithSocketClient(ip, port string, message []byte) (err error) {
	//log.Printf("Sending message to ip %s and port %s\n", ip, port)
	addr := net.JoinHostPort(ip, port)
	conn, err := net.Dial("tcp", addr)

	if err != nil {
		log.Warn("Dial() failed", "addr", addr, "error", err)
		return
	}
	defer conn.Close()

	nw, err := conn.Write(message)
	if err != nil {
		log.Warn("Write() failed", "addr", conn.RemoteAddr(), "error", err)
		return
	}
	if nw < len(message) {
		log.Warn("Write() returned short count",
			"addr", conn.RemoteAddr(), "actual", nw, "expected", len(message))
		return io.ErrShortWrite
	}

	//log.Printf("Sent to ip %s and port %s: %s\n", ip, port, message)

	// No ack (reply) message from the receiver for now.
	return
}

// Send a message to another node with given port.
func send(ip, port string, message []byte) {
	// Add attack code here.
	//attack.GetInstance().Run()
	backoff := NewExpBackoff(250*time.Millisecond, 10*time.Second, 2)

	for trial := 0; trial < 10; trial++ {
		var err error
		if Version == 1 {
			// TODO(ricl): remove sendWithSocketClient related code.
			err = sendWithSocketClient(ip, port, message)
		} else {
			err = p2pv2.Send(ip, port, message)
		}
		if err == nil {
			if trial > 0 {
				log.Warn("retry sendWithSocketClient", "rety", trial)
			}
			return
		}
		log.Info("sleeping before trying to send again",
			"duration", backoff.Cur, "addr", net.JoinHostPort(ip, port))
		backoff.Sleep()
	}
	log.Error("gave up sending a message", "addr", net.JoinHostPort(ip, port))
}

// DialWithSocketClient joins host port and establishes connection
func DialWithSocketClient(ip, port string) (conn net.Conn, err error) {
	//log.Printf("Sending message to ip %s and port %s\n", ip, port)
	addr := net.JoinHostPort(ip, port)
	conn, err = net.Dial("tcp", addr)
	return
}
