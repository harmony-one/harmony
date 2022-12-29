package stagedstreamsync

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

func marshalData(blockNumber uint64) []byte {
	return encodeBigEndian(blockNumber)
}

func unmarshalData(data []byte) (uint64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if len(data) < 8 {
		return 0, fmt.Errorf("value must be at least 8 bytes, got %d", len(data))
	}
	return binary.BigEndian.Uint64(data[:8]), nil
}

func encodeBigEndian(n uint64) []byte {
	var v [8]byte
	binary.BigEndian.PutUint64(v[:], n)
	return v[:]
}

func divideCeil(x, y int) int {
	fVal := float64(x) / float64(y)
	return int(math.Ceil(fVal))
}

// computeBlockNumberByMaxVote computes the target block number by max vote.
func computeBlockNumberByMaxVote(votes map[sttypes.StreamID]uint64) uint64 {
	var (
		nm     = make(map[uint64]int)
		res    uint64
		maxCnt int
	)
	for _, bn := range votes {
		_, ok := nm[bn]
		if !ok {
			nm[bn] = 0
		}
		nm[bn]++
		cnt := nm[bn]

		if cnt > maxCnt || (cnt == maxCnt && bn > res) {
			res = bn
			maxCnt = cnt
		}
	}
	return res
}

func checkGetBlockByHashesResult(blocks []*types.Block, hashes []common.Hash) error {
	if len(blocks) != len(hashes) {
		return ErrUnexpectedNumberOfBlockHashes
	}
	for i, block := range blocks {
		if block == nil {
			return ErrNilBlock
		}
		if block.Hash() != hashes[i] {
			return fmt.Errorf("unexpected block hash: %x / %x", block.Hash(), hashes[i])
		}
	}
	return nil
}

func countHashMaxVote(m map[sttypes.StreamID]common.Hash, whitelist map[sttypes.StreamID]struct{}) (common.Hash, map[sttypes.StreamID]struct{}) {
	var (
		voteM  = make(map[common.Hash]int)
		res    common.Hash
		maxCnt = 0
	)

	for st, h := range m {
		if len(whitelist) != 0 {
			if _, ok := whitelist[st]; !ok {
				continue
			}
		}
		if _, ok := voteM[h]; !ok {
			voteM[h] = 0
		}
		voteM[h]++
		if voteM[h] > maxCnt {
			maxCnt = voteM[h]
			res = h
		}
	}

	nextWl := make(map[sttypes.StreamID]struct{})
	for st, h := range m {
		if h != res {
			continue
		}
		if len(whitelist) != 0 {
			if _, ok := whitelist[st]; ok {
				nextWl[st] = struct{}{}
			}
		} else {
			nextWl[st] = struct{}{}
		}
	}
	return res, nextWl
}

func ByteCount(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
