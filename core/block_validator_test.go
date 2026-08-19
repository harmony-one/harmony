package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
)

// TestValidateCXReceiptsProofRejectsMerkleProofLengthMismatch checks that a
// merkle proof whose ShardIDs and CXShardHashes lists differ in length is
// reported as invalid. The two lists pair up positionally, so an unequal pairing
// is not a proof this function can evaluate.
func TestValidateCXReceiptsProofRejectsMerkleProofLengthMismatch(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, header, _ := getTestEnvironment(*key)
	// AcceptsCrossTx requires an epoch past CrossTxEpoch, otherwise validation
	// short-circuits before reaching the merkle proof.
	header = header.With().Epoch(big.NewInt(1)).Header()
	validator := NewBlockValidator(chain)

	to := common.BytesToAddress([]byte{0x42})
	cxp := &types.CXReceiptsProof{
		Header: header,
		Receipts: types.CXReceipts{{
			TxHash:    common.Hash{0x01},
			From:      common.BytesToAddress([]byte{0x11}),
			To:        &to,
			ShardID:   0,
			ToShardID: 1,
			Amount:    big.NewInt(1),
		}},
		MerkleProof: &types.CXMerkleProof{
			BlockNum:      big.NewInt(1),
			ShardID:       0,
			ShardIDs:      []uint32{0, 1, 2},
			CXShardHashes: []common.Hash{}, // shorter than ShardIDs
		},
		CommitSig:    []byte{0x01},
		CommitBitmap: []byte{0x01},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ValidateCXReceiptsProof did not handle the length mismatch: %v", r)
		}
	}()

	if err := validator.ValidateCXReceiptsProof(cxp); err == nil {
		t.Fatal("expected error for merkle proof with mismatched ShardIDs/CXShardHashes lengths")
	}
}

// TestValidateCXReceiptsProofRejectsNilMerkleProof checks that a receipts proof
// carrying no merkle proof at all is reported as invalid.
func TestValidateCXReceiptsProofRejectsNilMerkleProof(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, header, _ := getTestEnvironment(*key)
	// AcceptsCrossTx requires an epoch past CrossTxEpoch, otherwise validation
	// short-circuits before reaching the merkle proof.
	header = header.With().Epoch(big.NewInt(1)).Header()
	validator := NewBlockValidator(chain)

	to := common.BytesToAddress([]byte{0x42})
	cxp := &types.CXReceiptsProof{
		Header: header,
		Receipts: types.CXReceipts{{
			TxHash:    common.Hash{0x01},
			From:      common.BytesToAddress([]byte{0x11}),
			To:        &to,
			ShardID:   0,
			ToShardID: 1,
			Amount:    big.NewInt(1),
		}},
		CommitSig:    []byte{0x01},
		CommitBitmap: []byte{0x01},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ValidateCXReceiptsProof did not handle the missing merkle proof: %v", r)
		}
	}()

	if err := validator.ValidateCXReceiptsProof(cxp); err == nil {
		t.Fatal("expected error for nil merkle proof")
	}
}
