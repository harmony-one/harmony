package consensus

import (
	"time"

	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/internal/utils"
)

// Start is the entry point and main loop for consensus
func (consensus *Consensus) Start(stopChan chan struct{}, stoppedChan chan struct{}) {
	defer close(stoppedChan)
	tick := time.NewTicker(blockDuration)
	consensus.idleTimeout.Start()
	for {
		select {
		default:
			msg := consensus.recvWithTimeout(receiveTimeout)
			consensus.handleMessageUpdate(msg)
			if consensus.idleTimeout.CheckExpire() {
				consensus.startViewChange(consensus.consensusID + 1)
			}
			if consensus.commitTimeout.CheckExpire() {
				consensus.startViewChange(consensus.consensusID + 1)
			}
			if consensus.viewChangeTimeout.CheckExpire() {
				if consensus.mode.Mode() == Normal {
					continue
				}
				consensusID := consensus.mode.ConsensusID()
				consensus.startViewChange(consensusID + 1)
			}
		case <-tick.C:
			consensus.tryPublishBlock()
		case <-stopChan:
			return
		}
	}

}

// recvWithTimeout receives message before timeout
func (consensus *Consensus) recvWithTimeout(timeoutDuration time.Duration) []byte {
	var msg []byte
	select {
	case msg = <-consensus.MsgChan:
		utils.GetLogInstance().Debug("[Consensus] recvWithTimeout received", "msg", msg)
	case <-time.After(timeoutDuration):
		utils.GetLogInstance().Debug("[Consensus] recvWithTimeout timeout", "duration", timeoutDuration)
	}
	return msg
}

// handleMessageUpdate will update the consensus state according to received message
func (consensus *Consensus) handleMessageUpdate(payload []byte) {
	msg := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, msg)
	if err != nil {
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "consensus", consensus)
	}

	switch msg.Type {
	case msg_pb.MessageType_ANNOUNCE:
		consensus.onAnnounce(msg)
	case msg_pb.MessageType_PREPARE:
		consensus.onPrepare()
	case msg_pb.MessageType_PREPARED:
		consensus.onPrepared()
	case msg_pb.MessageType_COMMIT:
		consensus.onCommit()
	case msg_pb.MessageType_COMMITTED:
		consensus.onCommitted()
	case msg_pb.MessageType_VIEWCHANGE:
		consensus.onViewChange()
	case msg_pb.MessageType_NEWVIEW:
		consensus.onNewView()
	case msg_pb.MessageType_NEWBLOCK:
		consensus.onNewBlock()
	case msg_pb.MessageType_COMMITBLOCK:
		consensus.onCommitBlock()

	}

}

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	if consensus.IsLeader {
		return
	}
	consensus.processAnnounceMessage(msg)
	return
}

func (consensus *Consensus) onPrepare() {
	return
}
func (consensus *Consensus) onPrepared() {
	return
}
func (consensus *Consensus) onCommit() {
	return
}
func (consensus *Consensus) onCommitted() {
	return
}
func (consensus *Consensus) onViewChange() {
	return
}
func (consensus *Consensus) onNewView() {
	return
}

func (consensus *Consensus) onNewBlock() {
	return
}
func (consensus *Consensus) onCommitBlock() {
	return
}

// tryPublishBlock periodically check if block is finalized and then publish it
// TODO: cm
func (consensus *Consensus) tryPublishBlock() {
}
