package hmyapi

import (
	"bytes"
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/core/types"
)

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendTxArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	ShardID  uint32          `json:"shardID"`
	Gas      uint64          `json:"gas"`
	GasPrice *big.Int        `json:"gasPrice"`
	Value    *big.Int        `json:"value"`
	Nonce    uint64          `json:"nonce"`
	// We accept "data" and "input" for backwards-compatibility reasons. "input" is the
	// newer name and should be preferred by clients.
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`
}

// setDefaults is a helper function that fills in default values for unspecified tx fields.
func (args *SendTxArgs) setDefaults(ctx context.Context, b Backend) error {
	if args.Gas == 0 {
		args.Gas = 90000
	}
	// TODO(ricl): add check for shardID
	if args.GasPrice == nil {
		// TODO(ricl): port
		// price, err := b.SuggestPrice(ctx)
		// if err != nil {
		// 	return err
		// }
		// args.GasPrice = (*hexutil.Big)(price)
	}
	if args.Value == nil {
		args.Value = big.NewInt(0)
	}
	if &args.Nonce == nil {
		nonce, err := b.GetPoolNonce(ctx, args.From)
		if err != nil {
			return err
		}
		args.Nonce = nonce
	}
	if args.Data != nil && args.Input != nil && !bytes.Equal(*args.Data, *args.Input) {
		return errors.New(`both "data" and "input" are set and not equal. Please use "input" to pass transaction call data`)
	}
	if args.To == nil {
		// Contract creation
		var input []byte
		if args.Data != nil {
			input = *args.Data
		} else if args.Input != nil {
			input = *args.Input
		}
		if len(input) == 0 {
			return errors.New(`contract creation without any data provided`)
		}
	}
	return nil
}

func (args *SendTxArgs) toTransaction() *types.Transaction {
	var input []byte
	if args.Data != nil {
		input = *args.Data
	} else if args.Input != nil {
		input = *args.Input
	}
	if args.To == nil {
		return types.NewContractCreation(args.Nonce, args.ShardID, args.Value, args.Gas, args.GasPrice, input)
	}
	return types.NewTransaction(args.Nonce, *args.To, args.ShardID, args.Value, args.Gas, args.GasPrice, input)
}
