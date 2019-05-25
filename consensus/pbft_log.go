package consensus

import (
	"sync"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
)

// PbftLog represents the log stored by a node during PBFT process
type PbftLog struct {
	blocks     mapset.Set //store blocks received in PBFT
	messages   mapset.Set // store messages received in PBFT
	maxLogSize uint32
	mutex      sync.Mutex
}

// PbftMessage is the record of pbft messages received by a node during PBFT process
type PbftMessage struct {
	MessageType  msg_pb.MessageType
	ConsensusID  uint32
	SeqNum       uint64
	BlockHash    common.Hash
	SenderPubkey []byte
	Payload      []byte
}

// NewPbftLog returns new instance of PbftLog
func NewPbftLog() *PbftLog {
	blocks := mapset.NewSet()
	messages := mapset.NewSet()
	logSize := maxLogSize
	pbftLog := PbftLog{blocks: blocks, messages: messages, maxLogSize: logSize}
	return &pbftLog
}

// Blocks return the blocks stored in the log
func (log *PbftLog) Blocks() mapset.Set {
	return log.blocks
}

// Messages return the messages stored in the log
func (log *PbftLog) Messages() mapset.Set {
	return log.messages
}

// AddBlock add a new block into the log
func (log *PbftLog) AddBlock(block *types.Block) {
	log.blocks.Add(block)
}

// GetBlockByHash returns the block matches the given block hash
func (log *PbftLog) GetBlockByHash(hash common.Hash) *types.Block {
	var found *types.Block
	it := log.Blocks().Iterator()
	for block := range it.C {
		if block.(*types.Block).Header().Hash() == hash {
			found = block.(*types.Block)
			it.Stop()
		}
	}
	return found
}

// GetBlocksByNumber returns the blocks match the given block number
func (log *PbftLog) GetBlocksByNumber(number uint64) []*types.Block {
	found := []*types.Block{}
	it := log.Blocks().Iterator()
	for block := range it.C {
		if block.(*types.Block).NumberU64() == number {
			found = append(found, block.(*types.Block))
		}
	}
	return found
}

// DeleteBlocksByNumber deletes blocks match given block number
func (log *PbftLog) DeleteBlocksByNumber(number uint64) {
	found := mapset.NewSet()
	it := log.Blocks().Iterator()
	for block := range it.C {
		if block.(*types.Block).NumberU64() == number {
			found.Add(block)
		}
	}
	log.blocks = log.blocks.Difference(found)
}

// DeleteMessagesByNumber deletes messages match given block number
func (log *PbftLog) DeleteMessagesByNumber(number uint64) {
	found := mapset.NewSet()
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*PbftMessage).SeqNum == number {
			found.Add(msg)
		}
	}
	log.messages = log.messages.Difference(found)
}

// AddMessage adds a pbft message into the log
func (log *PbftLog) AddMessage(msg *PbftMessage) {
	log.messages.Add(msg)
}

// GetMessagesByTypeSeqViewHash returns pbft messages with matching type, seqNum, consensusID and blockHash
func (log *PbftLog) GetMessagesByTypeSeqViewHash(typ msg_pb.MessageType, seqNum uint64, consensusID uint32, blockHash common.Hash) []*PbftMessage {
	found := []*PbftMessage{}
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*PbftMessage).MessageType == typ && msg.(*PbftMessage).SeqNum == seqNum && msg.(*PbftMessage).ConsensusID == consensusID && msg.(*PbftMessage).BlockHash == blockHash {
			found = append(found, msg.(*PbftMessage))
		}
	}
	return found
}

// GetMessagesByTypeSeq returns pbft messages with matching type, seqNum
func (log *PbftLog) GetMessagesByTypeSeq(typ msg_pb.MessageType, seqNum uint64) []*PbftMessage {
	found := []*PbftMessage{}
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*PbftMessage).MessageType == typ && msg.(*PbftMessage).SeqNum == seqNum {
			found = append(found, msg.(*PbftMessage))
		}
	}
	return found
}

// GetMessagesByTypeSeqHash returns pbft messages with matching type, seqNum
func (log *PbftLog) GetMessagesByTypeSeqHash(typ msg_pb.MessageType, seqNum uint64, blockHash common.Hash) []*PbftMessage {
	found := []*PbftMessage{}
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*PbftMessage).MessageType == typ && msg.(*PbftMessage).SeqNum == seqNum && msg.(*PbftMessage).BlockHash == blockHash {
			found = append(found, msg.(*PbftMessage))
		}
	}
	return found
}

// HasMatchingAnnounce returns whether the log contains announce type message with given seqNum, consensusID and blockHash
func (log *PbftLog) HasMatchingAnnounce(seqNum uint64, consensusID uint32, blockHash common.Hash) bool {
	found := log.GetMessagesByTypeSeqViewHash(msg_pb.MessageType_ANNOUNCE, seqNum, consensusID, blockHash)
	return len(found) == 1
}

// GetMessagesByTypeSeqView returns pbft messages with matching type, seqNum and consensusID
func (log *PbftLog) GetMessagesByTypeSeqView(typ msg_pb.MessageType, seqNum uint64, consensusID uint32) []*PbftMessage {
	found := []*PbftMessage{}
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*PbftMessage).MessageType != typ || msg.(*PbftMessage).SeqNum != seqNum || msg.(*PbftMessage).ConsensusID != consensusID {
			continue
		}
		found = append(found, msg.(*PbftMessage))
	}
	return found
}

// ParsePbftMessage parses PBFT message into PbftMessage structure
func ParsePbftMessage(msg *msg_pb.Message) (*PbftMessage, error) {
	pbftMsg := PbftMessage{}
	pbftMsg.MessageType = msg.GetType()
	consensusMsg := msg.GetConsensus()

	pbftMsg.ConsensusID = consensusMsg.ConsensusId
	pbftMsg.SeqNum = consensusMsg.SeqNum
	pbftMsg.BlockHash = common.Hash{}
	copy(pbftMsg.BlockHash[:], consensusMsg.BlockHash[:])
	pbftMsg.Payload = make([]byte, len(consensusMsg.Payload))
	copy(pbftMsg.Payload[:], consensusMsg.Payload[:])
	pbftMsg.SenderPubkey = make([]byte, len(consensusMsg.SenderPubkey))
	copy(pbftMsg.SenderPubkey[:], consensusMsg.SenderPubkey[:])

	return &pbftMsg, nil
}
