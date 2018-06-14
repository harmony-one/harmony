package node

import (
	"log"
	"net"
	"os"
	"harmony-benchmark/consensus"
	"harmony-benchmark/p2p"
)

// A node represents a program (machine) participating in the network
type Node struct {
	nodeType NodeType
}

type NodeType int

const (
	COMMITTEE NodeType = iota
	// TODO: add more types
)


// Start a server and process the request by a handler.
func (node Node) StartServer(port string, consensus *consensus.Consensus) {
	listenOnPort(port, NodeHandler, consensus)
}

func listenOnPort(port string, handler func(net.Conn, *consensus.Consensus), consensus *consensus.Consensus) {
	listen, err := net.Listen("tcp4", ":"+port)
	defer listen.Close()
	if err != nil {
		log.Fatalf("Socket listen port %s failed,%s", port, err)
		os.Exit(1)
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Printf("Error listening on port: %s. Exiting.", port)
			log.Fatalln(err)
			continue
		}
		go handler(conn, consensus)
	}
}

// Handler of the leader node.
func NodeHandler(conn net.Conn, consensus *consensus.Consensus) {
	defer conn.Close()

	payload, err := p2p.ReadMessagePayload(conn)
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Receive data failed:%s", err)
		} else {
			log.Printf("[Slave] Receive data failed:%s", err)
		}
		return
	}
	if consensus.IsLeader {
		consensus.ProcessMessageLeader(payload)
	} else {
		consensus.ProcessMessageValidator(payload)
	}
	//relayToPorts(receivedMessage, conn)
}
