package verify

import (
	"encoding/binary"
	"fmt"
)

// preimageMarkerState is what the raw scan observed for the stock preimage
// bookkeeping pair. Values are copied out of the iterator's buffers.
type preimageMarkerState struct {
	startPresent, endPresent bool
	start, end               []byte
}

// validatePreimageMarkers enforces the OPERATOR-DECIDED preimage-marker
// contract (round 16 findings 1 and 2): the two stock bookkeeping keys are
// excluded from the logical digest (see DigestExcludedKey) and instead
// pinned here — they must appear as a COMPLETE pair or not at all, with
// exact values:
//
//   - preimage-gen-end must equal the pinned target: the stock node writes
//     it as the head height on every clean Stop
//     (core/blockchain_impl.go:1283), so any other height means the node
//     moved the head.
//   - preimage-gen-start must be 1 (full coverage carried by the artifact)
//     or target+1 (the preimage-enabled stock open path found none and
//     starts fresh above the head, core/blockchain_impl.go:377).
//
// A lone half is refused: the compact artifact carries neither key, and a
// stock boot cycle with preimages enabled (the default, cmd/config/
// default.go defaultCacheConfig) writes both — start on open, end on Stop —
// so a single marker is evidence of a torn write, a preimages-disabled
// non-default configuration, or tampering, none of which this verifier
// accepts silently.
func validatePreimageMarkers(s preimageMarkerState, target uint64) error {
	if !s.startPresent && !s.endPresent {
		return nil
	}
	if s.startPresent != s.endPresent {
		return fmt.Errorf(
			"incomplete preimage marker pair: gen-start present=%v, gen-end present=%v (must be both or neither)",
			s.startPresent, s.endPresent)
	}
	if len(s.end) != 8 || binary.BigEndian.Uint64(s.end) != target {
		return fmt.Errorf("preimage-gen-end marker %x != pinned target %d", s.end, target)
	}
	if len(s.start) != 8 {
		return fmt.Errorf("malformed preimage-gen-start marker %x", s.start)
	}
	if v := binary.BigEndian.Uint64(s.start); v != 1 && v != target+1 {
		return fmt.Errorf("preimage-gen-start marker %d is neither 1 nor target+1 (%d)", v, target+1)
	}
	return nil
}
