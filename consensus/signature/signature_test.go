package signature

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/params"
)

func TestConstructCommitPayload(t *testing.T) {
	epoch := big.NewInt(0)
	blockHash := common.Hash{}
	blockNum := uint64(0)
	viewID := uint64(0)
	config := &params.ChainConfig{
		StakingEpoch: new(big.Int).Set(epoch),
	}
	if rs := ConstructCommitPayload(config, epoch, blockHash, blockNum, viewID); len(rs) == 0 {
		t.Error("ConstructCommitPayload failed")
	}

	config.StakingEpoch = new(big.Int).Add(epoch, big.NewInt(1))
	if rs := ConstructCommitPayload(config, epoch, blockHash, blockNum, viewID); len(rs) == 0 {
		t.Error("ConstructCommitPayload failed")
	}
}
