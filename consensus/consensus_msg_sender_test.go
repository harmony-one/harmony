package consensus

import (
	"sync/atomic"
	"testing"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/test/helpers"
	"github.com/stretchr/testify/assert"
)

func TestMessageSenderInitialization(t *testing.T) {
	hostData := helpers.Hosts[0]
	host, _, err := helpers.GenerateHost(hostData.IP, hostData.Port)
	assert.NoError(t, err)

	messageSender := NewMessageSender(host)
	expectedMessageSender := &MessageSender{blockNum: 0, host: host, retryTimes: int(phaseDuration.Seconds()) / RetryIntervalInSec}
	assert.Equal(t, expectedMessageSender.host, messageSender.host)
	assert.Equal(t, expectedMessageSender.retryTimes, messageSender.retryTimes)
	assert.Equal(t, uint64(0), messageSender.blockNum)

	assert.Equal(t, 0, numberOfMessagesToRetry(messageSender))
}

func TestMessageSenderReset(t *testing.T) {
	hostData := helpers.Hosts[0]
	host, _, err := helpers.GenerateHost(hostData.IP, hostData.Port)
	assert.NoError(t, err)

	messageSender := NewMessageSender(host)
	assert.Equal(t, uint64(0), messageSender.blockNum)
	assert.Equal(t, 0, numberOfMessagesToRetry(messageSender))

	groups := []nodeconfig.GroupID{
		"hmy/testnet/0.0.1/client/beacon",
		"hmy/testnet/0.0.1/node/beacon",
	}
	p2pMsg := []byte{0}
	msgType := msg_pb.MessageType_ANNOUNCE
	msgRetry := MessageRetry{blockNum: 1, groups: groups, p2pMsg: p2pMsg, msgType: msgType, retryCount: 0}
	atomic.StoreUint32(&msgRetry.isActive, 1)
	messageSender.messagesToRetry.Store(msgType, &msgRetry)
	assert.Equal(t, 1, numberOfMessagesToRetry(messageSender))

	messageSender.Reset(1)
	assert.Equal(t, uint64(1), messageSender.blockNum)
	assert.Equal(t, 0, numberOfMessagesToRetry(messageSender))

	msgType = msg_pb.MessageType_COMMITTED
	msgRetry = MessageRetry{blockNum: 2, groups: groups, p2pMsg: p2pMsg, msgType: msgType, retryCount: 0}
	atomic.StoreUint32(&msgRetry.isActive, 1)
	messageSender.messagesToRetry.Store(msgType, &msgRetry)

	messageSender.Reset(2)
	assert.Equal(t, uint64(2), messageSender.blockNum)
	assert.Equal(t, 1, numberOfMessagesToRetry(messageSender))
}

func numberOfMessagesToRetry(messageSender *MessageSender) int {
	messagesToRetryCount := 0
	messageSender.messagesToRetry.Range(func(_, _ interface{}) bool {
		messagesToRetryCount++
		return true
	})

	return messagesToRetryCount
}
