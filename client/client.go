package client

import (
	"bytes"
	"encoding/gob"
	"github.com/simple-rules/harmony-benchmark/proto/node"
	"sync"

	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/p2p"
	client_proto "github.com/simple-rules/harmony-benchmark/proto/client"
)

// A client represents a node (e.g. a wallet) which  sends transactions and receives responses from the harmony network
type Client struct {
	PendingCrossTxs      map[[32]byte]*blockchain.Transaction // Map of TxId to pending cross shard txs. Pending means the proof-of-accept/rejects are not complete
	PendingCrossTxsMutex sync.Mutex                           // Mutex for the pending txs list
	leaders              *[]p2p.Peer                          // All the leaders for each shard
	UpdateBlocks         func([]*blockchain.Block)            // Closure function used to sync new block with the leader. Once the leader finishes the consensus on a new block, it will send it to the clients. Clients use this method to update their blockchain

	UtxoMap              blockchain.UtxoMap
	ShardResponseTracker map[uint32]bool // A map containing the shard id of responded shard.
	log                  log.Logger      // Log utility
}

// The message handler for CLIENT/TRANSACTION messages.
func (client *Client) TransactionMessageHandler(msgPayload []byte) {
	messageType := client_proto.TransactionMessageType(msgPayload[0])
	switch messageType {
	case client_proto.PROOF_OF_LOCK:
		// Decode the list of blockchain.CrossShardTxProof
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the PROOF_OF_LOCK messge type
		proofs := new([]blockchain.CrossShardTxProof)
		err := txDecoder.Decode(proofs)

		if err != nil {
			client.log.Error("Failed deserializing cross transaction proof list")
		}
		client.handleProofOfLockMessage(proofs)
	case client_proto.UTXO_RESPONSE:
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the PROOF_OF_LOCK messge type
		fetchUtxoResponse := new(client_proto.FetchUtxoResponseMessage)
		err := txDecoder.Decode(fetchUtxoResponse)

		if err != nil {
			client.log.Error("Failed deserializing utxo resposne")
		}
		client.handleFetchUtxoResponseMessage(*fetchUtxoResponse)
	}
}

// Client once receives a list of proofs from a leader, for each proof:
// 1) retreive the pending cross shard transaction
// 2) add the proof to the transaction
// 3) checks whether all input utxos of the transaction have a corresponding proof.
// 4) for all transactions with full proofs, broadcast them back to the leaders
func (client *Client) handleProofOfLockMessage(proofs *[]blockchain.CrossShardTxProof) {
	txsToSend := []blockchain.Transaction{}

	// Loop through the newly received list of proofs
	client.PendingCrossTxsMutex.Lock()
	for _, proof := range *proofs {
		// Find the corresponding pending cross tx
		txAndProofs, ok := client.PendingCrossTxs[proof.TxID]

		readyToUnlock := true // A flag used to mark whether whether this pending cross tx have all the proofs for its utxo input
		if ok {
			// Add the new proof to the cross tx's proof list
			txAndProofs.Proofs = append(txAndProofs.Proofs, proof)

			// Check whether this pending cross tx have all the proofs for its utxo inputs
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

func (client *Client) handleFetchUtxoResponseMessage(utxoResponse client_proto.FetchUtxoResponseMessage) {
	_, ok := client.ShardResponseTracker[utxoResponse.ShardId]
	if ok {
		return
	}
	client.ShardResponseTracker[utxoResponse.ShardId] = true
	// Merge utxo response into client utxo map.
	for address, txHash2Vout2AmountMap := range utxoResponse.UtxoMap {
		clientTxHashMap, ok := client.UtxoMap[address]
		if ok {
			for txHash, vout2AmountMap := range txHash2Vout2AmountMap {
				clientVout2AmountMap, ok := clientTxHashMap[txHash]
				if ok {
					for vout, amount := range vout2AmountMap {
						clientVout2AmountMap[vout] = amount
					}
				} else {
					clientTxHashMap[txHash] = vout2AmountMap
				}

			}
		} else {
			client.UtxoMap[address] = txHash2Vout2AmountMap
		}
	}
}

func (client *Client) broadcastCrossShardTxUnlockMessage(txsToSend *[]blockchain.Transaction) {
	p2p.BroadcastMessage(*client.leaders, node.ConstructUnlockToCommitOrAbortMessage(*txsToSend))
}

// Create a new Client
func NewClient(leaders *[]p2p.Peer) *Client {
	client := Client{}
	client.PendingCrossTxs = make(map[[32]byte]*blockchain.Transaction)
	client.leaders = leaders

	// Logger
	client.log = log.New()
	return &client
}
