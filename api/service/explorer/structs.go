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
	Cross    = "CROSS"
)

// Data ...
type Data struct {
	Addresses []string `json:"Addresses"`
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
	GasFee    *big.Int `json:"gasFee"`
	FromShard uint32   `json:"fromShard"`
	ToShard   uint32   `json:"toShard"`
	Type      string   `json:"type"`
}

// AddrBalance ...
type AddrBalance struct {
	Address string   `json:"address"`
	Balance *big.Int `json:"balance"`
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
