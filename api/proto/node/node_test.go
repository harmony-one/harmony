package node

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
)

var (
	senderPriKey, _   = crypto.GenerateKey()
	receiverPriKey, _ = crypto.GenerateKey()
	receiverAddress   = crypto.PubkeyToAddress(receiverPriKey.PublicKey)
	amountBigInt      = big.NewInt(8000000000000000000)
)

func TestConstructTransactionListMessageAccount(t *testing.T) {
	tx, _ := types.SignTx(
		types.NewTransaction(100, receiverAddress, uint32(0),
			amountBigInt, params.TxGas, nil, nil),
		types.HomesteadSigner{}, senderPriKey,
	)
	transactions := types.Transactions{tx}
	buf := ConstructTransactionListMessageAccount(transactions)
	if len(buf) == 0 {
		t.Error("Failed to contruct transaction list message")
	}
}

func TestConstructBlocksSyncMessage(t *testing.T) {
	db := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db))
	root := statedb.IntermediateRoot(false)
	head := blockfactory.NewTestHeader().With().
		Number(new(big.Int).SetUint64(uint64(10000))).
		ShardID(0).
		Time(new(big.Int).SetUint64(uint64(100000))).
		Root(root).
		GasLimit(10000000000).
		Header()

	if _, err := statedb.Commit(false); err != nil {
		t.Fatalf("statedb.Commit() failed: %s", err)
	}
	if err := statedb.Database().TrieDB().Commit(root, true); err != nil {
		t.Fatalf("statedb.Database().TrieDB().Commit() failed: %s", err)
	}

	block1 := types.NewBlock(head, nil, nil, nil, nil, nil)

	blocks := []*types.Block{
		block1,
	}

	buf := ConstructBlocksSyncMessage(blocks)

	if len(buf) == 0 {
		t.Error("Failed to contruct block sync message")
	}

}
