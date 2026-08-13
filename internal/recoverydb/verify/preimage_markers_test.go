package verify

import (
	"encoding/binary"
	"strings"
	"testing"
)

func be8(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

// TestValidatePreimageMarkers pins the operator-decided marker contract
// (round 16 findings 1 and 2): the pair is required complete or absent,
// with exact values validated separately from the digest exclusion.
func TestValidatePreimageMarkers(t *testing.T) {
	const target = uint64(22)
	cases := []struct {
		name    string
		s       preimageMarkerState
		wantErr string // empty = must pass
	}{
		{"bothAbsent", preimageMarkerState{}, ""},
		{"completePairFullCoverage", preimageMarkerState{
			startPresent: true, endPresent: true, start: be8(1), end: be8(target)}, ""},
		{"completePairFreshStart", preimageMarkerState{
			startPresent: true, endPresent: true, start: be8(target + 1), end: be8(target)}, ""},
		{"startOnly", preimageMarkerState{
			startPresent: true, start: be8(1)}, "incomplete preimage marker pair"},
		{"endOnly", preimageMarkerState{
			endPresent: true, end: be8(target)}, "incomplete preimage marker pair"},
		{"endWrongHeight", preimageMarkerState{
			startPresent: true, endPresent: true, start: be8(1), end: be8(target + 7)}, "!= pinned target"},
		{"endMalformed", preimageMarkerState{
			startPresent: true, endPresent: true, start: be8(1), end: []byte{1, 2}}, "!= pinned target"},
		{"startWrongValue", preimageMarkerState{
			startPresent: true, endPresent: true, start: be8(5), end: be8(target)}, "neither 1 nor target+1"},
		{"startMalformed", preimageMarkerState{
			startPresent: true, endPresent: true, start: []byte{9}, end: be8(target)}, "malformed preimage-gen-start"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validatePreimageMarkers(tc.s, target)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("must pass, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
