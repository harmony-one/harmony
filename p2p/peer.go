package p2p

import (
	"bytes"
	"encoding/binary"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/simple-rules/harmony-benchmark/attack"
	"github.com/simple-rules/harmony-benchmark/log"

	"github.com/dedis/kyber"
)

// Peer is the object for a p2p peer (node)
type Peer struct {
	Ip     string      // Ip address of the peer
	Port   string      // Port number of the peer
	PubKey kyber.Point // Public key of the peer
	Ready  bool        // Ready is true if the peer is ready to join consensus.
	// TODO(minhdoan, rj): use this Ready to not send/broadcast to this peer if it wasn't available.
}

// SendMessage sends the message to the peer
func SendMessage(peer Peer, msg []byte) {
	// Construct normal p2p message
	content := ConstructP2pMessage(byte(0), msg)

	send(peer.Ip, peer.Port, content)
}

// BroadcastMessage sends the message to a list of peers
func BroadcastMessage(peers []Peer, msg []byte) {
	// Construct broadcast p2p message
	content := ConstructP2pMessage(byte(17), msg)

	var wg sync.WaitGroup
	wg.Add(len(peers))

	log.Info("Start Broadcasting", "gomaxprocs", runtime.GOMAXPROCS(0))
	start := time.Now()
	for _, peer := range peers {
		peerCopy := peer
		go func() {
			defer wg.Done()
			send(peerCopy.Ip, peerCopy.Port, content)
		}()
	}
	wg.Wait()
	log.Info("Broadcasting Down", "time spent", time.Now().Sub(start).Seconds())
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
	return compressContent(byteBuffer.Bytes())
}

// SocketClient is to connect a socket given a port and send the given message.
// TODO(minhdoan, rj): need to check if a peer is reachable or not.
func sendWithSocketClient(ip, port string, message []byte) (res string) {
	//log.Printf("Sending message to ip %s and port %s\n", ip, port)
	addr := strings.Join([]string{ip, port}, ":")
	conn, err := net.Dial("tcp", addr)

	if err != nil {
		log.Warn("Error dailing tcp", "address", addr, "error", err)
		return
	}
	defer conn.Close()

	conn.Write(message)
	//log.Printf("Sent to ip %s and port %s: %s\n", ip, port, message)

	// No ack (reply) message from the receiver for now.
	return
}

// Send a message to another node with given port.
func send(ip, port string, message []byte) (returnMessage string) {
	// Add attack code here.
	attack.GetInstance().Run()

	sendWithSocketClient(ip, port, message)
	return
}

func DialWithSocketClient(ip, port string) (conn net.Conn, err error) {
	//log.Printf("Sending message to ip %s and port %s\n", ip, port)
	addr := strings.Join([]string{ip, port}, ":")
	conn, err = net.Dial("tcp", addr)
	return
}
