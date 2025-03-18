package eth

import (
	"fmt"
	"math/big"
	"testing"

	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/stretchr/testify/require"
)

func asString(i any) string {
	if s, ok := i.(hexutil.Big); ok {
		return s.String()
	}
	return "<failed to convert>"
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
		require.EqualValues(t, fmt.Sprintf("0x%x", types.DefaultEffectiveGasPrice), asString(r["effectiveGasPrice"]))

		rec := &types.Receipt{
			EffectiveGasPrice: big.NewInt(1),
		}
		r, err = NewReceipt(common2.Address(senderAddr), tx, blockHash, blockNumber, blockIndex, rec)
		require.NoError(t, err)
		require.EqualValues(t, "0x1", asString(r["effectiveGasPrice"]))
	})
}
