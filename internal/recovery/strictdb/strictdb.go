// Package strictdb provides the metadata pipeline's strict database
// access discipline (plan WS1): error-latching ordered iteration (stock
// off-chain iterators never check Iterator.Error(); here exhaustion
// without a checked error is indistinguishable from truncation and
// therefore always checked), per-namespace raw key-shape validation, and
// the shared namespace classifier used by the scan inventory and the
// audit's write-log reconciliation.
package strictdb

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/ethdb"
)

// ForEach iterates kv over prefix in ascending raw-key order, calling fn
// with copies of key and value, and checks Iterator.Error() on exhaustion
// (fail-closed: an unread error would silently truncate the scan).
func ForEach(kv ethdb.Iteratee, prefix []byte, fn func(key, value []byte) error) error {
	it := kv.NewIterator(prefix, nil)
	defer it.Release()
	for it.Next() {
		k := append([]byte(nil), it.Key()...)
		v := append([]byte(nil), it.Value()...)
		if err := fn(k, v); err != nil {
			return err
		}
	}
	if err := it.Error(); err != nil {
		return fmt.Errorf("strictdb: iterator over prefix %q failed: %w", prefix, err)
	}
	return nil
}

// Namespace identifies a raw-key family.
type Namespace string

const (
	NsValidatorList     Namespace = "validator-list"
	NsDVL               Namespace = "dvl"
	NsValidatorSnapshot Namespace = "validator-snapshot"
	NsValidatorStats    Namespace = "validator-stats"
	NsShardState        Namespace = "shard-state" // "ss" + epoch.Bytes()
	NsEpochBlockNumber  Namespace = "harmony-epoch-block-number"
	NsEpochVRF          Namespace = "epoch-vrf-block-numbers"
	NsEpochVDF          Namespace = "epoch-vdf-block-number"
	NsBlockRewardAccum  Namespace = "blk-rwd"
	NsBlockCommitSig    Namespace = "block-sig"
	NsLastCommits       Namespace = "LastCommits"
	NsPendingCrossLink  Namespace = "pendingCL"
	NsPendingSlashing   Namespace = "pendingSC"
	NsCrossLink         Namespace = "crosslink"         // "cl" + shard(4) + num(8)
	NsCrossLinkPointer  Namespace = "crosslink-pointer" // "cl" + shard(4) — shorter key, same namespace
	NsCXReceipt         Namespace = "cxReceipt"
	NsCXReceiptSpent    Namespace = "cxReceiptSpent"
	NsCXLookup          Namespace = "cx-lookup"
	NsHeader            Namespace = "header"
	NsHeaderTD          Namespace = "header-td"
	NsCanonicalHash     Namespace = "canonical-hash"
	NsHeaderNumber      Namespace = "header-number"
	NsBody              Namespace = "body"
	NsReceipts          Namespace = "receipts"
	NsTxLookup          Namespace = "tx-lookup"
	NsCode              Namespace = "code"           // "c" + hash
	NsValidatorCode     Namespace = "validator-code" // "vc" + hash
	NsStateNode         Namespace = "state-node"     // bare 32-byte key: trie node or legacy code
	NsPreimage          Namespace = "preimage"
	NsHead              Namespace = "head-pointer"
	NsSyncEra           Namespace = "sync-era"
	NsLeaderContinuous  Namespace = "continuous"
	NsConfig            Namespace = "config"
	NsBloom             Namespace = "bloom"
	NsSkeleton          Namespace = "skeleton"
	NsOther             Namespace = "other"
)

// Meta carries the parsed key components a rule needs.
type Meta struct {
	Address []byte   // 20-byte address where applicable
	Epoch   *big.Int // parsed epoch suffix where applicable
	Number  uint64   // block number where applicable
	ShardID uint32   // shard id where applicable
	// CanonicalEpochSuffix reports whether the raw epoch suffix byte-equals
	// epoch.Bytes() (big.Int canonical form, no leading zeros; empty for 0).
	CanonicalEpochSuffix bool
}

var (
	pValidatorList = []byte("validator-list")
	pDVL           = []byte("dvl")
	pSnapshot      = []byte("validator-snapshot")
	pStats         = []byte("validator-stats")
	pSS            = []byte("ss")
	pEpochNum      = []byte("harmony-epoch-block-number")
	pEpochVRF      = []byte("epoch-vrf-block-numbers")
	pEpochVDF      = []byte("epoch-vdf-block-number")
	pBlkRwd        = []byte("blk-rwd-")
	pBlockSig      = []byte("block-sig-")
	pLastCommits   = []byte("LastCommits")
	pPendingCL     = []byte("pendingCL")
	pPendingSC     = []byte("pendingSC")
	pCL            = []byte("cl")
	pCXSpent       = []byte("cxReceiptSpent")
	pCXReceipt     = []byte("cxReceipt")
	pCX            = []byte("cx")
	pSecureKey     = []byte("secure-key-")
	pConfig        = []byte("ethereum-config-")
	pGenesis       = []byte("ethereum-genesis-")
	pBloomIdx      = []byte("iB")

	headKeys = [][]byte{
		[]byte("LastHeader"), []byte("LastBlock"), []byte("LastFast"), []byte("LastFinalized"),
	}
	syncEraKeys = [][]byte{
		[]byte("LastPivot"), []byte("TrieSync"), []byte("SnapshotDisabled"), []byte("SnapshotRoot"),
		[]byte("SnapshotJournal"), []byte("SnapshotGenerator"), []byte("SnapshotRecovery"),
		[]byte("SnapshotSyncStatus"), []byte("SkeletonSyncStatus"), []byte("SnapdbInfo"),
		[]byte("unclean-shutdown"), []byte("InvalidBlock"), []byte("TransactionIndexTail"),
		[]byte("FastTransactionLookupLimit"), []byte("eth2-transition"), []byte("DatabaseVersion"),
		[]byte("preimage-import"), []byte("preimage-gen-start"), []byte("preimage-gen-end"),
	}
)

func be8(b []byte) uint64 {
	var n uint64
	for _, x := range b {
		n = n<<8 | uint64(x)
	}
	return n
}

func be4(b []byte) uint32 {
	var n uint32
	for _, x := range b {
		n = n<<8 | uint32(x)
	}
	return n
}

// epochMeta parses a variable-length big-endian epoch suffix and records
// whether it is canonical (byte-equal to epoch.Bytes(): no leading zeros,
// empty for zero).
func epochMeta(suffix []byte) (m Meta) {
	e := new(big.Int).SetBytes(suffix)
	m.Epoch = e
	m.CanonicalEpochSuffix = bytes.Equal(suffix, e.Bytes())
	return m
}

// Classify identifies the namespace of a raw LevelDB key and parses its
// components. It is a total function: unknown shapes map to NsOther.
func Classify(key []byte) (Namespace, Meta) {
	switch {
	case bytes.Equal(key, pValidatorList):
		return NsValidatorList, Meta{}
	case bytes.HasPrefix(key, pSnapshot): // check before shorter prefixes
		rest := key[len(pSnapshot):]
		if len(rest) < 20 {
			return NsValidatorSnapshot, Meta{}
		}
		m := epochMeta(rest[20:])
		m.Address = rest[:20]
		return NsValidatorSnapshot, m
	case bytes.HasPrefix(key, pStats):
		m := Meta{}
		if rest := key[len(pStats):]; len(rest) == 20 {
			m.Address = rest
		}
		return NsValidatorStats, m
	case bytes.HasPrefix(key, pDVL):
		m := Meta{}
		if rest := key[len(pDVL):]; len(rest) == 20 {
			m.Address = rest
		}
		return NsDVL, m
	case bytes.HasPrefix(key, pEpochNum):
		return NsEpochBlockNumber, epochMeta(key[len(pEpochNum):])
	case bytes.HasPrefix(key, pEpochVRF):
		return NsEpochVRF, epochMeta(key[len(pEpochVRF):])
	case bytes.HasPrefix(key, pEpochVDF):
		return NsEpochVDF, epochMeta(key[len(pEpochVDF):])
	case bytes.HasPrefix(key, pBlkRwd):
		m := Meta{}
		if rest := key[len(pBlkRwd):]; len(rest) == 8 {
			m.Number = be8(rest)
		}
		return NsBlockRewardAccum, m
	case bytes.HasPrefix(key, pBlockSig):
		m := Meta{}
		if rest := key[len(pBlockSig):]; len(rest) == 8 {
			m.Number = be8(rest)
		}
		return NsBlockCommitSig, m
	case bytes.Equal(key, pLastCommits):
		return NsLastCommits, Meta{}
	case bytes.HasPrefix(key, pPendingCL):
		return NsPendingCrossLink, Meta{}
	case bytes.HasPrefix(key, pPendingSC):
		return NsPendingSlashing, Meta{}
	case bytes.HasPrefix(key, pCXSpent): // longer than pCXReceipt/pCX; first
		m := Meta{}
		if rest := key[len(pCXSpent):]; len(rest) == 12 {
			m.ShardID = be4(rest[:4])
			m.Number = be8(rest[4:])
		}
		return NsCXReceiptSpent, m
	case bytes.HasPrefix(key, pCXReceipt) && len(key) == len(pCXReceipt)+4+8+32:
		m := Meta{ShardID: be4(key[len(pCXReceipt) : len(pCXReceipt)+4]), Number: be8(key[len(pCXReceipt)+4 : len(pCXReceipt)+12])}
		return NsCXReceipt, m
	case bytes.HasPrefix(key, pCX) && len(key) == 2+32:
		return NsCXLookup, Meta{}
	case bytes.HasPrefix(key, pCL) && len(key) == 2+4:
		return NsCrossLinkPointer, Meta{ShardID: be4(key[2:])}
	case bytes.HasPrefix(key, pCL) && len(key) == 2+4+8:
		return NsCrossLink, Meta{ShardID: be4(key[2:6]), Number: be8(key[6:])}
	case bytes.HasPrefix(key, pSS):
		return NsShardState, epochMeta(key[len(pSS):])
	case bytes.HasPrefix(key, pSecureKey):
		return NsPreimage, Meta{}
	case bytes.HasPrefix(key, pConfig), bytes.HasPrefix(key, pGenesis):
		return NsConfig, Meta{}
	case bytes.HasPrefix(key, pBloomIdx):
		return NsBloom, Meta{}
	case len(key) == 32:
		return NsStateNode, Meta{}
	case len(key) == 1+8+32 && key[0] == 'h':
		return NsHeader, Meta{Number: be8(key[1:9])}
	case len(key) == 1+8+32+1 && key[0] == 'h' && key[41] == 't':
		return NsHeaderTD, Meta{Number: be8(key[1:9])}
	case len(key) == 1+8+1 && key[0] == 'h' && key[9] == 'n':
		return NsCanonicalHash, Meta{Number: be8(key[1:9])}
	case len(key) == 1+32 && key[0] == 'H':
		return NsHeaderNumber, Meta{}
	case len(key) == 1+8+32 && key[0] == 'b':
		return NsBody, Meta{Number: be8(key[1:9])}
	case len(key) == 1+8+32 && key[0] == 'r':
		return NsReceipts, Meta{Number: be8(key[1:9])}
	case len(key) == 1+32 && key[0] == 'l':
		return NsTxLookup, Meta{}
	case len(key) == 1+32 && key[0] == 'c':
		return NsCode, Meta{}
	case len(key) == 2+32 && key[0] == 'v' && key[1] == 'c':
		return NsValidatorCode, Meta{}
	case len(key) == 1+8 && key[0] == 'S':
		return NsSkeleton, Meta{Number: be8(key[1:])}
	case len(key) == 1+2+8+32 && key[0] == 'B':
		return NsBloom, Meta{}
	case bytes.Equal(key, []byte("continuous")):
		return NsLeaderContinuous, Meta{}
	}
	for _, hk := range headKeys {
		if bytes.Equal(key, hk) {
			return NsHead, Meta{}
		}
	}
	for _, sk := range syncEraKeys {
		if bytes.Equal(key, sk) {
			return NsSyncEra, Meta{}
		}
	}
	return NsOther, Meta{}
}

// KeyShapeError reports a key inside a namespace prefix whose shape is
// invalid (wrong component lengths), with the offending key hex.
func KeyShapeError(ns Namespace, key []byte) error {
	return fmt.Errorf("strictdb: malformed %s key %x (len %d)", ns, key, len(key))
}
