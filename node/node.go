package node

import (
	"log"
	"net"
	"os"
	"harmony-benchmark/p2p"
	"harmony-benchmark/consensus"
	"harmony-benchmark/message"
)

// A node represents a program (machine) participating in the network
type Node struct {
	consensus *consensus.Consensus
}

// Start a server and process the request by a handler.
func (node Node) StartServer(port string) {
	listenOnPort(port, node.NodeHandler)
}

func listenOnPort(port string, handler func(net.Conn)) {
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
		go handler(conn)
	}
}

// Handler of the leader node.
func (node *Node) NodeHandler(conn net.Conn) {
	defer conn.Close()

	// Read p2p message payload
	payload, err := p2p.ReadMessageContent(conn)

	consensus := node.consensus
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read p2p data failed:%s", err)
		} else {
			log.Printf("[Slave] Read p2p data failed:%s", err)
		}
		return
	}

	msgCategory, err := message.GetMessageCategory(payload)
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read node type failed:%s", err)
		} else {
			log.Printf("[Slave] Read node type failed:%s", err)
		}
		return
	}

	msgType, err := message.GetMessageType(payload)
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read action type failed:%s", err)
		} else {
			log.Printf("[Slave] Read action type failed:%s", err)
		}
		return
	}

	msgPayload, err := message.GetMessagePayload(payload)
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read message payload failed:%s", err)
		} else {
			log.Printf("[Slave] Read message payload failed:%s", err)
		}
		return
	}

	switch msgCategory {
	case message.COMMITTEE:
		actionType := message.CommitteeMessageType(msgType)
		switch actionType {
		case message.CONSENSUS:
			if consensus.IsLeader {
				consensus.ProcessMessageLeader(msgPayload)
			} else {
				consensus.ProcessMessageValidator(msgPayload)
			}
		}
	case message.NODE:
		actionType := message.NodeMessageType(msgType)
		switch actionType {
		case message.TRANSACTION:
			// TODO: process transaction
		}
	}
}

// Create a new Node
func NewNode(consensus *consensus.Consensus) Node {
	node := Node{}
	node.consensus = consensus
	return node
}