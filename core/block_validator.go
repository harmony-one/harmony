// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	"github.com/pkg/errors"

	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/params"
)

// BlockValidator is responsible for validating block headers, uncles and
// processed state.
//
// BlockValidator implements validator.
type BlockValidator struct {
	bc BlockChain // Canonical blockchain
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(blockchain BlockChain) *BlockValidator {
	validator := &BlockValidator{
		bc: blockchain,
	}
	return validator
}

// ValidateBody verifies the block header's transaction root.
// The headers are assumed to be already validated at this point.
func (v *BlockValidator) ValidateBody(block *types.Block) error {
	// Check whether the block's known, and if not, that it's linkable
	if v.bc.HasBlockAndState(block.Hash(), block.NumberU64()) {
		return ErrKnownBlock
	}
	if !v.bc.HasBlockAndState(block.ParentHash(), block.NumberU64()-1) {
		if !v.bc.HasBlock(block.ParentHash(), block.NumberU64()-1) {
			return consensus_engine.ErrUnknownAncestor
		}
		return consensus_engine.ErrPrunedAncestor
	}
	// Header validity is known at this point, check the uncles and transactions
	header := block.Header()
	//if err := v.engine.VerifyUncles(v.bc, block); err != nil {
	//	return err
	//}
	if hash := types.DeriveSha(
		block.Transactions(),
		block.StakingTransactions(),
	); hash != header.TxHash() {
		return fmt.Errorf("transaction root hash mismatch: have %x, want %x", hash, header.TxHash())
	}
	return nil
}

// ValidateState validates the various changes that happen after a state
// transition, such as amount of used gas, the receipt roots and the state root
// itself. ValidateState returns a database batch if the validation was a success
// otherwise nil and an error is returned.
func (v *BlockValidator) ValidateState(block *types.Block, statedb *state.DB, receipts types.Receipts, cxReceipts types.CXReceipts, usedGas uint64) error {
	header := block.Header()
	if block.GasUsed() != usedGas {
		return fmt.Errorf("invalid gas used (remote: %d local: %d)", block.GasUsed(), usedGas)
	}
	// Validate the received block's bloom with the one derived from the generated receipts.
	// For valid blocks this should always validate to true.
	rbloom := types.CreateBloom(receipts)
	if rbloom != header.Bloom() {
		return fmt.Errorf("invalid bloom (remote: %x  local: %x)", header.Bloom(), rbloom)
	}
	// Tre receipt Trie's root (R = (Tr [[H1, R1], ... [Hn, R1]]))
	receiptSha := types.DeriveSha(receipts)
	if receiptSha != header.ReceiptHash() {
		return fmt.Errorf("invalid receipt root hash (remote: %x local: %x)", header.ReceiptHash(), receiptSha)
	}

	if v.bc.Config().AcceptsCrossTx(block.Epoch()) {
		cxsSha := cxReceipts.ComputeMerkleRoot()
		if cxsSha != header.OutgoingReceiptHash() {
			legacySha := types.DeriveMultipleShardsSha(cxReceipts)
			if legacySha != header.OutgoingReceiptHash() {
				return fmt.Errorf("invalid cross shard receipt root hash (remote: %x local: %x, legacy: %x)", header.OutgoingReceiptHash(), cxsSha, legacySha)
			}
		}
	}

	// Validate the state root against the received state root and throw
	// an error if they don't match.
	if root := statedb.IntermediateRoot(v.bc.Config().IsS3(header.Epoch())); header.Root() != root {
		dump, _ := rlp.EncodeToBytes(header)
		const msg = "invalid merkle root (remote: %x local: %x, rlp dump %s)"
		return fmt.Errorf(msg, header.Root(), root, hex.EncodeToString(dump))
	}
	return nil
}

// ValidateHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (v *BlockValidator) ValidateHeader(block *types.Block, seal bool) error {
	if block == nil {
		return errors.New("block is nil")
	}
	if h := block.Header(); h != nil {
		return v.bc.Engine().VerifyHeader(v.bc, h, true)
	}
	return errors.New("header field was nil")
}

// CalcGasLimit computes the gas limit of the next block after parent. It aims
// to keep the baseline gas above the provided floor, and increase it towards the
// ceil if the blocks are full. If the ceil is exceeded, it will always decrease
// the gas allowance.
func CalcGasLimit(parent *block.Header, gasFloor, gasCeil uint64) uint64 {
	// contrib = (parentGasUsed * 3 / 2) / 1024
	contrib := (parent.GasUsed() + parent.GasUsed()/2) / params.GasLimitBoundDivisor

	// decay = parentGasLimit / 1024 -1
	decay := parent.GasLimit()/params.GasLimitBoundDivisor - 1

	/*
		strategy: gasLimit of block-to-mine is set based on parent's
		gasUsed value.  if parentGasUsed > parentGasLimit * (2/3) then we
		increase it, otherwise lower it (or leave it unchanged if it's right
		at that usage) the amount increased/decreased depends on how far away
		from parentGasLimit * (2/3) parentGasUsed is.
	*/
	limit := parent.GasLimit() - decay + contrib
	if limit < params.MinGasLimit {
		limit = params.MinGasLimit
	}
	// If we're outside our allowed gas range, we try to hone towards them
	if limit < gasFloor {
		limit = parent.GasLimit() + decay
		if limit > gasFloor {
			limit = gasFloor
		}
	} else if limit > gasCeil {
		limit = parent.GasLimit() - decay
		if limit < gasCeil {
			limit = gasCeil
		}
	}
	return limit
}

// ValidateCXReceiptsProof checks whether the given CXReceiptsProof is consistency with itself
func (v *BlockValidator) ValidateCXReceiptsProof(cxp *types.CXReceiptsProof) error {
	if !v.bc.Config().AcceptsCrossTx(cxp.Header.Epoch()) {
		return errors.New("[ValidateCXReceiptsProof] cross shard receipt received before cx fork")
	}

	toShardID, err := cxp.GetToShardID()
	if err != nil {
		return errors.Wrapf(err, "[ValidateCXReceiptsProof] invalid shardID")
	}

	merkleProof := cxp.MerkleProof
	shardRoot := common.Hash{}
	foundMatchingShardID := false
	byteBuffer := bytes.Buffer{}

	// prepare to calculate source shard outgoing cxreceipts root hash
	for j := 0; j < len(merkleProof.ShardIDs); j++ {
		sKey := make([]byte, 4)
		binary.BigEndian.PutUint32(sKey, merkleProof.ShardIDs[j])
		byteBuffer.Write(sKey)
		byteBuffer.Write(merkleProof.CXShardHashes[j][:])
		if merkleProof.ShardIDs[j] == toShardID {
			shardRoot = merkleProof.CXShardHashes[j]
			foundMatchingShardID = true
		}
	}

	if !foundMatchingShardID {
		return errors.New("[ValidateCXReceiptsProof] Didn't find matching toShardID (no receipts for my shard)")
	}

	sha := types.DeriveSha(cxp.Receipts)
	// (1) verify the CXReceipts trie root match
	if sha != shardRoot {
		return errors.New(
			"[ValidateCXReceiptsProof] Trie Root of ReadCXReceipts Not Match",
		)
	}

	// (2) verify the outgoingCXReceiptsHash match
	outgoingHashFromSourceShard := crypto.Keccak256Hash(byteBuffer.Bytes())
	if byteBuffer.Len() == 0 {
		outgoingHashFromSourceShard = types.EmptyRootHash
	}
	if outgoingHashFromSourceShard != merkleProof.CXReceiptHash {
		return errors.New(
			"[ValidateCXReceiptsProof] IncomingReceiptRootHash from source shard not match",
		)
	}

	// (3) verify the block hash matches
	if cxp.Header.Hash() != merkleProof.BlockHash ||
		cxp.Header.OutgoingReceiptHash() != merkleProof.CXReceiptHash {
		return errors.New(
			"[ValidateCXReceiptsProof] BlockHash or OutgoingReceiptHash not match in block Header",
		)
	}

	// (4) verify blockHeader with seal
	var commitSig bls.SerializedSignature
	copy(commitSig[:], cxp.CommitSig)
	return v.bc.Engine().VerifyHeaderSignature(v.bc, cxp.Header, commitSig, cxp.CommitBitmap)
}
