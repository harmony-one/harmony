package consensus

import (
	"sync"
	"sync/atomic"
	"time"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

const (
	// RetryIntervalInSec is the interval for message retry
	RetryIntervalInSec = 7
)

// MessageSender is the wrapper object that controls how a consensus message is sent
type MessageSender struct {
	blockNum        uint64 // The current block number at consensus
	messagesToRetry sync.Map
	// The p2p host used to send/receive p2p messages
	host p2p.Host
	// RetryTimes is number of retry attempts
	retryTimes int
}

// MessageRetry controls the message that can be retried
type MessageRetry struct {
	blockNum   uint64 // The block number this message is for
	groups     []nodeconfig.GroupID
	p2pMsg     []byte
	msgType    msg_pb.MessageType
	retryCount int
	isActive   uint32 // 0 is false, != 0 is true, in order to use atomic.Load/Store functions
}

// NewMessageSender initializes the consensus message sender.
func NewMessageSender(host p2p.Host) *MessageSender {
	return &MessageSender{host: host, retryTimes: int(phaseDuration.Seconds()) / RetryIntervalInSec}
}

// Reset resets the sender's state for new block
func (sender *MessageSender) Reset(blockNum uint64) {
	atomic.StoreUint64(&sender.blockNum, blockNum)
	sender.StopAllRetriesExceptCommitted()
	sender.messagesToRetry.Range(func(key interface{}, value interface{}) bool {
		if msgRetry, ok := value.(*MessageRetry); ok {
			if msgRetry.msgType != msg_pb.MessageType_COMMITTED {
				sender.messagesToRetry.Delete(key)
			}
		}
		return true
	})
}

// SendWithRetry sends message with retry logic.
func (sender *MessageSender) SendWithRetry(blockNum uint64, msgType msg_pb.MessageType, groups []nodeconfig.GroupID, p2pMsg []byte) error {
	if sender.retryTimes != 0 {
		msgRetry := MessageRetry{blockNum: blockNum, groups: groups, p2pMsg: p2pMsg, msgType: msgType, retryCount: 0}
		atomic.StoreUint32(&msgRetry.isActive, 1)
		// First stop the old one
		sender.StopRetry(msgType)
		sender.messagesToRetry.Store(msgType, &msgRetry)
		go func() {
			sender.Retry(&msgRetry)
		}()
	}
	// MessageSender lays inside consensus, but internally calls consensus public api.
	// Tt would be deadlock if run in current thread.
	go sender.host.SendMessageToGroups(groups, p2pMsg)
	return nil
}

// DelayedSendWithRetry is similar to SendWithRetry but without the initial message sending but only retries.
func (sender *MessageSender) DelayedSendWithRetry(blockNum uint64, msgType msg_pb.MessageType, groups []nodeconfig.GroupID, p2pMsg []byte) {
	if sender.retryTimes != 0 {
		msgRetry := MessageRetry{blockNum: blockNum, groups: groups, p2pMsg: p2pMsg, msgType: msgType, retryCount: 0}
		atomic.StoreUint32(&msgRetry.isActive, 1)
		// First stop the old one
		sender.StopRetry(msgType)
		sender.messagesToRetry.Store(msgType, &msgRetry)
		go func() {
			sender.Retry(&msgRetry)
		}()
	}
}

// SendWithoutRetry sends message without retry logic.
func (sender *MessageSender) SendWithoutRetry(groups []nodeconfig.GroupID, p2pMsg []byte) error {
	// MessageSender lays inside consensus, but internally calls consensus public api.
	// It would be deadlock if run in current thread.
	go sender.host.SendMessageToGroups(groups, p2pMsg)
	return nil
}

// Retry will retry the consensus message for <RetryTimes> times.
func (sender *MessageSender) Retry(msgRetry *MessageRetry) {
	for {
		time.Sleep(RetryIntervalInSec * time.Second)

		if msgRetry.retryCount >= sender.retryTimes {
			// Retried enough times
			return
		}

		isActive := atomic.LoadUint32(&msgRetry.isActive)
		if isActive == 0 {
			// Retry is stopped
			return
		}

		if msgRetry.msgType != msg_pb.MessageType_COMMITTED {
			senderBlockNum := atomic.LoadUint64(&sender.blockNum)
			if msgRetry.blockNum < senderBlockNum {
				// Block already moved ahead, no need to retry old block's messages
				return
			}
		}

		msgRetry.retryCount++
		if err := sender.host.SendMessageToGroups(msgRetry.groups, msgRetry.p2pMsg); err != nil {
			utils.Logger().Warn().Str("groupID[0]", msgRetry.groups[0].String()).Uint64("blockNum", msgRetry.blockNum).Str("MsgType", msgRetry.msgType.String()).Int("RetryCount", msgRetry.retryCount).Msg("[Retry] Failed re-sending consensus message")
		} else {
			utils.Logger().Info().Str("groupID[0]", msgRetry.groups[0].String()).Uint64("blockNum", msgRetry.blockNum).Str("MsgType", msgRetry.msgType.String()).Int("RetryCount", msgRetry.retryCount).Msg("[Retry] Successfully resent consensus message")
		}
	}
}

// StopRetry stops the retry.
func (sender *MessageSender) StopRetry(msgType msg_pb.MessageType) {
	data, ok := sender.messagesToRetry.Load(msgType)
	if ok {
		msgRetry := data.(*MessageRetry)
		atomic.StoreUint32(&msgRetry.isActive, 0)
	}
}

// StopAllRetriesExceptCommitted stops all the existing retries except committed message (which lives across consensus).
func (sender *MessageSender) StopAllRetriesExceptCommitted() {
	sender.messagesToRetry.Range(func(k, v interface{}) bool {
		if msgRetry, ok := v.(*MessageRetry); ok {
			if msgRetry.msgType != msg_pb.MessageType_COMMITTED {
				atomic.StoreUint32(&msgRetry.isActive, 0)
			}
		}
		return true
	})
}
