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
