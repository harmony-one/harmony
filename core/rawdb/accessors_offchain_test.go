package rawdb

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/rawdb"
)

// Tests block header storage and retrieval operations.
func TestBlockPruningStateStorage(t *testing.T) {
	db := rawdb.NewMemoryDatabase()

	var testBlockNumber uint64
	testBlockNumber = 12345
	WriteBlockPruningState(db, 0, testBlockNumber)

	testBlockNumberRetrieved := ReadBlockPruningState(db, 0)

	if testBlockNumber != *testBlockNumberRetrieved {
		t.Fatalf("Block pruning state doesn't match what was stored: read %d, stored %d", *testBlockNumberRetrieved, testBlockNumber)
	}
}
