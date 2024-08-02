package message

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func FuzzCrosslink(t *testing.F) {
	t.Fuzz(func(t *testing.T, shardID uint32, viewID int64, epoch int64, blockNumber int64) {
		hash := []byte{0x01, 0x02, 0x03, 0x04}
		bitmap := []byte{0x05, 0x06, 0x07, 0x08}
		signature := []byte{0x09, 0x0a, 0x0b, 0x0c}
		proposer := []byte{0x0d, 0x0e, 0x0f, 0x10}

		cl := &CrosslinkMessage{
			ShardId:     shardID,
			ViewId:      new(big.Int).SetInt64(viewID).Bytes(),
			Epoch:       new(big.Int).SetInt64(epoch).Bytes(),
			BlockNumber: new(big.Int).SetInt64(blockNumber).Bytes(),
			Hash:        hash,
			Bitmap:      bitmap,
			Signature:   signature,
			Proposer:    proposer,
		}
		v2 := CreateCrossLinkV2(cl)

		require.Equal(t, uint32(1), v2.ShardID())
		require.Equal(t, viewID, v2.ViewID().Int64())
		require.Equal(t, epoch, v2.Epoch().Int64())
		require.Equal(t, blockNumber, v2.Number().Int64())
		require.Equal(t, common.BytesToHash(hash), v2.Hash())
		require.Equal(t, bitmap, v2.Bitmap())
		require.Equal(t, signature, v2.Signature())
		require.Equal(t, proposer, v2.BlockProposer())
	})
}
