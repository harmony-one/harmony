package core

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
	"github.com/stretchr/testify/require"
)

// TestValidateCXReceiptsProofRejectsShortMerkleHashList checks that a merkle
// proof with fewer CXShardHashes than ShardIDs is reported as invalid. The loop
// indexes hashes by shard ID position, so a shorter hash list cannot be evaluated.
func TestValidateCXReceiptsProofRejectsShortMerkleHashList(t *testing.T) {
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

// TestValidateCXReceiptsProofAcceptsTrailingMerkleHashes preserves the legacy
// behavior of walking ShardIDs and ignoring any trailing CXShardHashes. Tightening
// this to exact length equality requires coordinated activation because older
// validators accept the same proof.
func TestValidateCXReceiptsProofAcceptsTrailingMerkleHashes(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, header, _ := getTestEnvironment(*key)
	header.SetEpoch(big.NewInt(1))
	header.SetNumber(big.NewInt(1))
	header.SetShardID(0)
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
	shardHash := types.DeriveSha(receipts)
	var encoded bytes.Buffer
	require.NoError(t, binary.Write(&encoded, binary.BigEndian, uint32(1)))
	_, err := encoded.Write(shardHash[:])
	require.NoError(t, err)
	outgoingHash := crypto.Keccak256Hash(encoded.Bytes())
	header.SetOutgoingReceiptHash(outgoingHash)

	cxp := &types.CXReceiptsProof{
		Header:   header,
		Receipts: receipts,
		MerkleProof: &types.CXMerkleProof{
			BlockNum:      big.NewInt(1),
			BlockHash:     header.Hash(),
			ShardID:       0,
			CXReceiptHash: outgoingHash,
			ShardIDs:      []uint32{1},
			CXShardHashes: []common.Hash{shardHash, {0xFF}},
		},
		CommitSig: make([]byte, 96),
	}

	require.NoError(t, validator.ValidateCXReceiptsProof(cxp))
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
