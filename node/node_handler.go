package node

import (
	"bytes"
	"harmony-benchmark/blockchain"
	"time"
	"net"
	"harmony-benchmark/p2p"
	"harmony-benchmark/common"
	"os"
	"encoding/gob"
)

// Handler of the leader node.
func (node *Node) NodeHandler(conn net.Conn) {
	defer conn.Close()

	// Read p2p message payload
	content, err := p2p.ReadMessageContent(conn)

	consensus := node.consensus
	if err != nil {
		node.log.Error("Read p2p data failed", "err", err, "node", node)
		return
	}

	msgCategory, err := common.GetMessageCategory(content)
	if err != nil {
		node.log.Error("Read node type failed", "err", err, "node", node)
		return
	}

	msgType, err := common.GetMessageType(content)
	if err != nil {
		node.log.Error("Read action type failed", "err", err, "node", node)
		return
	}

	msgPayload, err := common.GetMessagePayload(content)
	if err != nil {
		node.log.Error("Read message payload failed", "err", err, "node", node)
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
				node.log.Debug("Stopping Node", "node", node)
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

		txList := new([]*blockchain.Transaction)
		err := txDecoder.Decode(&txList)
		if err != nil {
			node.log.Error("Failed deserializing transaction list", "node", node)
		}
		node.addPendingTransactions(*txList)
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

		var txToReturn []*blockchain.Transaction
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
	node.log.Debug("Waiting for consensus ready", "node", node)

	for { // keep waiting for consensus ready
		<-readySignal
		// create a new block
		newBlock := new(blockchain.Block)
		for {
			if len(node.pendingTransactions) >= 100 {
				selectedTxs := node.getTransactionsForNewBlock()
				if len(selectedTxs) == 0 {
					node.log.Debug("No transactions is selected for consensus", "pendingTx", len(node.pendingTransactions))
				} else {
					node.log.Debug("Creating new block", "node", node)
					newBlock = blockchain.NewBlock(selectedTxs, []byte{})
					node.log.Debug("Num of Pending Tx", "count", len(node.pendingTransactions))
					break
				}
			}
			time.Sleep(1 * time.Second) // Periodically check whether we have enough transactions to package into block.
		}
		node.BlockChannel <- *newBlock
	}
}