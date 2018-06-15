package node

import (
	"log"
	"net"
	"os"
	"harmony-benchmark/p2p"
	"harmony-benchmark/consensus"
	"harmony-benchmark/message"
	"harmony-benchmark/blockchain"
	"bytes"
	"encoding/gob"
)

// A node represents a program (machine) participating in the network
type Node struct {
	consensus *consensus.Consensus
	pendingTransactions []blockchain.Transaction
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
	content, err := p2p.ReadMessageContent(conn)

	consensus := node.consensus
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read p2p data failed:%s", err)
		} else {
			log.Printf("[Slave] Read p2p data failed:%s", err)
		}
		return
	}

	msgCategory, err := message.GetMessageCategory(content)
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read node type failed:%s", err)
		} else {
			log.Printf("[Slave] Read node type failed:%s", err)
		}
		return
	}

	msgType, err := message.GetMessageType(content)
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read action type failed:%s", err)
		} else {
			log.Printf("[Slave] Read action type failed:%s", err)
		}
		return
	}

	msgPayload, err := message.GetMessagePayload(content)
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
			txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the SEND messge type

			txList := new([]blockchain.Transaction)
			err := txDecoder.Decode(&txList)
			if err != nil {
				log.Println("Failed deserializing transaction list")
			}
			node.pendingTransactions = append(node.pendingTransactions, *txList...)
			log.Println(len(node.pendingTransactions))
		}
	}
}

// Create a new Node
func NewNode(consensus *consensus.Consensus) Node {
	node := Node{}
	node.consensus = consensus
	return node
}