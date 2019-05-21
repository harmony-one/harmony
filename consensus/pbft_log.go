package consensus

import (
	"fmt"
	"sync"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
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
	MessageType   msg_pb.MessageType
	ConsensusID   uint32
	SeqNum        uint64
	BlockHash     common.Hash
	SenderPubkey  *bls.PublicKey
	LeaderPubkey  *bls.PublicKey
	Payload       []byte
	ViewchangeSig *bls.Sign
	M1AggSig      *bls.Sign
	M1Bitmap      *bls_cosi.Mask
	M2AggSig      *bls.Sign
	M2Bitmap      *bls_cosi.Mask
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
	pubKey, err := bls_cosi.BytesToBlsPublicKey(consensusMsg.SenderPubkey)
	if err != nil {
		return nil, err
	}
	pbftMsg.SenderPubkey = pubKey
	return &pbftMsg, nil
}

// ParseViewChangeMessage parses view change message into PbftMessage structure
func ParseViewChangeMessage(msg *msg_pb.Message) (*PbftMessage, error) {
	pbftMsg := PbftMessage{}
	pbftMsg.MessageType = msg.GetType()
	if pbftMsg.MessageType != msg_pb.MessageType_VIEWCHANGE {
		return nil, fmt.Errorf("ParseViewChangeMessage: incorrect message type %s", pbftMsg.MessageType)
	}

	vcMsg := msg.GetViewchange()
	pbftMsg.ConsensusID = vcMsg.ConsensusId
	pbftMsg.SeqNum = vcMsg.SeqNum
	pbftMsg.Payload = make([]byte, len(vcMsg.Payload))
	copy(pbftMsg.Payload[:], vcMsg.Payload[:])

	pubKey, err := bls_cosi.BytesToBlsPublicKey(vcMsg.SenderPubkey)
	if err != nil {
		utils.GetLogInstance().Warn("ParseViewChangeMessage failed to parse senderpubkey", "error", err)
		return nil, err
	}
	leaderKey, err := bls_cosi.BytesToBlsPublicKey(vcMsg.LeaderPubkey)
	if err != nil {
		utils.GetLogInstance().Warn("ParseViewChangeMessage failed to parse leaderpubkey", "error", err)
		return nil, err
	}

	vcSig := bls.Sign{}
	err = vcSig.Deserialize(vcMsg.ViewchangeSig)
	if err != nil {
		utils.GetLogInstance().Warn("ParseViewChangeMessage failed to deserialize the viewchange signature", "error", err)
		return nil, err
	}
	pbftMsg.SenderPubkey = pubKey
	pbftMsg.LeaderPubkey = leaderKey
	pbftMsg.ViewchangeSig = &vcSig

	return &pbftMsg, nil

}

// ParseNewViewMessage parses new view message into PbftMessage structure
func (consensus *Consensus) ParseNewViewMessage(msg *msg_pb.Message) (*PbftMessage, error) {
	pbftMsg := PbftMessage{}
	pbftMsg.MessageType = msg.GetType()

	if pbftMsg.MessageType != msg_pb.MessageType_NEWVIEW {
		return nil, fmt.Errorf("ParseNewViewMessage: incorrect message type %s", pbftMsg.MessageType)
	}

	vcMsg := msg.GetViewchange()
	pbftMsg.ConsensusID = vcMsg.ConsensusId
	pbftMsg.SeqNum = vcMsg.SeqNum
	pbftMsg.Payload = make([]byte, len(vcMsg.Payload))
	copy(pbftMsg.Payload[:], vcMsg.Payload[:])

	pubKey, err := bls_cosi.BytesToBlsPublicKey(vcMsg.SenderPubkey)
	if err != nil {
		utils.GetLogInstance().Warn("ParseViewChangeMessage failed to parse senderpubkey", "error", err)
		return nil, err
	}
	pbftMsg.SenderPubkey = pubKey

	if len(vcMsg.M1Aggsigs) > 0 {
		m1Sig := bls.Sign{}
		err = m1Sig.Deserialize(vcMsg.M1Aggsigs)
		if err != nil {
			utils.GetLogInstance().Warn("ParseViewChangeMessage failed to deserialize the multi signature for M1 aggregated signature", "error", err)
			return nil, err
		}
		m1mask, err := bls_cosi.NewMask(consensus.PublicKeys, nil)
		if err != nil {
			utils.GetLogInstance().Warn("ParseViewChangeMessage failed to create mask for multi signature", "error", err)
			return nil, err
		}
		m1mask.SetMask(vcMsg.M1Bitmap)
		pbftMsg.M1AggSig = &m1Sig
		pbftMsg.M1Bitmap = m1mask
	}

	if len(vcMsg.M2Aggsigs) > 0 {
		m2Sig := bls.Sign{}
		err = m2Sig.Deserialize(vcMsg.M2Aggsigs)
		if err != nil {
			utils.GetLogInstance().Warn("ParseViewChangeMessage failed to deserialize the multi signature for M2 aggregated signature", "error", err)
			return nil, err
		}
		m2mask, err := bls_cosi.NewMask(consensus.PublicKeys, nil)
		if err != nil {
			utils.GetLogInstance().Warn("ParseViewChangeMessage failed to create mask for multi signature", "error", err)
			return nil, err
		}
		m2mask.SetMask(vcMsg.M2Bitmap)
		pbftMsg.M2AggSig = &m2Sig
		pbftMsg.M2Bitmap = m2mask
	}

	return &pbftMsg, nil
}
