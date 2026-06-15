package rawdb

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/core/types"
)

func TestWriteCXReceiptsProofSpentUsesMerkleProofIdentity(t *testing.T) {
	db := NewMemoryDatabase()

	cxp := &types.CXReceiptsProof{
		MerkleProof: &types.CXMerkleProof{
			ShardID:  1,
			BlockNum: big.NewInt(999),
		},
	}

	if err := WriteCXReceiptsProofSpent(db, cxp); err != nil {
		t.Fatalf("failed to write spent marker: %v", err)
	}

	marker, err := ReadCXReceiptsProofSpent(db, cxp.MerkleProof.ShardID, cxp.MerkleProof.BlockNum.Uint64())
	if err != nil {
		t.Fatalf("failed to read spent marker keyed by merkle proof identity: %v", err)
	}
	if marker != SpentByte {
		t.Fatalf("wrong marker for merkle key: got %v want %v", marker, SpentByte)
	}
}

func TestWriteCXReceiptsProofSpentWithKey(t *testing.T) {
	db := NewMemoryDatabase()
	const shardID uint32 = 2
	const blockNum uint64 = 42

	if err := WriteCXReceiptsProofSpentWithKey(db, shardID, blockNum); err != nil {
		t.Fatalf("failed to write spent marker: %v", err)
	}
	marker, err := ReadCXReceiptsProofSpent(db, shardID, blockNum)
	if err != nil {
		t.Fatalf("failed to read spent marker: %v", err)
	}
	if marker != SpentByte {
		t.Fatalf("wrong marker value: got %v want %v", marker, SpentByte)
	}
}
