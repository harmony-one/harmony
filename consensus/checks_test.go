package consensus

import (
	"testing"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/stretchr/testify/require"
)

// verifyMessageSig modifies the message signature when returns error
func TestVerifyMessageSig(t *testing.T) {
	message := &msg_pb.Message{
		Signature: []byte("signature"),
	}

	err := verifyMessageSig(nil, message)
	require.Error(t, err)
	require.Empty(t, message.Signature)
}

func TestIsTooFarAhead(t *testing.T) {
	tests := []struct {
		name     string
		myBlock  uint64
		msgBlock uint64
		want     bool
	}{
		{name: "equal", myBlock: 100, msgBlock: 100, want: false},
		{name: "next", myBlock: 100, msgBlock: 101, want: false},
		{name: "at cap", myBlock: 100, msgBlock: 100 + MaxBlockNumDiff, want: false},
		{name: "over cap", myBlock: 100, msgBlock: 100 + MaxBlockNumDiff + 1, want: true},
		{name: "large gap", myBlock: 100, msgBlock: 1_000_000, want: true},
		{name: "older", myBlock: 100, msgBlock: 99, want: false},
		{name: "from zero at cap", myBlock: 0, msgBlock: MaxBlockNumDiff, want: false},
		{name: "from zero over cap", myBlock: 0, msgBlock: MaxBlockNumDiff + 1, want: true},
		{name: "near max uint", myBlock: ^uint64(0) - 10, msgBlock: ^uint64(0), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isTooFarAhead(tt.myBlock, tt.msgBlock))
		})
	}
}
