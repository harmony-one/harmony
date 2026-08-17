package types

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	blockfactory "github.com/harmony-one/harmony/block/factory"
)

// TestCXReceiptsProofCopyPreservesCommitFields checks that Copy reproduces the
// commit signature and the commit bitmap as distinct values. Block bodies are
// stored and reloaded through Copy, so a copy has to round-trip both fields
// exactly for the reloaded body to still match the hash in its header.
func TestCXReceiptsProofCopyPreservesCommitFields(t *testing.T) {
	to := common.BytesToAddress([]byte{0x42})
	sig := bytes.Repeat([]byte{0xAA}, 96)
	bitmap := []byte{0x01, 0x02, 0x03}

	original := &CXReceiptsProof{
		Receipts: CXReceipts{{
			TxHash:    common.Hash{0x01},
			From:      common.BytesToAddress([]byte{0x11}),
			To:        &to,
			ShardID:   0,
			ToShardID: 1,
			Amount:    big.NewInt(7),
		}},
		MerkleProof: &CXMerkleProof{
			BlockNum:      big.NewInt(9),
			BlockHash:     common.Hash{0x02},
			ShardID:       0,
			CXReceiptHash: common.Hash{0x03},
			ShardIDs:      []uint32{1},
			CXShardHashes: []common.Hash{{0x04}},
		},
		Header:       blockfactory.ForTest.NewHeader(big.NewInt(1)),
		CommitSig:    sig,
		CommitBitmap: bitmap,
	}

	cpy := original.Copy()

	if !bytes.Equal(cpy.CommitSig, sig) {
		t.Errorf("CommitSig not preserved: got %x want %x", cpy.CommitSig, sig)
	}
	if !bytes.Equal(cpy.CommitBitmap, bitmap) {
		t.Errorf("CommitBitmap not preserved: got %x want %x", cpy.CommitBitmap, bitmap)
	}

	// The copy must be independent of the original.
	cpy.CommitBitmap[0] = 0xFF
	cpy.CommitSig[0] = 0xFF
	if original.CommitBitmap[0] != 0x01 || original.CommitSig[0] != 0xAA {
		t.Error("Copy shares backing arrays with the original")
	}
}

// TestContainsEmptyFieldNilReceiver checks that a nil receipts proof reports
// itself as empty instead of reading through the nil receiver.
func TestContainsEmptyFieldNilReceiver(t *testing.T) {
	var cxp *CXReceiptsProof
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ContainsEmptyField did not handle a nil receiver: %v", r)
		}
	}()
	if !cxp.ContainsEmptyField() {
		t.Fatal("expected a nil proof to be reported as empty")
	}
}
