package node

import (
	"harmony-benchmark/blockchain"
	"harmony-benchmark/consensus"
	"harmony-benchmark/common"
	"harmony-benchmark/p2p"
	"harmony-benchmark/log"
	"net"
	"os"
	"time"
	"bytes"
	"encoding/gob"
)

// A node represents a program (machine) participating in the network
type Node struct {
	consensus           *consensus.Consensus
	BlockChannel        chan blockchain.Block
	pendingTransactions []blockchain.Transaction
	log log.Logger
}

// Start a server and process the request by a handler.
func (node *Node) StartServer(port string) {
	node.listenOnPort(port)
}

func (node *Node)listenOnPort(port string) {
	listen, err := net.Listen("tcp4", ":"+port)
	defer listen.Close()
	if err != nil {
		node.log.Crit("Socket listen port failed", "port", port, "err", err)
		os.Exit(1)
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			node.log.Crit("Error listening on port. Exiting.", "port", port)
			continue
		}
		go node.NodeHandler(conn)
	}
}

// Handler of the leader node.
func (node *Node) NodeHandler(conn net.Conn) {
	defer conn.Close()

	// Read p2p message payload
	content, err := p2p.ReadMessageContent(conn)

	consensus := node.consensus
	if err != nil {
		node.log.Error("Read p2p data failed", "err", err)
		return
	}

	msgCategory, err := common.GetMessageCategory(content)
	if err != nil {
		node.log.Error("Read node type failed", "err", err)
		return
	}

	msgType, err := common.GetMessageType(content)
	if err != nil {
		node.log.Error("Read action type failed", "err", err)
		return
	}

	msgPayload, err := common.GetMessagePayload(content)
	if err != nil {
		node.log.Error("Read message payload failed", "err", err)
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
			node.transactionMessageHandler(msgPayload)
		case common.CONTROL:
			controlType := msgPayload[0]
			if ControlMessageType(controlType) == STOP {
				node.log.Debug("Stopping Node")
				os.Exit(0)
			}

		}
	}
}

func (node *Node) transactionMessageHandler(msgPayload []byte) {
	txMessageType := TransactionMessageType(msgPayload[0])

	switch txMessageType {
	case SEND:
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the SEND messge type

		txList := new([]blockchain.Transaction)
		err := txDecoder.Decode(&txList)
		if err != nil {
			node.log.Error("Failed deserializing transaction list")
		}
		node.pendingTransactions = append(node.pendingTransactions, *txList...)
	case REQUEST:
		reader := bytes.NewBuffer(msgPayload[1:])
		var txIds map[[32]byte]bool
		txId := make([]byte, 32) // 32 byte hash Id
		for {
			_, err := reader.Read(txId)
			if err != nil {
				break
			}

			txIds[getFixedByteTxId(txId)] = true
		}

		var txToReturn []blockchain.Transaction
		for _, tx := range node.pendingTransactions {
			if txIds[getFixedByteTxId(tx.ID)] {
				txToReturn = append(txToReturn, tx)
			}
		}

		// TODO: return the transaction list to requester
	}
}

// Copy the txId byte slice over to 32 byte array so the map can key on it
func getFixedByteTxId(txId []byte) [32]byte {
	var id [32]byte
	for i := range id {
		id[i] = txId[i]
	}
	return id
}

func (node *Node) WaitForConsensusReady(readySignal chan int) {
	for { // keep waiting for consensus ready
		<-readySignal
		// create a new block
		newBlock := new(blockchain.Block)
		for {
			if len(node.pendingTransactions) >= 10 {
				node.log.Debug("Creating new block")
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
	node.log = node.consensus.Log
	return node
}