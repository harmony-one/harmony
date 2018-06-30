package client

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/log"
	"harmony-benchmark/p2p"
	"sync"
)

// A client represent a node (e.g. wallet) which  sends transactions and receive responses from the harmony network
type Client struct {
	PendingCrossTxs      map[[32]byte]*blockchain.Transaction // Map of TxId to pending cross shard txs. Pending means the proof-of-accept/rejects are not complete
	PendingCrossTxsMutex sync.Mutex                           // Mutex for the pending txs list
	leaders              *[]p2p.Peer                          // All the leaders for each shard
	UpdateBlocks         func([]*blockchain.Block)            // Closure function used to sync new block with the leader. Once the leader finishes the consensus on a new block, it will send it to the clients. Clients use this method to update their blockchain

	log log.Logger // Log utility
}

// The message handler for CLIENT/TRANSACTION messages.
func (client *Client) TransactionMessageHandler(msgPayload []byte) {
	messageType := TransactionMessageType(msgPayload[0])
	switch messageType {
	case PROOF_OF_LOCK:
		// Decode the list of blockchain.CrossShardTxProof
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the PROOF_OF_LOCK messge type
		proofList := new([]blockchain.CrossShardTxProof)
		err := txDecoder.Decode(&proofList)

		if err != nil {
			client.log.Error("Failed deserializing cross transaction proof list")
		}

		txsToSend := []blockchain.Transaction{}

		// Loop through the newly received list of proofs
		client.PendingCrossTxsMutex.Lock()
		for _, proof := range *proofList {

			// Find the corresponding pending cross tx
			txAndProofs, ok := client.PendingCrossTxs[proof.TxID]

			readyToUnlock := true // A flag used to mark whether whether this pending cross tx have all the proofs for its utxo input
			if ok {
				// Add the new proof to the cross tx's proof list
				txAndProofs.Proofs = append(txAndProofs.Proofs, proof)

				// Check whether this pending cross tx have all the proofs for its utxo input
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

		// Delete all the transactions with full proofs from the pending cross txs
		for _, txToSend := range txsToSend {
			delete(client.PendingCrossTxs, txToSend.ID)
		}
		client.PendingCrossTxsMutex.Unlock()

		// Broadcast the cross txs with full proofs for unlock-to-commit/abort
		if len(txsToSend) != 0 {
			client.broadcastCrossShardTxUnlockMessage(&txsToSend)
		}
	}
}

func (client *Client) broadcastCrossShardTxUnlockMessage(txsToSend *[]blockchain.Transaction) {
	p2p.BroadcastMessage(*client.leaders, ConstructUnlockToCommitOrAbortMessage(*txsToSend))
}

// Create a new Cient
func NewClient(leaders *[]p2p.Peer) *Client {
	client := Client{}
	client.PendingCrossTxs = make(map[[32]byte]*blockchain.Transaction)
	client.leaders = leaders

	// Logger
	client.log = log.New()
	return &client
}
