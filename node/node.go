package node

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/consensus"
	"harmony-benchmark/message"
	"harmony-benchmark/p2p"
	"log"
	"net"
	"os"
	"time"
)

// A node represents a program (machine) participating in the network
type Node struct {
	consensus           *consensus.Consensus
	BlockChannel        chan blockchain.Block
	pendingTransactions []blockchain.Transaction
}

// Start a server and process the request by a handler.
func (node *Node) StartServer(port string) {
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
		node.Logf("Read p2p data failed:%s", err)
		return
	}

	msgCategory, err := message.GetMessageCategory(content)
	if err != nil {
		node.Logf("Read node type failed:%s", err)
		return
	}

	msgType, err := message.GetMessageType(content)
	if err != nil {
		node.Logf("Read action type failed:%s", err)
		return
	}

	msgPayload, err := message.GetMessagePayload(content)
	if err != nil {
		node.Logf("Read message payload failed:%s", err)
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
				node.Logln("Failed deserializing transaction list")
			}
			node.pendingTransactions = append(node.pendingTransactions, *txList...)
			node.Logln(len(node.pendingTransactions))
		case message.CONTROL:
			controlType := msgPayload[0]
			if ControlMessageType(controlType) == STOP {
				node.Logln("Stopping Node")
				os.Exit(0)
			}

		}
	}
}

func (node *Node) WaitForConsensusReady(readySignal chan int) {
	for { // keep waiting for consensus ready
		<-readySignal
		// create a new block
		newBlock := new(blockchain.Block)
		for {
			if len(node.pendingTransactions) >= 10 {
				node.Logln("Creating new block")
				// TODO (Minh): package actual transactions
				// For now, just take out 10 transactions
				var txList []*blockchain.Transaction
				for _, tx := range node.pendingTransactions[0:10] {
					txList = append(txList, &tx)
				}
				node.pendingTransactions = node.pendingTransactions[10:]
				newBlock = blockchain.NewBlock(txList, []byte{})
				break
			}
			time.Sleep(1 * time.Second) // Periodically check whether we have enough transactions to package into block.
		}
		node.BlockChannel <- *newBlock
	}
}

// Create a new Node
func NewNode(consensus *consensus.Consensus) Node {
	node := Node{}
	node.consensus = consensus
	node.BlockChannel = make(chan blockchain.Block)
	return node
}

// Returns the identity string of this node
func (node *Node) GetIdentityString() string {
	return node.consensus.GetIdentityString()
}

// Prints log with ID of this node
func (node *Node) Logln(v ...interface{}) {
	node.consensus.Logln(v...)
}

// Prints formatted log with ID of this node
func (node *Node) Logf(format string, v ...interface{}) {
	node.consensus.Logf(format, v...)
}
