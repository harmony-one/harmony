package explorer

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
)

// Test for GetBlockInfoKey
func TestGetTransaction(t *testing.T) {
	tx1 := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	tx2 := types.NewTransaction(2, common.BytesToAddress([]byte{0x22}), 0, big.NewInt(222), 2222, big.NewInt(22222), []byte{0x22, 0x22, 0x22})
	tx3 := types.NewTransaction(3, common.BytesToAddress([]byte{0x33}), 0, big.NewInt(333), 3333, big.NewInt(33333), []byte{0x33, 0x33, 0x33})
	txs := []*types.Transaction{tx1, tx2, tx3}

	block := types.NewBlock(&types.Header{Number: big.NewInt(314)}, txs, nil, nil, nil)

	tx := GetTransaction(tx1, block)
	assert.Equal(t, tx.ID, tx1.Hash().Hex(), "should be equal tx1.Hash()")
	assert.Equal(t, tx.To, tx1.To().Hex(), "should be equal tx1.To()") // TODO ek – use bech32
	assert.Equal(t, tx.Bytes, strconv.Itoa(int(tx1.Size())), "should be equal tx1.Size()")
}
