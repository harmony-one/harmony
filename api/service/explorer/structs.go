package explorer

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
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

// Committee ...
type Committee struct {
	Validators []*Validator `json:"validators"`
}

// Validator ...
type Validator struct {
	Address string   `json:"address"`
	Balance *big.Int `json:"balance"`
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
	BlockTime  int64          `json:"blockTime"`
	MerkleRoot string         `json:"merkleRoot"`
	PrevBlock  RefBlock       `json:"prevBlock"`
	Bytes      string         `json:"bytes"`
	NextBlock  RefBlock       `json:"nextBlock"`
	TXs        []*Transaction `json:"txs"`
	Signers    []string       `json:"signers"`
	ExtraData  string         `json:"extra_data"`
}

// RefBlock ...
type RefBlock struct {
	ID     string `json:"id"`
	Height string `json:"height"`
}

// Node ...
type Node struct {
	ID string `json:"id"`
}

// Shard ...
type Shard struct {
	Nodes []Node `json:"nodes"`
}

// NewBlock ...
func NewBlock(block *types.Block, height int) *Block {
	// TODO(ricl): use block.Header().CommitBitmap and GetPubKeyFromMask
	return &Block{
		Height:     strconv.Itoa(height),
		ID:         block.Hash().Hex(),
		TXCount:    strconv.Itoa(block.Transactions().Len()),
		Timestamp:  strconv.Itoa(int(block.Time().Int64() * 1000)),
		MerkleRoot: block.Root().Hex(),
		Bytes:      strconv.Itoa(int(block.Size())),
		Signers:    []string{},
		ExtraData:  string(block.Extra()),
	}
}

// GetTransaction ...
func GetTransaction(tx *types.Transaction, accountBlock *types.Block) *Transaction {
	if tx.To() == nil {
		return nil
	}
	msg, err := tx.AsMessage(types.HomesteadSigner{})
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Error when parsing tx into message")
	}
	return &Transaction{
		ID:        tx.Hash().Hex(),
		Timestamp: strconv.Itoa(int(accountBlock.Time().Int64() * 1000)),
		From:      msg.From().Hex(), // TODO ek – use bech32
		To:        msg.To().Hex(),   // TODO ek – use bech32
		Value:     msg.Value(),
		Bytes:     strconv.Itoa(int(tx.Size())),
		Data:      hex.EncodeToString(tx.Data()),
	}
}
