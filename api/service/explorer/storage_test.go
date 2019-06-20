package explorer

import (
	"bytes"
	"math/big"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/stretchr/testify/assert"
)

// Test for GetBlockInfoKey
func TestGetBlockInfoKey(t *testing.T) {
	assert.Equal(t, GetBlockInfoKey(3), "bi_3", "error")
}

// Test for GetAddressKey
func TestGetAddressKey(t *testing.T) {
	assert.Equal(t, GetAddressKey("abcd"), "ad_abcd", "error")
}

// Test for GetBlockKey
func TestGetBlockKey(t *testing.T) {
	assert.Equal(t, GetBlockKey(3), "b_3", "error")
}

// Test for GetTXKey
func TestGetTXKey(t *testing.T) {
	assert.Equal(t, GetTXKey("abcd"), "tx_abcd", "error")
}

func TestInit(t *testing.T) {
	ins := GetStorageInstance("1.1.1.1", "3333", true)
	if err := ins.GetDB().Put([]byte{1}, []byte{2}); err != nil {
		t.Fatal("(*LDBDatabase).Put failed:", err)
	}
	value, err := ins.GetDB().Get([]byte{1})
	assert.Equal(t, bytes.Compare(value, []byte{2}), 0, "value should be []byte{2}")
	assert.Nil(t, err, "error should be nil")
}

func TestDump(t *testing.T) {
	tx1 := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	tx2 := types.NewTransaction(2, common.BytesToAddress([]byte{0x22}), 0, big.NewInt(222), 2222, big.NewInt(22222), []byte{0x22, 0x22, 0x22})
	tx3 := types.NewTransaction(3, common.BytesToAddress([]byte{0x33}), 0, big.NewInt(333), 3333, big.NewInt(33333), []byte{0x33, 0x33, 0x33})
	txs := []*types.Transaction{tx1, tx2, tx3}

	block := types.NewBlock(&types.Header{Number: big.NewInt(314)}, txs, nil)
	ins := GetStorageInstance("1.1.1.1", "3333", true)
	ins.Dump(block, uint64(1))
	db := ins.GetDB()

	res, err := db.Get([]byte(BlockHeightKey))
	if err == nil {
		toInt, err := strconv.Atoi(string(res))
		assert.Equal(t, toInt, 1, "error")
		assert.Nil(t, err, "error")
	} else {
		t.Error("Error")
	}

	data, err := db.Get([]byte(GetBlockKey(1)))
	assert.Nil(t, err, "should be nil")
	blockData, err := rlp.EncodeToBytes(block)
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, bytes.Compare(data, blockData), 0, "should be equal")
}

func TestUpdateAddressStorage(t *testing.T) {
	tx1 := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	tx2 := types.NewTransaction(2, common.BytesToAddress([]byte{0x22}), 0, big.NewInt(222), 2222, big.NewInt(22222), []byte{0x22, 0x22, 0x22})
	tx3 := types.NewTransaction(3, common.BytesToAddress([]byte{0x33}), 0, big.NewInt(333), 3333, big.NewInt(33333), []byte{0x33, 0x33, 0x33})
	txs := []*types.Transaction{tx1, tx2, tx3}

	block := types.NewBlock(&types.Header{Number: big.NewInt(314)}, txs, nil)
	ins := GetStorageInstance("1.1.1.1", "3333", true)
	ins.Dump(block, uint64(1))
	db := ins.GetDB()

	res, err := db.Get([]byte(BlockHeightKey))
	if err == nil {
		toInt, err := strconv.Atoi(string(res))
		assert.Equal(t, toInt, 1, "error")
		assert.Nil(t, err, "error")
	} else {
		t.Error("Error")
	}

	data, err := db.Get([]byte(GetBlockKey(1)))
	assert.Nil(t, err, "should be nil")
	blockData, err := rlp.EncodeToBytes(block)
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, bytes.Compare(data, blockData), 0, "should be equal")
}
