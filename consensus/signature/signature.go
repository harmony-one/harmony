package signature

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/params"
)

type configChainReader interface {
	Config() *params.ChainConfig
}

// ConstructCommitPayload returns the commit payload for consensus signatures.
func ConstructCommitPayload(
	chain configChainReader, blockHash common.Hash, epoch, blockNum, viewID uint64,
) []byte {
	blockNumBytes := make([]byte, 8)
	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, blockNum)
	binary.LittleEndian.PutUint64(viewIDBytes, viewID)
	if chain.Config().IsStaking(new(big.Int).SetUint64(epoch)) {
		return append(blockNumBytes, append(viewIDBytes, blockHash.Bytes()...)...)
	}
	return append(blockNumBytes, blockHash.Bytes()...)
}
