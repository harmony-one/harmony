package signature

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/params"
)

type signatureChainReader interface {
	Config() *params.ChainConfig
}

// ConstructCommitPayload returns the commit payload for consensus signatures.
func ConstructCommitPayload(
	chain signatureChainReader, epoch *big.Int, blockHash common.Hash, blockNum, viewID uint64,
) []byte {
	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, blockNum)
	commitPayload := append(blockNumBytes, blockHash.Bytes()...)
	if !chain.Config().IsStaking(epoch) {
		return commitPayload
	}
	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, viewID)
	return append(commitPayload, viewIDBytes...)
}
