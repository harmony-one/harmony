package consensus

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	consensusengine "github.com/harmony-one/harmony/consensus/engine"
)

func TestValidateNewBlockRejectsAbandonedChainAnchorBeforeVerifiedCache(t *testing.T) {
	hash := common.HexToHash("0x890473cdb9aa8dc5c0bbd54cf20b6d8d84bda60d3dcb2273443d34432d8539e8")
	log := NewFBFTLog()
	log.verifiedBlocks[hash] = struct{}{}
	consensus := &Consensus{fBFTLog: log}

	_, err := consensus.validateNewBlock(&FBFTMessage{BlockHash: hash})
	if !errors.Is(err, consensusengine.ErrRejectedBlock) {
		t.Fatalf("validateNewBlock() error = %v, want %v", err, consensusengine.ErrRejectedBlock)
	}
}
