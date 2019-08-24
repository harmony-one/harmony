package consensus

import (
	"sync"
	"time"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

const (
	// RetryIntervalInSec is the interval for message retry
	RetryIntervalInSec = 10
)

// MessageSender is the wrapper object that controls how a consensus message is sent
type MessageSender struct {
	blockNum        uint64 // The current block number at consensus
	blockNumMutex   sync.Mutex
	messagesToRetry sync.Map
	// The p2p host used to send/receive p2p messages
	host p2p.Host
	// RetryTimes is number of retry attempts
	retryTimes int
}

// MessageRetry controls the message that can be retried
type MessageRetry struct {
	blockNum      uint64 // The block number this message is for
	groups        []p2p.GroupID
	p2pMsg        []byte
	msgType       msg_pb.MessageType
	retryCount    int
	isActive      bool
	isActiveMutex sync.Mutex
}

// NewMessageSender initializes the consensus message sender.
func NewMessageSender(host p2p.Host) *MessageSender {
	return &MessageSender{host: host, retryTimes: int(phaseDuration.Seconds()) / RetryIntervalInSec}
}

// Reset resets the sender's state for new block
func (sender *MessageSender) Reset(blockNum uint64) {
	sender.blockNumMutex.Lock()
	sender.blockNum = blockNum
	sender.blockNumMutex.Unlock()
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
func (sender *MessageSender) SendWithRetry(blockNum uint64, msgType msg_pb.MessageType, groups []p2p.GroupID, p2pMsg []byte) error {
	willRetry := sender.retryTimes != 0
	msgRetry := MessageRetry{blockNum: blockNum, groups: groups, p2pMsg: p2pMsg, msgType: msgType, retryCount: 0, isActive: willRetry}
	if willRetry {
		sender.messagesToRetry.Store(msgType, &msgRetry)
		go func() {
			sender.Retry(&msgRetry)
		}()
	}
	return sender.host.SendMessageToGroups(groups, p2pMsg)
}

// SendWithoutRetry sends message without retry logic.
func (sender *MessageSender) SendWithoutRetry(groups []p2p.GroupID, p2pMsg []byte) error {
	return sender.host.SendMessageToGroups(groups, p2pMsg)
}

// Retry will retry the consensus message for <RetryTimes> times.
func (sender *MessageSender) Retry(msgRetry *MessageRetry) {
	for {
		time.Sleep(RetryIntervalInSec * time.Second)

		if msgRetry.retryCount >= sender.retryTimes {
			// Retried enough times
			return
		}

		msgRetry.isActiveMutex.Lock()
		if !msgRetry.isActive {
			msgRetry.isActiveMutex.Unlock()
			// Retry is stopped
			return
		}
		msgRetry.isActiveMutex.Unlock()

		if msgRetry.msgType != msg_pb.MessageType_COMMITTED {
			sender.blockNumMutex.Lock()
			if msgRetry.blockNum < sender.blockNum {
				sender.blockNumMutex.Unlock()
				// Block already moved ahead, no need to retry old block's messages
				return
			}
			sender.blockNumMutex.Unlock()
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
		msgRetry.isActiveMutex.Lock()
		msgRetry.isActive = false
		msgRetry.isActiveMutex.Unlock()
	}
}

// StopAllRetriesExceptCommitted stops all the existing retries except committed message (which lives across consensus).
func (sender *MessageSender) StopAllRetriesExceptCommitted() {
	sender.messagesToRetry.Range(func(k, v interface{}) bool {
		if msgRetry, ok := v.(*MessageRetry); ok {
			if msgRetry.msgType != msg_pb.MessageType_COMMITTED {
				msgRetry.isActiveMutex.Lock()
				msgRetry.isActive = false
				msgRetry.isActiveMutex.Unlock()
			}
		}
		return true
	})
}
