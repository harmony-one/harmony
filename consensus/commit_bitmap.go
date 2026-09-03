package consensus

import (
	"math/bits"

	"github.com/harmony-one/harmony/crypto/bls"
)

// isMoreCompleteCommitPayload reports whether candidate has strictly more
// participating committee slots than current. Both payloads must use the same
// canonical signature-and-bitmap encoding; equal signer counts keep current.
func isMoreCompleteCommitPayload(current, candidate []byte, participantCount int) bool {
	if !hasCanonicalCommitBitmap(current, participantCount) ||
		!hasCanonicalCommitBitmap(candidate, participantCount) {
		return false
	}
	return commitPayloadSignerCount(candidate, participantCount) >
		commitPayloadSignerCount(current, participantCount)
}

// hasCanonicalCommitBitmap validates only the payload's structural encoding:
// its length must match one BLS signature followed by one bit per committee
// slot, and any unused high bits in the last bitmap byte must be zero. It does
// not verify the BLS signature or weighted quorum.
func hasCanonicalCommitBitmap(payload []byte, participantCount int) bool {
	bitmapLen := (participantCount + 7) / 8
	if participantCount <= 0 || len(payload) != bls.BLSSignatureSizeInBytes+bitmapLen {
		return false
	}
	if remainingBits := participantCount % 8; remainingBits != 0 {
		validBits := byte(1<<remainingBits) - 1
		bitmap := payload[bls.BLSSignatureSizeInBytes:]
		if bitmap[len(bitmap)-1]&^validBits != 0 {
			return false
		}
	}
	return true
}

// commitPayloadSignerCount counts enabled committee slots while ignoring the
// unused high bits in the last bitmap byte. The caller must first validate the
// payload with hasCanonicalCommitBitmap.
func commitPayloadSignerCount(payload []byte, participantCount int) int {
	bitmap := payload[bls.BLSSignatureSizeInBytes:]
	fullBytes := participantCount / 8
	count := 0
	for _, bitmapByte := range bitmap[:fullBytes] {
		count += bits.OnesCount8(bitmapByte)
	}
	if remainingBits := participantCount % 8; remainingBits != 0 {
		validBits := byte(1<<remainingBits) - 1
		count += bits.OnesCount8(bitmap[fullBytes] & validBits)
	}
	return count
}
