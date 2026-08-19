package core

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
)

// TestValidateCXReceiptsProofRejectsShortCXShardHashes checks that a merkle
// proof carrying fewer hashes than shard ids is reported as invalid, since there
// is no hash to read for the shard ids past the end of that list.
func TestValidateCXReceiptsProofRejectsShortCXShardHashes(t *testing.T) {
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
			t.Fatalf("ValidateCXReceiptsProof did not handle the short hash list: %v", r)
		}
	}()

	if err := validator.ValidateCXReceiptsProof(cxp); err == nil {
		t.Fatal("expected error for a merkle proof with fewer hashes than shard ids")
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

// TestValidateCXReceiptsProofAcceptsTrailingCXShardHashes checks that hashes
// beyond the last shard id are still tolerated. The loop never reads them, and
// the previous release accepted such a proof, so rejecting one here would mean
// disagreeing with nodes that have not upgraded about which proofs are valid.
func TestValidateCXReceiptsProofAcceptsTrailingCXShardHashes(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, header, _ := getTestEnvironment(*key)
	header = header.With().Epoch(big.NewInt(1)).Header()
	validator := NewBlockValidator(chain)

	to := common.BytesToAddress([]byte{0x42})
	receipts := types.CXReceipts{{
		TxHash:    common.Hash{0x01},
		From:      common.BytesToAddress([]byte{0x11}),
		To:        &to,
		ShardID:   0,
		ToShardID: 1,
		Amount:    big.NewInt(1),
	}}
	cxp := &types.CXReceiptsProof{
		Header:   header,
		Receipts: receipts,
		MerkleProof: &types.CXMerkleProof{
			BlockNum: big.NewInt(1),
			ShardID:  0,
			ShardIDs: []uint32{1},
			// One hash for the single shard id, plus a trailing one.
			CXShardHashes: []common.Hash{types.DeriveSha(receipts), {0x99}},
		},
		CommitSig:    []byte{0x01},
		CommitBitmap: []byte{0x01},
	}

	// The proof fails later on for other reasons; what matters is that it is not
	// turned away for the length of its hash list.
	err := validator.ValidateCXReceiptsProof(cxp)
	if err != nil && strings.Contains(err.Error(), "CXShardHashes") {
		t.Fatalf("trailing hashes should not be rejected: %v", err)
	}
}
