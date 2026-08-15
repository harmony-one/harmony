package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	blockv0 "github.com/harmony-one/harmony/block/v0"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/types"
)

func validateBlockHashes(block *types.Block) error {
	return validateBlockHashesWith(block, consensus_engine.ValidateBlockHash)
}

func validateBlockHashesWith(block *types.Block, validateHash func(common.Hash) error) error {
	if err := validateHash(block.Hash()); err != nil {
		return err
	}

	_, isV0 := block.Header().Header.(*blockv0.Header)
	if !isV0 {
		encoded := block.Header().CrossLinks()
		var crossLinks types.CrossLinks
		if len(encoded) > 0 && rlp.DecodeBytes(encoded, &crossLinks) == nil {
			for i := range crossLinks {
				if err := validateHash(crossLinks[i].Hash()); err != nil {
					return err
				}
			}
		}
	}

	for _, proof := range block.IncomingReceipts() {
		if proof != nil && proof.Header != nil {
			if err := validateHash(proof.Header.Hash()); err != nil {
				return err
			}
		}
	}
	return nil
}
