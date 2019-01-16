package node

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"

	//	"fmt"
	"math/big"
	"reflect"
	"testing"
)

var (
	senderPriKey, _   = crypto.GenerateKey()
	receiverPriKey, _ = crypto.GenerateKey()
	receiverAddress   = crypto.PubkeyToAddress(receiverPriKey.PublicKey)

	amountBigInt = big.NewInt(8000000000000000000)
)

func TestSerializeBlockchainSyncMessage(t *testing.T) {
	h1 := common.HexToHash("123")
	h2 := common.HexToHash("abc")

	msg := BlockchainSyncMessage{
		BlockHeight: 2,
		BlockHashes: []common.Hash{
			h1,
			h2,
		},
	}

	serializedByte := SerializeBlockchainSyncMessage(&msg)

	dMsg, err := DeserializeBlockchainSyncMessage(serializedByte)

	if err != nil || !reflect.DeepEqual(msg, *dMsg) {
		t.Errorf("Failed to serialize/deserialize blockchain sync message\n")
	}
}

func TestConstructTransactionListMessageAccount(t *testing.T) {
	tx, _ := types.SignTx(types.NewTransaction(100, receiverAddress, uint32(0), amountBigInt, params.TxGas, nil, nil), types.HomesteadSigner{}, senderPriKey)
	transactions := types.Transactions{tx}
	buf := ConstructTransactionListMessageAccount(transactions)
	if len(buf) == 0 {
		t.Error("Failed to contruct transaction list message")
	}
}

func TestConstructRequestTransactionsMessage(t *testing.T) {
	txIDs := [][]byte{
		[]byte{1, 2},
		[]byte{3, 4},
	}

	buf := ConstructRequestTransactionsMessage(txIDs)

	if len(buf) == 0 {
		t.Error("Failed to contruct request transaction message")
	}
}

func TestConstructStopMessage(t *testing.T) {
	buf := ConstructStopMessage()

	if len(buf) == 0 {
		t.Error("Failed to contruct STOP message")
	}
}

func TestConstructBlocksSyncMessage(t *testing.T) {

	db := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))

	root := statedb.IntermediateRoot(false)
	head := &types.Header{
		Number:  new(big.Int).SetUint64(uint64(10000)),
		Nonce:   types.EncodeNonce(uint64(10000)),
		ShardID: types.EncodeShardID(uint32(0)),
		Time:    new(big.Int).SetUint64(uint64(100000)),
		Root:    root,
	}
	head.GasLimit = 10000000000
	head.Difficulty = params.GenesisDifficulty

	statedb.Commit(false)
	statedb.Database().TrieDB().Commit(root, true)

	block1 := types.NewBlock(head, nil, nil)

	blocks := []*types.Block{
		block1,
	}

	buf := ConstructBlocksSyncMessage(blocks)

	if len(buf) == 0 {
		t.Error("Failed to contruct block sync message")
	}

}
