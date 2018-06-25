package client

import (
	"bytes"
	"encoding/gob"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/log"
)

// A client represent a entity/user which send transactions and receive responses from the harmony network
type Client struct {
	pendingCrossTxs map[[32]byte]*blockchain.Transaction // map of TxId to pending cross shard txs

	log log.Logger // Log utility
}

func (client *Client) TransactionMessageHandler(msgPayload []byte) {
	messageType := TransactionMessageType(msgPayload[0])
	switch messageType {
	case CROSS_TX:
		txDecoder := gob.NewDecoder(bytes.NewReader(msgPayload[1:])) // skip the CROSS_TX messge type

		proofList := new([]blockchain.CrossShardTxProof)
		err := txDecoder.Decode(&proofList)
		if err != nil {
			client.log.Error("Failed deserializing cross transaction proof list")
		}

		// TODO: process the proof list
	}
}

// Create a new Node
func NewClient() *Client {
	client := Client{}

	// Logger
	client.log = log.New()
	return &client
}
