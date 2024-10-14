package stagedstreamsync

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

// marshalData serializes a uint64 block number into a big-endian byte slice.
func marshalData(blockNumber uint64) []byte {
	return encodeBigEndian(blockNumber)
}

// unmarshalData deserializes a byte slice into a uint64 block number.
func unmarshalData(data []byte) (uint64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if len(data) < 8 {
		return 0, fmt.Errorf("invalid data length: expected at least 8 bytes, got %d", len(data))
	}
	return binary.BigEndian.Uint64(data[:8]), nil
}

// encodeBigEndian encodes a uint64 value into an 8-byte big-endian slice.
func encodeBigEndian(n uint64) []byte {
	var v [8]byte
	binary.BigEndian.PutUint64(v[:], n)
	return v[:]
}

// divideCeil performs ceiling division of two integers.
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

// checkGetBlockByHashesResult checks the integrity of blocks received by their hashes.
func checkGetBlockByHashesResult(blocks []*types.Block, hashes []common.Hash) error {
	if len(blocks) != len(hashes) {
		return ErrUnexpectedNumberOfBlockHashes
	}
	for i, block := range blocks {
		if block == nil {
			return ErrNilBlock
		}
		if block.Hash() != hashes[i] {
			return fmt.Errorf("unexpected block hash: got %x, expected %x", block.Hash(), hashes[i])
		}
	}
	return nil
}

// getBlockByMaxVote returns the block that has the most votes based on their hashes.
func getBlockByMaxVote(blocks []*types.Block) (*types.Block, error) {
	hashesVote := make(map[common.Hash]int)
	var maxVotedBlock *types.Block
	maxVote := -1

	for _, block := range blocks {
		if block == nil {
			continue
		}
		hash := block.Header().Hash()
		if _, exist := hashesVote[hash]; !exist {
			hashesVote[hash] = 0
		}
		hashesVote[hash]++
		if hashesVote[hash] > maxVote {
			maxVote = hashesVote[hash]
			maxVotedBlock = block
		}
	}
	if maxVote < 0 {
		return nil, ErrInvalidBlockBytes
	}
	return maxVotedBlock, nil
}

// countHashMaxVote counts the votes for each hash in the map, respecting the whitelist.
// It returns the hash with the most votes and the next whitelist of StreamIDs.
func countHashMaxVote(m map[sttypes.StreamID]common.Hash, whitelist map[sttypes.StreamID]struct{}) (common.Hash, map[sttypes.StreamID]struct{}) {
	var (
		voteM  = make(map[common.Hash]int)
		res    common.Hash
		maxCnt = 0
	)

	for st, h := range m {
		if h == emptyHash {
			continue
		}
		// If a whitelist is provided, skip StreamIDs not in the whitelist
		if len(whitelist) != 0 {
			if _, ok := whitelist[st]; !ok {
				continue
			}
		}
		voteM[h]++
		if voteM[h] > maxCnt {
			maxCnt = voteM[h]
			res = h
		}
	}

	// Build the next whitelist based on the winning hash
	nextWl := make(map[sttypes.StreamID]struct{})
	if res != emptyHash {
		for st, h := range m {
			if h == res {
				if len(whitelist) == 0 || (len(whitelist) != 0 && whitelist[st] != struct{}{}) {
					nextWl[st] = struct{}{}
				}
			}
		}
	}

	return res, nextWl
}

// ByteCount returns a human-readable string representation of a byte count.
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
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}
