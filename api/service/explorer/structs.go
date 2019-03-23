package explorer

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/harmony-one/harmony/core/types"
)

/*
 * All the code here is work of progress for the sprint.
 */

// Data ...
type Data struct {
	Blocks []*Block `json:"blocks"`
	// Block   Block        `json:"block"`
	Address Address `json:"address"`
	TX      Transaction
}

// Address ...
type Address struct {
	ID      string         `json:"id"`
	Balance *big.Int       `json:"balance"`
	TXs     []*Transaction `json:"txs"`
}

// Transaction ...
type Transaction struct {
	ID        string   `json:"id"`
	Timestamp string   `json:"timestamp"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Value     *big.Int `json:"value"`
	Bytes     string   `json:"bytes"`
	Data      string   `json:"data"`
}

// Block ...
type Block struct {
	Height     string         `json:"height"`
	ID         string         `json:"id"`
	TXCount    string         `json:"txCount"`
	Timestamp  string         `json:"timestamp"`
	MerkleRoot string         `json:"merkleRoot"`
	PrevBlock  RefBlock       `json:"prevBlock"`
	Bytes      string         `json:"bytes"`
	NextBlock  RefBlock       `json:"nextBlock"`
	TXs        []*Transaction `json:"txs"`
}

// RefBlock ...
type RefBlock struct {
	ID     string `json:"id"`
	Height string `json:"height"`
}

// GetTransaction ...
func GetTransaction(tx *types.Transaction, accountBlock *types.Block) *Transaction {
	if tx.To() == nil {
		return nil
	}
	msg, err := tx.AsMessage(types.HomesteadSigner{})
	if err != nil {
		Log.Error("Error when parsing tx into message")
	}
	return &Transaction{
		ID:        tx.Hash().Hex(),
		Timestamp: strconv.Itoa(int(accountBlock.Time().Int64() * 1000)),
		From:      msg.From().Hex(),
		To:        msg.To().Hex(),
		Value:     msg.Value(),
		Bytes:     strconv.Itoa(int(tx.Size())),
		Data:      hex.EncodeToString(tx.Data()),
	}
}
