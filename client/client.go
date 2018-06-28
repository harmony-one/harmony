package client

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/log"
	"harmony-benchmark/p2p"
	"sync"
)

// A client represent a entity/user which send transactions and receive responses from the harmony network
type Client struct {
	PendingCrossTxs      map[[32]byte]*blockchain.Transaction // map of TxId to pending cross shard txs
	PendingCrossTxsMutex sync.Mutex
	leaders              *[]p2p.Peer

	UpdateBlocks func([]*blockchain.Block) // func used to sync blocks with the leader

	log log.Logger // Log utility
}

func (client *Client) TransactionMessageHandler(msgPayload []byte) {
	messageType := TransactionMessageType(msgPayload[0])
	switch messageType {
	case PROOF_OF_LOCK:
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the PROOF_OF_LOCK messge type

		proofList := new([]blockchain.CrossShardTxProof)
		err := txDecoder.Decode(&proofList)
		if err != nil {
			client.log.Error("Failed deserializing cross transaction proof list")
		}

		txsToSend := []blockchain.Transaction{}
		client.PendingCrossTxsMutex.Lock()

		for _, proof := range *proofList {

			txAndProofs, ok := client.PendingCrossTxs[proof.TxID]

			readyToUnlock := true
			if ok {
				txAndProofs.Proofs = append(txAndProofs.Proofs, proof)

				txInputs := make(map[blockchain.TXInput]bool)
				for _, curProof := range txAndProofs.Proofs {
					for _, txInput := range curProof.TxInput {
						txInputs[txInput] = true
					}
				}
				for _, txInput := range txAndProofs.TxInput {
					val, ok := txInputs[txInput]
					if !ok || !val {
						readyToUnlock = false
					}
				}
			} else {
				readyToUnlock = false
			}
			if readyToUnlock {
				txsToSend = append(txsToSend, *txAndProofs)
			}
		}
		for _, txToSend := range txsToSend {
			delete(client.PendingCrossTxs, txToSend.ID)
		}
		client.PendingCrossTxsMutex.Unlock()

		if len(txsToSend) != 0 {
			client.sendCrossShardTxUnlockMessage(&txsToSend)
			tempList := []*blockchain.Transaction{}
			for i, _ := range txsToSend {
				tempList = append(tempList, &txsToSend[i])
			}
		}
	}
}

func (client *Client) sendCrossShardTxUnlockMessage(txsToSend *[]blockchain.Transaction) {
	p2p.BroadcastMessage(*client.leaders, ConstructUnlockToCommitOrAbortMessage(*txsToSend))
}

// Create a new Node
func NewClient(leaders *[]p2p.Peer) *Client {
	client := Client{}
	client.PendingCrossTxs = make(map[[32]byte]*blockchain.Transaction)
	client.leaders = leaders

	// Logger
	client.log = log.New()
	return &client
}
