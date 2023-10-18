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
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// Validator is an interface which defines the standard for block validation. It
// is only responsible for validating block contents, as the header validation is
// done by the specific consensus engines.
type Validator interface {
	// ValidateBody validates the given block's content.
	ValidateBody(block *types.Block) error

	// ValidateState validates the given statedb and optionally the receipts and
	// gas used.
	ValidateState(block *types.Block, state *state.DB, receipts types.Receipts, cxs types.CXReceipts, usedGas uint64) error

	// ValidateHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	ValidateHeader(block *types.Block, seal bool) error

	// ValidateCXReceiptsProof checks whether the given CXReceiptsProof is consistency with itself
	ValidateCXReceiptsProof(cxp *types.CXReceiptsProof) error
}

// Processor is an interface for processing blocks using a given initial state.
//
// Process takes the block to be processed and the statedb upon which the
// initial state is based. It should return the receipts generated, amount
// of gas used in the process and return an error if any of the internal rules
// failed.
// Process will cache the result of successfully processed blocks.
// readCache decides whether the method will try reading from result cache.
type Processor interface {
	Process(block *types.Block, statedb *state.DB, cfg vm.Config, readCache bool) (
		types.Receipts, types.CXReceipts, []stakingTypes.StakeMsg,
		[]*types.Log, uint64, reward.Reader, *state.DB, error,
	)
	CacheProcessorResult(cacheKey interface{}, result *ProcessorResult)
}
