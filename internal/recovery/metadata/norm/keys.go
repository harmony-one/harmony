package norm

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Raw key builders mirroring core/rawdb/schema.go (whose builders are
// package-private). Byte-for-byte identical shapes; pinned by unit vectors
// against rawdb-written fixtures.

var (
	prefixValidatorList = []byte("validator-list")
	prefixDVL           = []byte("dvl")
	prefixSnapshot      = []byte("validator-snapshot")
	prefixStats         = []byte("validator-stats")
	prefixShardState    = []byte("ss")
	prefixEpochNumber   = []byte("harmony-epoch-block-number")
	prefixEpochVRF      = []byte("epoch-vrf-block-numbers")
	prefixEpochVDF      = []byte("epoch-vdf-block-number")
	prefixBlkRwd        = []byte("blk-rwd-")
	prefixBlockSig      = []byte("block-sig-")
	keyLastCommits      = []byte("LastCommits")
	prefixPendingCL     = []byte("pendingCL")
	prefixPendingSC     = []byte("pendingSC")
	prefixCrossLink     = []byte("cl")
	prefixCXSpent       = []byte("cxReceiptSpent")
)

func u64be(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

func u32be(n uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return b
}

func dvlKey(delegator common.Address) []byte {
	return append(append([]byte(nil), prefixDVL...), delegator.Bytes()...)
}

func snapshotKey(addr common.Address, epoch *big.Int) []byte {
	k := append(append([]byte(nil), prefixSnapshot...), addr.Bytes()...)
	return append(k, epoch.Bytes()...)
}

func shardStateKey(epoch *big.Int) []byte {
	return append(append([]byte(nil), prefixShardState...), epoch.Bytes()...)
}

func blkRwdKey(number uint64) []byte {
	return append(append([]byte(nil), prefixBlkRwd...), u64be(number)...)
}

func hexKey(k []byte) string { return fmt.Sprintf("%x", k) }

func sectionSnapshots(epoch uint64) string {
	return fmt.Sprintf("validator-snapshot-%d", epoch)
}

func sectionShardState(epoch uint64) string {
	return fmt.Sprintf("shard-state-%d", epoch)
}
