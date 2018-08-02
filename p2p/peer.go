package p2p

import (
	"bytes"
	"encoding/binary"
	"github.com/dedis/kyber"
	"harmony-benchmark/attack"
	"log"
	"net"
	"strings"
	"sync"
)

// Peer is the object for a p2p peer (node)
type Peer struct {
	Ip     string      // Ip address of the peer
	Port   string      // Port number of the peer
	PubKey kyber.Point // Public key of the peer
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

	for _, peer := range peers {
		peerCopy := peer
		go func() {
			defer wg.Done()
			send(peerCopy.Ip, peerCopy.Port, content)
		}()
	}
	wg.Wait()
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
func sendWithSocketClient(ip, port string, message []byte) (res string) {
	//log.Printf("Sending message to ip %s and port %s\n", ip, port)
	addr := strings.Join([]string{ip, port}, ":")
	conn, err := net.Dial("tcp", addr)

	if err != nil {
		log.Println(err)
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
