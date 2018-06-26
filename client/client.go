package client

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/log"
	"harmony-benchmark/p2p"
	"sync"
)

// A client represent a entity/user which send transactions and receive responses from the harmony network
type Client struct {
	PendingCrossTxs      map[[32]byte]*CrossShardTxAndProofs // map of TxId to pending cross shard txs
	PendingCrossTxsMutex sync.Mutex
	leaders              *[]p2p.Peer

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

		txsToSend := []CrossShardTxAndProofs{}
		client.PendingCrossTxsMutex.Lock()
		for _, proof := range *proofList {

			txAndProofs, ok := client.PendingCrossTxs[proof.TxID]

			readyToUnlock := true
			if ok {
				txAndProofs.Proofs = append(txAndProofs.Proofs, proof)
				txInputs := make(map[blockchain.TXInput]bool)
				for _, txInput := range txAndProofs.Transaction.TxInput {
					txInputs[txInput] = true
				}
				for _, curProof := range txAndProofs.Proofs {
					for _, txInput := range curProof.TxInput {
						val, ok := txInputs[*txInput]
						if !ok || !val {
							readyToUnlock = false
						}
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
			delete(client.PendingCrossTxs, txToSend.Transaction.ID)
		}
		client.PendingCrossTxsMutex.Unlock()

		if len(txsToSend) != 0 {

			client.log.Debug("SENDING UNLOCK MESSAGE")
			for _, val := range txsToSend {
				client.log.Debug("sending---", "txAndProof", val)
			}
			client.sendCrossShardTxUnlockMessage(&txsToSend)
		}
	}
}

func (client *Client) sendCrossShardTxUnlockMessage(txsToSend *[]CrossShardTxAndProofs) {
	// TODO: Send unlock message back to output shards
	fmt.Println("SENDING UNLOCK MESSAGE")
	p2p.BroadcastMessage(*client.leaders, ConstructUnlockToCommitOrAbortMessage(*txsToSend))
}

// Create a new Node
func NewClient(leaders *[]p2p.Peer) *Client {
	client := Client{}
	client.PendingCrossTxs = make(map[[32]byte]*CrossShardTxAndProofs)
	client.leaders = leaders

	// Logger
	client.log = log.New()
	return &client
}
