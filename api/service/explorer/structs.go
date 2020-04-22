package explorer

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
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
	Addresses []string `json:"Addresses"`
}

// Address ...
type Address struct {
	ID         string         `json:"id"`
	Balance    *big.Int       `json:"balance"`
	TXs        []*Transaction `json:"txs"`
	StakingTXs []*Transaction `json:"staking_txs"`
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

// GetTransaction ...
func GetTransaction(tx *types.Transaction, addressBlock *types.Block) (*Transaction, error) {
	fromAddr, err := types.Sender(types.NewEIP155Signer(tx.ChainID()), tx)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Error when retrieving the tx sender")
	}
	gasFee := big.NewInt(0)
	gasFee = gasFee.Mul(tx.GasPrice(), new(big.Int).SetUint64(tx.Gas()))
	to := ""
	txTo, err := tx.To()
	if err != nil {
		return nil, err
	}
	if txTo != nil {
		if to, err = common2.AddressToBech32(*txTo); err != nil {
			return nil, err
		}
	}
	from := ""
	if from, err = common2.AddressToBech32(fromAddr); err != nil {
		return nil, err
	}
	shardID, err := tx.ShardID()
	if err != nil {
		return nil, err
	}
	toShardID, err := tx.ToShardID()
	if err != nil {
		return nil, err
	}
	data, err := tx.Data()
	if err != nil {
		return nil, err
	}
	value, err := tx.Value()
	if err != nil {
		return nil, err
	}
	return &Transaction{
		ID:        tx.Hash().Hex(),
		Timestamp: strconv.Itoa(int(addressBlock.Time().Int64() * 1000)),
		From:      from,
		To:        to,
		Value:     value,
		Bytes:     strconv.Itoa(int(tx.Size())),
		Data:      hex.EncodeToString(data),
		GasFee:    gasFee,
		FromShard: shardID,
		ToShard:   toShardID,
		Type:      "",
	}, nil
}
