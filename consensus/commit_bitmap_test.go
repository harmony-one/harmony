package consensus

import (
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"
)

func testCommitPayload(bitmap ...byte) []byte {
	payload := make([]byte, bls.BLSSignatureSizeInBytes+len(bitmap))
	copy(payload[bls.BLSSignatureSizeInBytes:], bitmap)
	return payload
}

func TestIsMoreCompleteCommitPayload(t *testing.T) {
	tests := []struct {
		name             string
		current          []byte
		candidate        []byte
		participantCount int
		want             bool
	}{
		{
			name:             "more signers wins",
			current:          testCommitPayload(0b00000111),
			candidate:        testCommitPayload(0b00001111),
			participantCount: 8,
			want:             true,
		},
		{
			name:             "fewer signers cannot downgrade",
			current:          testCommitPayload(0b00001111),
			candidate:        testCommitPayload(0b00000111),
			participantCount: 8,
		},
		{
			name:             "equal signer count keeps current",
			current:          testCommitPayload(0b00001111),
			candidate:        testCommitPayload(0b11110000),
			participantCount: 8,
		},
		{
			name:             "different bitmap size is incompatible",
			current:          testCommitPayload(0b00000001),
			candidate:        testCommitPayload(0b11111111, 0b00000001),
			participantCount: 8,
		},
		{
			name:             "noncanonical padding is rejected",
			current:          testCommitPayload(0b00000011, 0),
			candidate:        testCommitPayload(0b00000111, 0b11111110),
			participantCount: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMoreCompleteCommitPayload(tt.current, tt.candidate, tt.participantCount); got != tt.want {
				t.Fatalf("isMoreCompleteCommitPayload() = %t, want %t", got, tt.want)
			}
		})
	}
}
