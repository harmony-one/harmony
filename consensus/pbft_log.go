package consensus

import (
	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
)

// PbftLog represents the log stored by a node during PBFT process
type PbftLog struct {
	blocks     mapset.Set //store blocks received in PBFT
	messages   mapset.Set // store messages received in PBFT
	maxLogSize uint32
}

// PbftMessage is the record of pbft messages received by a node during PBFT process
type PbftMessage struct {
	MessageType  msg_pb.MessageType
	ConsensusID  uint32
	SeqNum       uint64
	BlockHash    common.Hash
	SenderPubkey []byte
	Payload      []byte
	Signature    *bls.Sign
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
		if block.(*types.Block).Hash() == hash {
			found = block.(*types.Block)
			it.Stop()
		}
	}
	return found
}

// GetBlockByNumber returns the blocks match the given block number
func (log *PbftLog) GetBlocksByNumber(id uint64) []*types.Block {
	var found []*types.Block
	it := log.Blocks().Iterator()
	for block := range it.C {
		if block.(*types.Block).NumberU64() == id {
			found = append(found, block.(*types.Block))
		}
	}
	return found
}

// AddMessage adds a pbft message into the log
func (log *PbftLog) AddMessage(msg *PbftMessage) {
	log.messages.Add(msg)
}

// GetMessagesByTypeSeqViewHash returns pbft messages with matching type, seqNum, consensusID and blockHash
func (log *PbftLog) GetMessagesByTypeSeqViewHash(typ msg_pb.MessageType, seqNum uint64, consensusID uint32, blockHash common.Hash) []*PbftMessage {
	var found []*types.Block
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*PbftMessage).MessageType == typ && msg.(*PbftMessage).SeqNum == seqNum && msg.(*PbftMessage).ConsensusID == consensusID && msg.(*PbftMessage).BlockHash == blockHash {
			found = append(found, msg.(*PbftMessage))
		}
	}
	return found
}

// HasMatchingAnnounce returns whether the log contains announce type message with given seqNum, consensusID and blockHash
func (log *PbftLog) HasMatchingAnnounce(seqNum uint64, consensusID uint32, blockHash common.Hash) bool {
	found := log.GetMessagesByTypeSeqViewHash(seqNum, consensusID, blockHash)
	return len(found) == 1
}

// GetMessagesByTypeSeqView returns pbft messages with matching type, seqNum and consensusID
func (log *PbftLog) GetMessagesByTypeSeqView(typ msg_pb.MessageType, seqNum uint64, consensusID uint32) []*PbftMessage {
	var found []*PbftMessage
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*PbftMessage).MessageType != typ || msg.(*PbftMessage).SeqNum != seqNum || msg.(*PbftMessage).ConsensusID != consensusID {
			continue
		}
		found = append(found, msg.(*PbftMessage))
	}
	return found
}

// ParsePbftmessage parses PBFT message into PbftMessage structure
func ParsePbftMessage(msg *msg_pb.Message) (*PbftMessage, error) {
	pbftMsg := PbftMessage{}
	pbftMsg.MessageType = msg.GetType()
	consensusMsg := msg.GetConsensus()

	pbftMsg.ConsensusID = consensusMsg.ConsensusId
	pbftMsg.SeqNum = consensusMsg.SeqNum
	pbftMsg.BlockHash = common.Hash{}
	copy(pbftMsg.BlockHash[:], consensusMsg.BlockHash[:])
	pbftMsg.Payload = make([]byte, len(consensusMsg.Payload))
	copy(pbftMsg.Payload, consensusMsg.Payload[:])
	pbftMsg.SenderPubkey = make([]byte, len(consensusMsg.SenderPubkey))
	copy(pbftMsg.SenderPubkey, consensusMsg.SenderPubkey[:])

	msgSig := bls.Sign{}
	err := msgSig.Deserialize(msg.GetSignature())
	if err != nil {
		return nil, err
	}
	pbftMsg.Signature = &msgSig
	return &pbftMsg, nil
}
