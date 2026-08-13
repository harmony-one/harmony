package chainread

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/recovery/inplace/anchor"
	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

// Outcome carries what the chain checks proved, feeding the certificate
// check and the state walk.
type Outcome struct {
	TargetHeader   *block.Header
	StateRoot      common.Hash
	ViewID         uint64
	BoundaryHeader *block.Header
	ShardState     *shard.State

	// ChildHeader is the header at target height+1 whose parent is the
	// target (certificate source B), nil when absent.
	ChildHeader *block.Header

	// ChildSourceErr is set when the canonical slot at target+1 holds a
	// present-but-malformed header: undecodable bytes, or content that does
	// not recompute to its canonical hash. Certificate verification fails
	// closed on it - a present source must verify, and malformed stored
	// bytes must not be papered over as absence.
	ChildSourceErr string

	// Head sample rows (informational, never gate).
	Head report.HeadSample

	// Checks records "ok"/"fail: ..."/"skipped" per check id as the ordered
	// checks progress.
	Checks map[string]string
}

// maxUpwardWalk bounds the informational head-to-target walk (the incident
// cites ~21k abandoned blocks above the target; walk a bit further, never
// unbounded).
const maxUpwardWalk = 1 << 18

// RunChecks performs the ordered chain checks (target tuple, body,
// downward ancestry, shard state, upward head sample). Certificate
// verification (check 6) lives in certverify and consumes this Outcome.
// The returned error is a *report.Failure for verification FAILs, or a read
// error to be classified by the retry runner.
func RunChecks(kv ethdb.KeyValueReader, a *anchor.Anchor, progress io.Writer) (*Outcome, error) {
	out := &Outcome{Checks: report.NewChecks()}

	// mark records a verification FAIL against a check id; read errors are
	// NOT verification results and leave the check "skipped" (the stage may
	// be retried against a fresh manifest generation).
	mark := func(id string, err error) error {
		if f, ok := err.(*report.Failure); ok {
			out.Checks[id] = "fail: " + f.Reason
		}
		return err
	}

	// Check 1: target tuple.
	if err := out.checkTargetHeader(kv, a); err != nil {
		return out, mark("target_header", err)
	}
	out.Checks["target_header"] = "ok"

	// Check 2: body integrity.
	if err := out.checkBody(kv, a); err != nil {
		return out, mark("body", err)
	}
	out.Checks["body"] = "ok"

	// Check 3: downward ancestry walk to the epoch boundary.
	if err := out.walkAncestryToBoundary(kv, a, progress); err != nil {
		return out, mark("ancestry_to_boundary", err)
	}
	out.Checks["ancestry_to_boundary"] = "ok"

	// Check 4: ss<epoch> byte-equality against the walk-authenticated
	// boundary header.
	if err := out.checkShardState(kv, a); err != nil {
		return out, mark("shard_state", err)
	}
	out.Checks["shard_state"] = "ok"

	// Check 5: upward sample - informational, never gates. Read errors on a
	// moving head are recorded as strings, not returned (the head is
	// expected to move on a live DB), and they must not dirty the shared
	// read-error latch either - a latched head error would send an
	// otherwise clean run into the retry machinery and exit 3. When the
	// reader can scope its latch (the rodb adapter), sample through an
	// unlatched view.
	informational := kv
	if u, ok := kv.(interface{ Unlatched() ethdb.KeyValueReader }); ok {
		informational = u.Unlatched()
	}
	out.sampleHeads(informational, a, progress)

	// Locate the child header for certificate source B.
	if err := out.locateChild(kv, a); err != nil {
		return out, err
	}
	return out, nil
}

func (o *Outcome) checkTargetHeader(kv ethdb.KeyValueReader, a *anchor.Anchor) error {
	canonical, found, err := ReadCanonicalHash(kv, a.TargetHeight)
	if err != nil {
		return err
	}
	if !found {
		return report.Failf("target_header", "no canonical hash at target height %d", a.TargetHeight)
	}
	if canonical != a.TargetHash {
		return report.Failf("target_header", "canonical hash at %d is %s, want anchored %s", a.TargetHeight, canonical.Hex(), a.TargetHash.Hex())
	}
	num, found, err := ReadHeaderNumber(kv, a.TargetHash)
	if err != nil {
		return err
	}
	if !found {
		return report.Failf("target_header", "no reverse number mapping for target hash %s", a.TargetHash.Hex())
	}
	if num != a.TargetHeight {
		return report.Failf("target_header", "reverse mapping of %s is height %d, want %d", a.TargetHash.Hex(), num, a.TargetHeight)
	}
	header, found, err := ReadHeader(kv, a.TargetHeight, a.TargetHash)
	if err != nil {
		if de, ok := err.(*DecodeErr); ok {
			return report.Failf("target_header", "%v", de)
		}
		return err
	}
	if !found {
		return report.Failf("target_header", "target header %d %s not present", a.TargetHeight, a.TargetHash.Hex())
	}
	// The recomputed hash pins the entire header content to the anchor.
	if got := header.Hash(); got != a.TargetHash {
		return report.Failf("target_header", "stored target header recomputes to %s, want anchored %s", got.Hex(), a.TargetHash.Hex())
	}
	if header.Number() == nil || header.Number().Uint64() != a.TargetHeight {
		return report.Failf("target_header", "target header embeds number %v, want %d", header.Number(), a.TargetHeight)
	}
	if header.ShardID() != a.ShardID {
		return report.Failf("target_header", "target header shard %d, want %d", header.ShardID(), a.ShardID)
	}
	if header.Epoch() == nil || header.Epoch().Cmp(a.Epoch) != 0 {
		return report.Failf("target_header", "target header epoch %v, want %s (schedule cross-check)", header.Epoch(), a.Epoch)
	}
	o.TargetHeader = header
	o.StateRoot = header.Root()
	o.ViewID = header.ViewID().Uint64()
	return nil
}

func (o *Outcome) checkBody(kv ethdb.KeyValueReader, a *anchor.Anchor) error {
	body, found, err := ReadBody(kv, a.TargetHeight, a.TargetHash)
	if err != nil {
		if de, ok := err.(*DecodeErr); ok {
			return report.Failf("body", "%v", de)
		}
		return err
	}
	if !found {
		return report.Failf("body", "target block body not present")
	}
	txs := types.Transactions(body.Transactions())
	stks := staking.StakingTransactions(body.StakingTransactions())
	if got := types.DeriveSha(txs, stks); got != o.TargetHeader.TxHash() {
		return report.Failf("body", "transaction root mismatch: derived %s, header %s", got.Hex(), o.TargetHeader.TxHash().Hex())
	}
	incoming := types.CXReceiptsProofs(body.IncomingReceipts())
	if got := types.DeriveSha(incoming); got != o.TargetHeader.IncomingReceiptHash() {
		return report.Failf("body", "incoming-receipt root mismatch: derived %s, header %s", got.Hex(), o.TargetHeader.IncomingReceiptHash().Hex())
	}
	// The v3 header has no UncleHash field, so stored uncles are
	// unauthenticated bytes; the check gates on emptiness.
	if n := len(body.Uncles()); n != 0 {
		return report.Failf("body", "target block body carries %d uncles, want 0 (uncles are unauthenticated)", n)
	}
	return nil
}

func (o *Outcome) walkAncestryToBoundary(kv ethdb.KeyValueReader, a *anchor.Anchor, progress io.Writer) error {
	steps := a.TargetHeight - a.BoundaryHeight
	current := o.TargetHeader
	for n := a.TargetHeight; n > a.BoundaryHeight; n-- {
		parentHash := current.ParentHash()
		parentNum := n - 1
		parent, found, err := ReadHeader(kv, parentNum, parentHash)
		if err != nil {
			if de, ok := err.(*DecodeErr); ok {
				return report.Failf("ancestry_to_boundary", "%v", de)
			}
			return err
		}
		if !found {
			return report.Failf("ancestry_to_boundary", "broken parent link: header %d %s (parent of %d) not present", parentNum, parentHash.Hex(), n)
		}
		// Recompute the parent hash: a valid-RLP-wrong-content header under
		// the right key must not pass.
		if got := parent.Hash(); got != parentHash {
			return report.Failf("ancestry_to_boundary", "header stored at %d %s recomputes to %s", parentNum, parentHash.Hex(), got.Hex())
		}
		if parent.Number() == nil || parent.Number().Uint64() != parentNum {
			return report.Failf("ancestry_to_boundary", "header %s embeds number %v, want %d", parentHash.Hex(), parent.Number(), parentNum)
		}
		// Canonical-mapping agreement: long-final heights must agree even on
		// a live DB.
		canonical, found, err := ReadCanonicalHash(kv, parentNum)
		if err != nil {
			return err
		}
		if !found {
			return report.Failf("ancestry_to_boundary", "no canonical mapping at height %d", parentNum)
		}
		if canonical != parentHash {
			return report.Failf("ancestry_to_boundary", "canonical mapping at %d is %s, ancestry expects %s", parentNum, canonical.Hex(), parentHash.Hex())
		}
		current = parent
		if progress != nil && (a.TargetHeight-parentNum)%8192 == 0 {
			fmt.Fprintf(progress, "ancestry walk: %d/%d parent steps\n", a.TargetHeight-parentNum, steps)
		}
	}
	o.BoundaryHeader = current
	if progress != nil {
		fmt.Fprintf(progress, "ancestry walk: authenticated boundary header %d %s (%d steps)\n",
			a.BoundaryHeight, current.Hash().Hex(), steps)
	}
	return nil
}

func (o *Outcome) checkShardState(kv ethdb.KeyValueReader, a *anchor.Anchor) error {
	raw, found, err := ReadShardStateBytes(kv, a.Epoch)
	if err != nil {
		return err
	}
	if !found {
		return report.Failf("shard_state", "ss<%s> record not present", a.Epoch)
	}
	boundaryBytes := o.BoundaryHeader.ShardState()
	if len(boundaryBytes) == 0 {
		return report.Failf("shard_state", "boundary header %d carries no shard state", a.BoundaryHeight)
	}
	if !bytes.Equal(raw, boundaryBytes) {
		return report.Failf("shard_state", "ss<%s> record (%d bytes) differs from boundary header %d ShardState (%d bytes)",
			a.Epoch, len(raw), a.BoundaryHeight, len(boundaryBytes))
	}
	decoded, err := shard.DecodeWrapper(raw)
	if err != nil {
		return report.Failf("shard_state", "ss<%s> does not decode: %v", a.Epoch, err)
	}
	if decoded.Epoch == nil || decoded.Epoch.Cmp(a.Epoch) != 0 {
		return report.Failf("shard_state", "ss<%s> decodes with epoch %v, want %s", a.Epoch, decoded.Epoch, a.Epoch)
	}
	committee, err := decoded.FindCommitteeByID(a.ShardID)
	if err != nil {
		return report.Failf("shard_state", "ss<%s> has no committee for shard %d: %v", a.Epoch, a.ShardID, err)
	}
	if len(committee.Slots) == 0 {
		return report.Failf("shard_state", "ss<%s> committee for shard %d is empty", a.Epoch, a.ShardID)
	}
	o.ShardState = decoded
	return nil
}

// sampleHeads reads the head pointers and walks head-to-target parent links.
// Informational only: on a live DB heads move and abandoned-branch shapes
// vary - gating here would produce false FAILs, and target-block eligibility
// does not depend on what sits above the target. All errors are recorded as
// strings, never returned.
func (o *Outcome) sampleHeads(kv ethdb.KeyValueReader, a *anchor.Anchor, progress io.Writer) {
	headHeader, foundHH, errHH := ReadHeadPointer(kv, HeadHeaderKey)
	switch {
	case errHH != nil:
		o.Head.LastHeader = "read-error: " + errHH.Error()
	case !foundHH:
		o.Head.LastHeader = "absent"
	default:
		o.Head.LastHeader = headHeader.Hex()
	}
	headBlock, foundHB, errHB := ReadHeadPointer(kv, HeadBlockKey)
	switch {
	case errHB != nil:
		o.Head.LastBlock = "read-error: " + errHB.Error()
	case !foundHB:
		o.Head.LastBlock = "absent"
	default:
		o.Head.LastBlock = headBlock.Hex()
	}

	start := common.Hash{}
	if foundHH && errHH == nil {
		start = headHeader
	} else if foundHB && errHB == nil {
		start = headBlock
	}
	if start == (common.Hash{}) {
		o.Head.WalkToTarget = "not-walked: no resolvable head pointer"
		return
	}
	num, found, err := ReadHeaderNumber(kv, start)
	if err != nil || !found {
		o.Head.WalkToTarget = "not-walked: head has no number mapping"
		return
	}
	if num < a.TargetHeight {
		o.Head.WalkToTarget = fmt.Sprintf("head-below-target: head height %d < target %d", num, a.TargetHeight)
		return
	}
	if num-a.TargetHeight > maxUpwardWalk {
		o.Head.WalkToTarget = fmt.Sprintf("not-walked: head %d is %d blocks above target (bound %d)", num, num-a.TargetHeight, maxUpwardWalk)
		return
	}
	if progress != nil {
		fmt.Fprintf(progress, "head sample: walking %d parent steps from head %d to target height\n", num-a.TargetHeight, num)
	}
	cursor, cursorNum := start, num
	for cursorNum > a.TargetHeight {
		h, found, err := ReadHeader(kv, cursorNum, cursor)
		if err != nil || !found {
			o.Head.WalkToTarget = fmt.Sprintf("walk-broken at height %d (%s)", cursorNum, cursor.Hex())
			return
		}
		cursor = h.ParentHash()
		cursorNum--
	}
	if cursor == a.TargetHash {
		o.Head.WalkToTarget = "reached-target"
	} else {
		o.Head.WalkToTarget = fmt.Sprintf("diverged: head ancestry at target height is %s, target is %s", cursor.Hex(), a.TargetHash.Hex())
	}
}

// locateChild finds the header at target+1 whose parent is the target, for
// certificate source B. Absence is not an error; read errors propagate.
func (o *Outcome) locateChild(kv ethdb.KeyValueReader, a *anchor.Anchor) error {
	childNum := a.TargetHeight + 1
	childHash, found, err := ReadCanonicalHash(kv, childNum)
	if err != nil {
		return err
	}
	if !found {
		o.Head.ChildAtTargetPlus = "absent"
		return nil
	}
	child, found, err := ReadHeader(kv, childNum, childHash)
	if err != nil {
		if de, ok := err.(*DecodeErr); ok {
			o.Head.ChildAtTargetPlus = fmt.Sprintf("undecodable header at %d (%s)", childNum, childHash.Hex())
			o.ChildSourceErr = fmt.Sprintf("header at %d (%s) does not decode: %v", childNum, childHash.Hex(), de)
			return nil
		}
		return err
	}
	if !found {
		// The canonical mapping outliving the header is a transient shape
		// during above-target cleanup on a live DB: the source payload is
		// genuinely absent, not malformed.
		o.Head.ChildAtTargetPlus = fmt.Sprintf("canonical %s at %d without stored header", childHash.Hex(), childNum)
		return nil
	}
	if got := child.Hash(); got != childHash {
		o.Head.ChildAtTargetPlus = fmt.Sprintf("stored header at %d recomputes to %s, canonical says %s", childNum, got.Hex(), childHash.Hex())
		o.ChildSourceErr = fmt.Sprintf("header stored at %d recomputes to %s, canonical says %s", childNum, got.Hex(), childHash.Hex())
		return nil
	}
	if child.ParentHash() != a.TargetHash {
		// A well-formed foreign block at target+1: its commit signature
		// certifies a different parent, so it is not a certificate source
		// for the target (informational only).
		o.Head.ChildAtTargetPlus = fmt.Sprintf("present-not-child: %s (parent %s != target)", childHash.Hex(), child.ParentHash().Hex())
		return nil
	}
	o.Head.ChildAtTargetPlus = childHash.Hex()
	if a.Network == "mainnet" && childHash == common.HexToHash(anchor.MainnetAbandonedChildHashHex) {
		o.Head.ChildAtTargetPlus += " (matches memo abandoned-child hash)"
	}
	o.ChildHeader = child
	return nil
}
