package eth

import (
	"math/big"
	"testing"

	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/stretchr/testify/require"
)

func asInt(i any) *big.Int {
	if s, ok := i.(hexutil.Big); ok {
		return s.ToInt()
	}
	panic("<failed to convert>")
}

func TestNewReceipt(t *testing.T) {
	var (
		senderAddr         = common.Address{}
		tx                 = &types.EthTransaction{}
		blockHash          = common2.Hash{}
		blockNumber uint64 = 1
		blockIndex  uint64 = 1
		receipt            = &types.Receipt{}
		r                  = make(map[string]interface{})
		err         error
	)

	t.Run("effectiveGasPrice", func(t *testing.T) {
		r, err = NewReceipt(common2.Address(senderAddr), tx, blockHash, blockNumber, blockIndex, receipt)
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(types.DefaultEffectiveGasPrice), asInt(r["effectiveGasPrice"]))

		rec := &types.Receipt{
			EffectiveGasPrice: big.NewInt(1),
		}
		r, err = NewReceipt(common2.Address(senderAddr), tx, blockHash, blockNumber, blockIndex, rec)
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(1), asInt(r["effectiveGasPrice"]))
	})
}
