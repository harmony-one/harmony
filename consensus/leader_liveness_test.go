package consensus

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/crypto/bls"
	bls_core "github.com/harmony-one/harmony/crypto/bls/core"
	"github.com/stretchr/testify/require"
)

// headerChain answers GetHeaderByNumber from a fixed map, and returns nil for anything
// it does not hold, which is how a node behind the tip behaves.
type headerChain map[uint64]*block.Header

func (h headerChain) GetHeaderByNumber(number uint64) *block.Header {
	return h[number]
}

func livenessTestKeys(t *testing.T, n int) []bls.PublicKeyWrapper {
	t.Helper()
	keys := make([]bls.PublicKeyWrapper, n)
	for i := range keys {
		private := bls_core.SecretKey{}
		private.SetByCSPRNG()
		public := private.GetPublicKey()
		keys[i].Object = public
		copy(keys[i].Bytes[:], public.Serialize())
	}
	return keys
}

// livenessChain builds `count` consecutive headers ending at `tip`, whose commit bitmaps
// enable exactly the members named in signersPerBlock[i] for the header at tip-i.
func livenessChain(
	t *testing.T, members []bls.PublicKeyWrapper,
	tip uint64, epoch *big.Int, signersPerBlock [][]int,
) headerChain {
	t.Helper()
	chain := headerChain{}
	for i, signers := range signersPerBlock {
		mask := bls.NewMask(members)
		for _, idx := range signers {
			require.NoError(t, mask.SetKey(members[idx].Bytes, true))
		}
		number := tip - uint64(i)
		chain[number] = blockfactory.ForTest.NewHeader(epoch).With().
			Number(new(big.Int).SetUint64(number)).
			Epoch(epoch).
			LastCommitBitmap(mask.Bitmap).
			Header()
	}
	return chain
}

func TestSignedRecently(t *testing.T) {
	members := livenessTestKeys(t, 4)
	const tip = uint64(100)
	epoch := big.NewInt(7)

	for _, tc := range []struct {
		name      string
		signers   [][]int
		candidate int
		want      bool
	}{
		{
			name:      "signed every recent block",
			signers:   [][]int{{0, 1}, {0, 1}, {0, 1}, {0, 1}},
			candidate: 0,
			want:      true,
		},
		{
			// One signature in the window is enough. The rule is meant to catch a node
			// that is gone, not one that dropped a block.
			name:      "signed only the oldest block in the window",
			signers:   [][]int{{1}, {1}, {1}, {0, 1}},
			candidate: 0,
			want:      true,
		},
		{
			name:      "absent from the whole window",
			signers:   [][]int{{1}, {1}, {1}, {1}},
			candidate: 0,
			want:      false,
		},
		{
			name:      "absent from all but one block outside the window",
			signers:   [][]int{{1}, {1}, {1}, {1}, {0}},
			candidate: 0,
			want:      false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := livenessChain(t, members, tip, epoch, tc.signers)
			alive, err := signedRecently(
				chain, members, &members[tc.candidate], tip, epoch.Uint64(),
			)
			require.NoError(t, err)
			require.Equal(t, tc.want, alive)
		})
	}
}

// Headers from a previous epoch end the scan rather than counting as misses: a member
// cannot be faulted for blocks produced before it held a seat.
func TestSignedRecentlyStopsAtEpochBoundary(t *testing.T) {
	members := livenessTestKeys(t, 4)
	const tip = uint64(100)
	epoch, prevEpoch := big.NewInt(7), big.NewInt(6)

	chain := livenessChain(t, members, tip, epoch, [][]int{{1}, {1}})
	older := livenessChain(t, members, tip-2, prevEpoch, [][]int{{1}, {1}})
	for number, header := range older {
		chain[number] = header
	}

	alive, err := signedRecently(chain, members, &members[0], tip, epoch.Uint64())
	require.NoError(t, err)
	require.True(t, alive,
		"only two in-epoch blocks were missed, which is short of the whole window")
}

// A node that cannot read back the window must report an error so callers fall back to
// plain index order instead of silently disagreeing with peers that can.
func TestSignedRecentlyErrorsOnMissingHeader(t *testing.T) {
	members := livenessTestKeys(t, 4)
	const tip = uint64(100)
	epoch := big.NewInt(7)

	chain := livenessChain(t, members, tip, epoch, [][]int{{0}, {0}})
	_, err := signedRecently(chain, members, &members[0], tip, epoch.Uint64())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get header by number")
}

// Near the start of a chain the window reaches past block zero, which reads as an
// unreadable window so callers keep the candidate they already have.
func TestSignedRecentlyNearChainStart(t *testing.T) {
	members := livenessTestKeys(t, 4)
	epoch := big.NewInt(0)
	const tip = uint64(1)

	chain := livenessChain(t, members, tip, epoch, [][]int{{0}, {0}})
	_, err := signedRecently(chain, members, &members[0], tip, epoch.Uint64())
	require.Error(t, err)
}

// A candidate outside the membership the mask is built from cannot be looked up, which
// callers treat the same as an unreadable window.
func TestSignedRecentlyRejectsNonMember(t *testing.T) {
	members := livenessTestKeys(t, 4)
	outsider := livenessTestKeys(t, 1)[0]
	const tip = uint64(100)
	epoch := big.NewInt(7)

	chain := livenessChain(t, members, tip, epoch, [][]int{{0}, {0}, {0}, {0}})
	_, err := signedRecently(chain, members, &outsider, tip, epoch.Uint64())
	require.Error(t, err)
}

// A bitmap sized for a different membership cannot be read into the mask, so the answer
// is an error rather than a guess at who signed.
func TestSignedRecentlyRejectsMismatchedBitmapWidth(t *testing.T) {
	members := livenessTestKeys(t, 4)
	const tip = uint64(100)
	epoch := big.NewInt(7)

	wider := append(append([]bls.PublicKeyWrapper{}, members...), livenessTestKeys(t, 16)...)
	chain := livenessChain(t, wider, tip, epoch, [][]int{{0}, {0}, {0}, {0}})

	_, err := signedRecently(chain, members, &members[0], tip, epoch.Uint64())
	require.Error(t, err)
}

// Every header in the window belonging to an earlier epoch leaves nothing measured, and
// a candidate with no signatures to its name yet counts as taking part.
func TestSignedRecentlyWithWholeWindowInPriorEpoch(t *testing.T) {
	members := livenessTestKeys(t, 4)
	const tip = uint64(100)
	epoch, prevEpoch := big.NewInt(7), big.NewInt(6)

	chain := livenessChain(t, members, tip, prevEpoch, [][]int{{1}, {1}, {1}, {1}})
	alive, err := signedRecently(chain, members, &members[0], tip, epoch.Uint64())
	require.NoError(t, err)
	require.True(t, alive)
}
