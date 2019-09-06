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
	"encoding/binary"
	"fmt"

	"github.com/harmony-one/bls/ffi/go/bls"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"

	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
)

// BlockValidator is responsible for validating block headers, uncles and
// processed state.
//
// BlockValidator implements Validator.
type BlockValidator struct {
	config *params.ChainConfig     // Chain configuration options
	bc     *BlockChain             // Canonical block chain
	engine consensus_engine.Engine // Consensus engine used for validating
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(config *params.ChainConfig, blockchain *BlockChain, engine consensus_engine.Engine) *BlockValidator {
	validator := &BlockValidator{
		config: config,
		engine: engine,
		bc:     blockchain,
	}
	return validator
}

// ValidateBody validates the given block's uncles and verifies the block
// header's transaction and uncle roots. The headers are assumed to be already
// validated at this point.
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
	if hash := types.DeriveSha(block.Transactions()); hash != header.TxHash() {
		return fmt.Errorf("transaction root hash mismatch: have %x, want %x", hash, header.TxHash())
	}
	return nil
}

// ValidateState validates the various changes that happen after a state
// transition, such as amount of used gas, the receipt roots and the state root
// itself. ValidateState returns a database batch if the validation was a success
// otherwise nil and an error is returned.
func (v *BlockValidator) ValidateState(block, parent *types.Block, statedb *state.DB, receipts types.Receipts, cxReceipts types.CXReceipts, usedGas uint64) error {
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

	cxsSha := types.DeriveMultipleShardsSha(cxReceipts)
	if cxsSha != header.OutgoingReceiptHash() {
		return fmt.Errorf("invalid cross shard receipt root hash (remote: %x local: %x)", header.OutgoingReceiptHash(), cxsSha)
	}

	// Validate the state root against the received state root and throw
	// an error if they don't match.
	if root := statedb.IntermediateRoot(v.config.IsS3(header.Epoch())); header.Root() != root {
		return fmt.Errorf("invalid merkle root (remote: %x local: %x)", header.Root(), root)
	}
	return nil
}

// VerifyHeaderWithSignature verifies the header with corresponding commit sigs
func VerifyHeaderWithSignature(bc *BlockChain, header *block.Header, commitSig [96]byte, commitBitmap []byte) error {
	shardState, err := bc.ReadShardState(header.Epoch())
	committee := shardState.FindCommitteeByID(header.ShardID())

	if err != nil || committee == nil {
		return ctxerror.New("[VerifyHeaderWithSignature] Failed to read shard state", "shardID", header.ShardID(), "blockNum", header.Number()).WithCause(err)
	}
	var committerKeys []*bls.PublicKey

	parseKeysSuccess := true
	for _, member := range committee.NodeList {
		committerKey := new(bls.PublicKey)
		err = member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			parseKeysSuccess = false
			break
		}
		committerKeys = append(committerKeys, committerKey)
	}
	if !parseKeysSuccess {
		return ctxerror.New("[VerifyBlockWithSignature] cannot convert BLS public key", "shardID", header.ShardID(), "blockNum", header.Number()).WithCause(err)
	}

	mask, err := bls2.NewMask(committerKeys, nil)
	if err != nil {
		return ctxerror.New("[VerifyHeaderWithSignature] cannot create group sig mask", "shardID", header.ShardID(), "blockNum", header.Number()).WithCause(err)
	}
	if err := mask.SetMask(commitBitmap); err != nil {
		return ctxerror.New("[VerifyHeaderWithSignature] cannot set group sig mask bits", "shardID", header.ShardID(), "blockNum", header.Number()).WithCause(err)
	}

	aggSig := bls.Sign{}
	err = aggSig.Deserialize(commitSig[:])
	if err != nil {
		return ctxerror.New("[VerifyNewBlock] unable to deserialize multi-signature from payload").WithCause(err)
	}

	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, header.Number().Uint64())
	hash := header.Hash()
	commitPayload := append(blockNumBytes, hash[:]...)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		return ctxerror.New("[VerifyHeaderWithSignature] Failed to verify the signature for last commit sig", "shardID", header.ShardID(), "blockNum", header.Number())
	}
	return nil
}

// VerifyBlockLastCommitSigs verifies the last commit sigs of the block
func VerifyBlockLastCommitSigs(bc *BlockChain, header *block.Header) error {
	parentBlock := bc.GetBlockByNumber(header.Number().Uint64() - 1)
	if parentBlock == nil {
		return ctxerror.New("[VerifyBlockLastCommitSigs] Failed to get parent block", "shardID", header.ShardID(), "blockNum", header.Number())
	}
	parentHeader := parentBlock.Header()
	lastCommitSig := header.LastCommitSignature()
	lastCommitBitmap := header.LastCommitBitmap()

	return VerifyHeaderWithSignature(bc, parentHeader, lastCommitSig, lastCommitBitmap)
}

// CalcGasLimit computes the gas limit of the next block after parent. It aims
// to keep the baseline gas above the provided floor, and increase it towards the
// ceil if the blocks are full. If the ceil is exceeded, it will always decrease
// the gas allowance.
func CalcGasLimit(parent *types.Block, gasFloor, gasCeil uint64) uint64 {
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
