package explorer

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
)

/*
 * All the code here is work of progress for the sprint.
 */

// Tx types ...
const (
	Received = "RECEIVED"
	Sent     = "SENT"
)

// Data ...
type Data struct {
	Blocks []*Block `json:"blocks"`
	// Block   Block        `json:"block"`
	Address Address `json:"Address"`
	TX      Transaction
}

// Address ...
type Address struct {
	ID      string         `json:"id"`
	Balance *big.Int       `json:"balance"`
	TXs     []*Transaction `json:"txs"`
}

// Committee contains list of node validators of a particular shard and epoch.
type Committee struct {
	Validators []*Validator `json:"validators"`
}

// Validator contains harmony validator node address and its balance.
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
	GasFee    *big.Int `json:"gasFee"`
	FromShard uint32   `json:"fromShard"`
	ToShard   uint32   `json:"toShard"`
	Type      string   `json:"type"`
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
	Epoch      uint64         `json:"epoch"`
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
		Epoch:      block.Epoch().Uint64(),
		ExtraData:  string(block.Extra()),
	}
}

// GetTransaction ...
func GetTransaction(tx *types.Transaction, addressBlock *types.Block) *Transaction {
	msg, err := tx.AsMessage(types.NewEIP155Signer(tx.ChainID()))
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Error when parsing tx into message")
	}
	gasFee := big.NewInt(0)
	gasFee = gasFee.Mul(tx.GasPrice(), new(big.Int).SetUint64(tx.Gas()))
	to := ""
	var err error
	if msg.To() != nil {
		if to, err = common.AddressToBech32(*msg.To()); err != nil {
			return nil			
		}
	}
	from := ""
	if from, err = common.AddressToBech32(msg.From()); err != nil {
		return nil
	}
	return &Transaction{
		ID:        tx.Hash().Hex(),
		Timestamp: strconv.Itoa(int(addressBlock.Time().Int64() * 1000)),
		From:      from,
		To:        to,
		Value:     msg.Value(),
		Bytes:     strconv.Itoa(int(tx.Size())),
		Data:      hex.EncodeToString(tx.Data()),
		GasFee:    gasFee,
		FromShard: tx.ShardID(),
		ToShard:   tx.ToShardID(),
		Type:      "",
	}
}
