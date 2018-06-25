package client

import (
	"harmony-benchmark/blockchain"
)

// A client represent a entity/user which send transactions and receive responses from the harmony network
type Client struct {
	pendingCrossTxs map[[32]byte]*blockchain.Transaction // map of TxId to pending cross shard txs
}

func (client *Client) TransactionMessageHandler(msgPayload []byte) {
	// TODO: Implement this
}
