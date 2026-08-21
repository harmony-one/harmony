package consensus

import (
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/pkg/errors"
)

// blocksCountAliveness is how many recently committed blocks are inspected to decide
// whether a leader candidate is still taking part in consensus. It also sets how many
// blocks in a row a leader produces before rotation moves on.
const blocksCountAliveness = 4

// headerByNumberReader is the part of the chain an aliveness check reads.
type headerByNumberReader interface {
	GetHeaderByNumber(number uint64) *block.Header
}

// signedRecently reports whether candidate appears in the commit bitmap of at least one
// of the last blocksCountAliveness committed blocks of curEpoch. One signature in the
// window is enough, so the answer tracks whether a member is taking part at all rather
// than how reliably it signs.
//
// It reads committed headers and nothing else, so two nodes sharing a chain tip reach
// the same answer and leader selection stays deterministic. Headers belonging to an
// earlier epoch end the scan, since a member's signatures only start once it is seated.
func signedRecently(
	bc headerByNumberReader, members []bls.PublicKeyWrapper,
	candidate *bls.PublicKeyWrapper, curNumber, curEpoch uint64,
) (bool, error) {
	mask := bls.NewMask(members)
	skipped := 0
	for j := 0; j < blocksCountAliveness; j++ {
		header := bc.GetHeaderByNumber(curNumber - uint64(j))
		if header == nil {
			return false, errors.Errorf(
				"failed to get header by number %d", curNumber-uint64(j),
			)
		}
		// if epoch is different, we should not check this block.
		if header.Epoch().Uint64() != curEpoch {
			break
		}
		// Populate the mask with the bitmap.
		if err := mask.SetMask(header.LastCommitBitmap()); err != nil {
			return false, errors.Wrap(err, "failed to set mask")
		}
		ok, err := mask.KeyEnabled(candidate.Bytes)
		if err != nil {
			return false, errors.Wrap(err, "failed to get key enabled")
		}
		if !ok {
			skipped++
		}
	}
	return skipped < blocksCountAliveness, nil
}
