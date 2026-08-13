package audit

import (
	"bytes"
	"fmt"

	"github.com/harmony-one/harmony/core/types"
)

// Legacy CommitBitmap corruption (the CXReceiptsProof.Copy bug, present
// since 2019-09-09, core/types/cx_receipt.go): rawdb.WriteBlock publishes
// the body through Block.Body() → SetIncomingReceipts → CXReceiptsProofs.Copy,
// which copies CommitSig INTO CommitBitmap. Every mainnet body stored with
// incoming receipts therefore carries a 96-byte copy of the aggregate
// signature where the quorum bitmap belongs, so a stored proof can never
// re-verify as-is (seal check needs the bitmap, and DeriveSha over the
// corrupted list no longer reproduces the header's IncomingReceiptHash).
// Fixing Copy itself is consensus-adjacent core code and out of scope for
// this recovery branch (no consensus/core behavior changes), so the repair
// below is strictly recovery-local: it operates on the in-memory block the
// audit read from the source DB and never touches stored bytes.
//
// The original bitmap is recoverable: the beacon DB stores the source
// shard's crosslink for the same source block, whose (signature, bitmap)
// pair is the commit signature material the proof carried. Restoration is
// VERIFIED, never assumed — the substituted list must reproduce the
// header's IncomingReceiptHash commitment (keccak-binding over the exact
// original proof bytes), otherwise every substitution is rolled back and
// the stored proof fails validation the normal way.

// crossLinkReader supplies stored crosslinks from the UNMASKED source: the
// restoration source must not depend on pass-2 masking (branch-written
// crosslink records are masked there) because bitmap restoration is a data
// repair on the stored bytes, not a chain-semantics check — its correctness
// is proven by the header commitment, not by where the bitmap came from.
type crossLinkReader interface {
	CrossLink(shardID uint32, blockNum uint64) (*types.CrossLink, error)
}

// restoreLegacyReceiptBitmaps repairs Copy-bug-corrupted CommitBitmaps on the
// block's incoming receipt proofs in place. Only proofs exhibiting the exact
// corruption signature (CommitBitmap byte-identical to the 96-byte CommitSig)
// are touched; the candidate bitmap comes from the stored crosslink of the
// proof's source block, gated on the crosslink covering the same source
// header hash. The whole substitution set is kept only if the restored list
// reproduces header.IncomingReceiptHash; otherwise it is rolled back and the
// proofs are left exactly as stored. Returns the number of restored proofs
// and diagnostic notes for proofs that matched the corruption signature but
// could not be verifiably restored.
func restoreLegacyReceiptBitmaps(src crossLinkReader, blk *types.Block) (restored int, notes []string) {
	cxps := blk.IncomingReceipts()
	if len(cxps) == 0 {
		return 0, nil
	}
	type undo struct {
		i   int
		old []byte
	}
	var undos []undo
	for i, cxp := range cxps {
		if cxp == nil || cxp.Header == nil || len(cxp.CommitSig) != 96 ||
			len(cxp.CommitBitmap) != 96 || !bytes.Equal(cxp.CommitBitmap, cxp.CommitSig) {
			continue
		}
		srcShard, srcNum := cxp.Header.ShardID(), cxp.Header.Number().Uint64()
		cl, err := src.CrossLink(srcShard, srcNum)
		if err != nil {
			notes = append(notes, fmt.Sprintf(
				"proof for shard-%d block %d matches the legacy Copy-bug corruption but its crosslink is unreadable: %v", srcShard, srcNum, err))
			continue
		}
		if cl == nil {
			notes = append(notes, fmt.Sprintf(
				"proof for shard-%d block %d matches the legacy Copy-bug corruption but no stored crosslink exists to restore the bitmap from", srcShard, srcNum))
			continue
		}
		if cl.Hash() != cxp.Header.Hash() {
			notes = append(notes, fmt.Sprintf(
				"proof for shard-%d block %d matches the legacy Copy-bug corruption but the stored crosslink covers a different source block (%s, proof %s)",
				srcShard, srcNum, cl.Hash().Hex(), cxp.Header.Hash().Hex()))
			continue
		}
		undos = append(undos, undo{i: i, old: cxp.CommitBitmap})
		cxps[i].CommitBitmap = append([]byte(nil), cl.Bitmap()...)
	}
	if len(undos) == 0 {
		return 0, notes
	}
	// The header's IncomingReceiptHash commits to the exact original proof
	// bytes; reproducing it proves every substituted bitmap is the original.
	if types.DeriveSha(cxps) == blk.Header().IncomingReceiptHash() {
		return len(undos), notes
	}
	for _, u := range undos {
		cxps[u.i].CommitBitmap = u.old
	}
	return 0, append(notes, fmt.Sprintf(
		"crosslink-restored bitmaps do not reproduce the header incoming-receipt commitment at block %d; substitutions rolled back, proofs left as stored", blk.NumberU64()))
}
