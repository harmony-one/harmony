package node

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/consensus"
	"harmony-benchmark/common"
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
		if consensus.IsLeader {
			log.Printf("[Leader] Read p2p data failed:%s", err)
		} else {
			log.Printf("[Slave] Read p2p data failed:%s", err)
		}
		return
	}

	msgCategory, err := common.GetMessageCategory(content)
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read node type failed:%s", err)
		} else {
			log.Printf("[Slave] Read node type failed:%s", err)
		}
		return
	}

	msgType, err := common.GetMessageType(content)
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read action type failed:%s", err)
		} else {
			log.Printf("[Slave] Read action type failed:%s", err)
		}
		return
	}

	msgPayload, err := common.GetMessagePayload(content)
	if err != nil {
		if consensus.IsLeader {
			log.Printf("[Leader] Read message payload failed:%s", err)
		} else {
			log.Printf("[Slave] Read message payload failed:%s", err)
		}
		return
	}

	switch msgCategory {
	case common.COMMITTEE:
		actionType := common.CommitteeMessageType(msgType)
		switch actionType {
		case common.CONSENSUS:
			if consensus.IsLeader {
				consensus.ProcessMessageLeader(msgPayload)
			} else {
				consensus.ProcessMessageValidator(msgPayload)
			}
		}
	case common.NODE:
		actionType := common.NodeMessageType(msgType)
		switch actionType {
		case common.TRANSACTION:
			txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the SEND messge type

			txList := new([]blockchain.Transaction)
			err := txDecoder.Decode(&txList)
			if err != nil {
				log.Println("Failed deserializing transaction list")
			}
			node.pendingTransactions = append(node.pendingTransactions, *txList...)
			log.Println(len(node.pendingTransactions))
		case common.CONTROL:
			controlType := msgPayload[0]
			if ControlMessageType(controlType) == STOP {
				log.Println("Stopping Node")
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
				log.Println("Creating new block")
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
