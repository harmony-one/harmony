package p2p

import (
	"strings"
	"net"
	"log"
	"fmt"
)

// Object for a p2p peer (node)
type Peer struct {
	// Ip address of the peer
	Ip   string
	// Port number of the peer
	Port string
	// Public key of the peer
	PubKey string
}

// Send the message to the peer
func SendMessage(peer Peer, msg string){
	send(peer.Ip, peer.Port, msg)
}

func BroadcastMessage(peers []Peer, msg string) {
	fmt.Println(peers)
	for _, peer := range peers {
		SendMessage(peer, msg)
	}
}


// SocketClient is to connect a socket given a port and send the given message.
func sendWithSocketClient(ip, port, message string) (res string) {
	fmt.Printf("Sending message to ip %s and port %s\n", ip, port)
	addr := strings.Join([]string{ip, port}, ":")
	conn, err := net.Dial("tcp", addr)


	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	conn.Write([]byte(message))
	log.Printf("Sent to ip %s and port %s: %s\n", ip, port, message)

	buff := make([]byte, 1024)
	n, _ := conn.Read(buff) // do we need this?
	res = string(buff[:n])
	return
}

// Send a message to another node with given port.
func send(ip, port, message string) (returnMessage string) {
	returnMessage = sendWithSocketClient(ip, port, message)
	fmt.Println(returnMessage)
	return
}