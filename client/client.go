package client

import (
	"bytes"
	"encoding/gob"
	"sync"

	"github.com/harmony-one/harmony/p2p/host"
	"github.com/harmony-one/harmony/proto/node"

	"github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/log"
	"github.com/harmony-one/harmony/p2p"
	client_proto "github.com/harmony-one/harmony/proto/client"
)

// Client represents a node (e.g. a wallet) which  sends transactions and receives responses from the harmony network
type Client struct {
	PendingCrossTxs      map[[32]byte]*blockchain.Transaction // Map of TxId to pending cross shard txs. Pending means the proof-of-accept/rejects are not complete
	PendingCrossTxsMutex sync.Mutex                           // Mutex for the pending txs list
	Leaders              *map[uint32]p2p.Peer                 // Map of shard Id and corresponding leader
	UpdateBlocks         func([]*blockchain.Block)            // Closure function used to sync new block with the leader. Once the leader finishes the consensus on a new block, it will send it to the clients. Clients use this method to update their blockchain

	ShardUtxoMap      map[uint32]blockchain.UtxoMap
	ShardUtxoMapMutex sync.Mutex // Mutex for the UTXO maps
	log               log.Logger // Log utility

	// The p2p host used to send/receive p2p messages
	host host.Host
}

// TransactionMessageHandler is the message handler for Client/Transaction messages.
func (client *Client) TransactionMessageHandler(msgPayload []byte) {
	messageType := client_proto.TransactionMessageType(msgPayload[0])
	switch messageType {
	case client_proto.ProofOfLock:
		// Decode the list of blockchain.CrossShardTxProof
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the ProofOfLock messge type
		proofs := new([]blockchain.CrossShardTxProof)
		err := txDecoder.Decode(proofs)

		if err != nil {
			client.log.Error("Failed deserializing cross transaction proof list")
		}
		client.handleProofOfLockMessage(proofs)
	case client_proto.UtxoResponse:
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the ProofOfLock messge type
		fetchUtxoResponse := new(client_proto.FetchUtxoResponseMessage)
		err := txDecoder.Decode(fetchUtxoResponse)
		client.log.Debug("UtxoResponse")

		if err != nil {
			client.log.Error("Failed deserializing utxo response")
		}
		client.handleFetchUtxoResponseMessage(*fetchUtxoResponse)
	}
}

// handleProofOfLockMessage handles the followings:
// Client once receives a list of proofs from a leader, for each proof:
// 1) retreive the pending cross shard transaction
// 2) add the proof to the transaction
// 3) checks whether all input utxos of the transaction have a corresponding proof.
// 4) for all transactions with full proofs, broadcast them back to the leaders
func (client *Client) handleProofOfLockMessage(proofs *[]blockchain.CrossShardTxProof) {
	txsToSend := []*blockchain.Transaction{}

	//fmt.Printf("PENDING CLIENT TX - %d\n", len(client.PendingCrossTxs))
	// Loop through the newly received list of proofs
	client.PendingCrossTxsMutex.Lock()
	log.Info("CLIENT PENDING CROSS TX", "num", len(client.PendingCrossTxs))
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
			txsToSend = append(txsToSend, txAndProofs)
		}
	}

	// Delete all the transactions with full proofs from the pending cross txs
	for _, txToSend := range txsToSend {
		delete(client.PendingCrossTxs, txToSend.ID)
	}
	client.PendingCrossTxsMutex.Unlock()

	// Broadcast the cross txs with full proofs for unlock-to-commit/abort
	if len(txsToSend) != 0 {
		client.sendCrossShardTxUnlockMessage(txsToSend)
	}
}

func (client *Client) handleFetchUtxoResponseMessage(utxoResponse client_proto.FetchUtxoResponseMessage) {
	client.ShardUtxoMapMutex.Lock()
	defer client.ShardUtxoMapMutex.Unlock()
	_, ok := client.ShardUtxoMap[utxoResponse.ShardID]
	if ok {
		return
	}
	client.ShardUtxoMap[utxoResponse.ShardID] = utxoResponse.UtxoMap
}

func (client *Client) sendCrossShardTxUnlockMessage(txsToSend []*blockchain.Transaction) {
	for shardID, txs := range BuildOutputShardTransactionMap(txsToSend) {
		host.SendMessage(client.host, (*client.Leaders)[shardID], node.ConstructUnlockToCommitOrAbortMessage(txs), nil)
	}
}

// NewClient creates a new Client
func NewClient(host host.Host, leaders *map[uint32]p2p.Peer) *Client {
	client := Client{}
	client.PendingCrossTxs = make(map[[32]byte]*blockchain.Transaction)
	client.Leaders = leaders
	client.host = host
	// Logger
	client.log = log.New()
	return &client
}

// BuildOutputShardTransactionMap builds output shard transaction map.
func BuildOutputShardTransactionMap(txs []*blockchain.Transaction) map[uint32][]*blockchain.Transaction {
	txsShardMap := make(map[uint32][]*blockchain.Transaction)

	// Put txs into corresponding output shards
	for _, crossTx := range txs {
		for curShardID := range GetOutputShardIDsOfCrossShardTx(crossTx) {
			txsShardMap[curShardID] = append(txsShardMap[curShardID], crossTx)
		}
	}
	return txsShardMap
}

// GetInputShardIDsOfCrossShardTx gets input shardID.
func GetInputShardIDsOfCrossShardTx(crossTx *blockchain.Transaction) map[uint32]bool {
	shardIDs := map[uint32]bool{}
	for _, txInput := range crossTx.TxInput {
		shardIDs[txInput.ShardID] = true
	}
	return shardIDs
}

// GetOutputShardIDsOfCrossShardTx gets output shard ids.
func GetOutputShardIDsOfCrossShardTx(crossTx *blockchain.Transaction) map[uint32]bool {
	shardIDs := map[uint32]bool{}
	for _, txOutput := range crossTx.TxOutput {
		shardIDs[txOutput.ShardID] = true
	}
	return shardIDs
}

// GetLeaders returns leader peers.
func (client *Client) GetLeaders() []p2p.Peer {
	leaders := []p2p.Peer{}
	for _, leader := range *client.Leaders {
		leaders = append(leaders, leader)
	}
	return leaders
}

//// GetLeaders returns leader peers.
//func (client *Client) GetShardLeader(uint32 shardID) p2p.Peer {
//	leaders := []p2p.Peer{}
//	for _, leader := range *client.Leaders {
//		leaders = append(leaders, leader)
//	}
//	return leaders
//}
