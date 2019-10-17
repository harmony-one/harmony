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

// FBFTLog represents the log stored by a node during FBFT process
type FBFTLog struct {
	blocks     mapset.Set //store blocks received in FBFT
	messages   mapset.Set // store messages received in FBFT
	maxLogSize uint32
	mutex      sync.Mutex
}

// FBFTMessage is the record of pbft messages received by a node during FBFT process
type FBFTMessage struct {
	MessageType   msg_pb.MessageType
	ViewID        uint64
	BlockNum      uint64
	BlockHash     common.Hash
	Block         []byte
	SenderPubkey  *bls.PublicKey
	LeaderPubkey  *bls.PublicKey
	Payload       []byte
	ViewchangeSig *bls.Sign
	ViewidSig     *bls.Sign
	M2AggSig      *bls.Sign
	M2Bitmap      *bls_cosi.Mask
	M3AggSig      *bls.Sign
	M3Bitmap      *bls_cosi.Mask
}

// NewFBFTLog returns new instance of FBFTLog
func NewFBFTLog() *FBFTLog {
	blocks := mapset.NewSet()
	messages := mapset.NewSet()
	logSize := maxLogSize
	pbftLog := FBFTLog{blocks: blocks, messages: messages, maxLogSize: logSize}
	return &pbftLog
}

// Blocks return the blocks stored in the log
func (log *FBFTLog) Blocks() mapset.Set {
	return log.blocks
}

// Messages return the messages stored in the log
func (log *FBFTLog) Messages() mapset.Set {
	return log.messages
}

// AddBlock add a new block into the log
func (log *FBFTLog) AddBlock(block *types.Block) {
	log.blocks.Add(block)
}

// GetBlockByHash returns the block matches the given block hash
func (log *FBFTLog) GetBlockByHash(hash common.Hash) *types.Block {
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
func (log *FBFTLog) GetBlocksByNumber(number uint64) []*types.Block {
	found := []*types.Block{}
	it := log.Blocks().Iterator()
	for block := range it.C {
		if block.(*types.Block).NumberU64() == number {
			found = append(found, block.(*types.Block))
		}
	}
	return found
}

// DeleteBlocksLessThan deletes blocks less than given block number
func (log *FBFTLog) DeleteBlocksLessThan(number uint64) {
	found := mapset.NewSet()
	it := log.Blocks().Iterator()
	for block := range it.C {
		if block.(*types.Block).NumberU64() < number {
			found.Add(block)
		}
	}
	log.blocks = log.blocks.Difference(found)
}

// DeleteBlockByNumber deletes block of specific number
func (log *FBFTLog) DeleteBlockByNumber(number uint64) {
	found := mapset.NewSet()
	it := log.Blocks().Iterator()
	for block := range it.C {
		if block.(*types.Block).NumberU64() == number {
			found.Add(block)
		}
	}
	log.blocks = log.blocks.Difference(found)
}

// DeleteMessagesLessThan deletes messages less than given block number
func (log *FBFTLog) DeleteMessagesLessThan(number uint64) {
	found := mapset.NewSet()
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*FBFTMessage).BlockNum < number {
			found.Add(msg)
		}
	}
	log.messages = log.messages.Difference(found)
}

// AddMessage adds a pbft message into the log
func (log *FBFTLog) AddMessage(msg *FBFTMessage) {
	log.messages.Add(msg)
}

// GetMessagesByTypeSeqViewHash returns pbft messages with matching type, blockNum, viewID and blockHash
func (log *FBFTLog) GetMessagesByTypeSeqViewHash(typ msg_pb.MessageType, blockNum uint64, viewID uint64, blockHash common.Hash) []*FBFTMessage {
	found := []*FBFTMessage{}
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*FBFTMessage).MessageType == typ && msg.(*FBFTMessage).BlockNum == blockNum && msg.(*FBFTMessage).ViewID == viewID && msg.(*FBFTMessage).BlockHash == blockHash {
			found = append(found, msg.(*FBFTMessage))
		}
	}
	return found
}

// GetMessagesByTypeSeq returns pbft messages with matching type, blockNum
func (log *FBFTLog) GetMessagesByTypeSeq(typ msg_pb.MessageType, blockNum uint64) []*FBFTMessage {
	found := []*FBFTMessage{}
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*FBFTMessage).MessageType == typ && msg.(*FBFTMessage).BlockNum == blockNum {
			found = append(found, msg.(*FBFTMessage))
		}
	}
	return found
}

// GetMessagesByTypeSeqHash returns pbft messages with matching type, blockNum
func (log *FBFTLog) GetMessagesByTypeSeqHash(typ msg_pb.MessageType, blockNum uint64, blockHash common.Hash) []*FBFTMessage {
	found := []*FBFTMessage{}
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*FBFTMessage).MessageType == typ && msg.(*FBFTMessage).BlockNum == blockNum && msg.(*FBFTMessage).BlockHash == blockHash {
			found = append(found, msg.(*FBFTMessage))
		}
	}
	return found
}

// HasMatchingAnnounce returns whether the log contains announce type message with given blockNum, blockHash
func (log *FBFTLog) HasMatchingAnnounce(blockNum uint64, blockHash common.Hash) bool {
	found := log.GetMessagesByTypeSeqHash(msg_pb.MessageType_ANNOUNCE, blockNum, blockHash)
	return len(found) >= 1
}

// HasMatchingViewAnnounce returns whether the log contains announce type message with given blockNum, viewID and blockHash
func (log *FBFTLog) HasMatchingViewAnnounce(blockNum uint64, viewID uint64, blockHash common.Hash) bool {
	found := log.GetMessagesByTypeSeqViewHash(msg_pb.MessageType_ANNOUNCE, blockNum, viewID, blockHash)
	return len(found) >= 1
}

// HasMatchingPrepared returns whether the log contains prepared message with given blockNum, viewID and blockHash
func (log *FBFTLog) HasMatchingPrepared(blockNum uint64, blockHash common.Hash) bool {
	found := log.GetMessagesByTypeSeqHash(msg_pb.MessageType_PREPARED, blockNum, blockHash)
	return len(found) >= 1
}

// HasMatchingViewPrepared returns whether the log contains prepared message with given blockNum, viewID and blockHash
func (log *FBFTLog) HasMatchingViewPrepared(blockNum uint64, viewID uint64, blockHash common.Hash) bool {
	found := log.GetMessagesByTypeSeqViewHash(msg_pb.MessageType_PREPARED, blockNum, viewID, blockHash)
	return len(found) >= 1
}

// GetMessagesByTypeSeqView returns pbft messages with matching type, blockNum and viewID
func (log *FBFTLog) GetMessagesByTypeSeqView(typ msg_pb.MessageType, blockNum uint64, viewID uint64) []*FBFTMessage {
	found := []*FBFTMessage{}
	it := log.Messages().Iterator()
	for msg := range it.C {
		if msg.(*FBFTMessage).MessageType != typ || msg.(*FBFTMessage).BlockNum != blockNum || msg.(*FBFTMessage).ViewID != viewID {
			continue
		}
		found = append(found, msg.(*FBFTMessage))
	}
	return found
}

// FindMessageByMaxViewID returns the message that has maximum ViewID
func (log *FBFTLog) FindMessageByMaxViewID(msgs []*FBFTMessage) *FBFTMessage {
	if len(msgs) == 0 {
		return nil
	}
	maxIdx := -1
	maxViewID := uint64(0)
	for k, v := range msgs {
		if v.ViewID >= maxViewID {
			maxIdx = k
			maxViewID = v.ViewID
		}
	}
	return msgs[maxIdx]
}

// ParseFBFTMessage parses FBFT message into FBFTMessage structure
func ParseFBFTMessage(msg *msg_pb.Message) (*FBFTMessage, error) {
	pbftMsg := FBFTMessage{}
	pbftMsg.MessageType = msg.GetType()
	consensusMsg := msg.GetConsensus()

	pbftMsg.ViewID = consensusMsg.ViewId
	pbftMsg.BlockNum = consensusMsg.BlockNum
	pbftMsg.BlockHash = common.Hash{}
	copy(pbftMsg.BlockHash[:], consensusMsg.BlockHash[:])
	pbftMsg.Payload = make([]byte, len(consensusMsg.Payload))
	copy(pbftMsg.Payload[:], consensusMsg.Payload[:])
	pbftMsg.Block = make([]byte, len(consensusMsg.Block))
	copy(pbftMsg.Block[:], consensusMsg.Block[:])

	pubKey, err := bls_cosi.BytesToBlsPublicKey(consensusMsg.SenderPubkey)
	if err != nil {
		return nil, err
	}
	pbftMsg.SenderPubkey = pubKey
	return &pbftMsg, nil
}

// ParseViewChangeMessage parses view change message into FBFTMessage structure
func ParseViewChangeMessage(msg *msg_pb.Message) (*FBFTMessage, error) {
	pbftMsg := FBFTMessage{}
	pbftMsg.MessageType = msg.GetType()
	if pbftMsg.MessageType != msg_pb.MessageType_VIEWCHANGE {
		return nil, fmt.Errorf("ParseViewChangeMessage: incorrect message type %s", pbftMsg.MessageType)
	}

	vcMsg := msg.GetViewchange()
	pbftMsg.ViewID = vcMsg.ViewId
	pbftMsg.BlockNum = vcMsg.BlockNum
	pbftMsg.Payload = make([]byte, len(vcMsg.Payload))
	copy(pbftMsg.Payload[:], vcMsg.Payload[:])

	pubKey, err := bls_cosi.BytesToBlsPublicKey(vcMsg.SenderPubkey)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("ParseViewChangeMessage failed to parse senderpubkey")
		return nil, err
	}
	leaderKey, err := bls_cosi.BytesToBlsPublicKey(vcMsg.LeaderPubkey)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("ParseViewChangeMessage failed to parse leaderpubkey")
		return nil, err
	}

	vcSig := bls.Sign{}
	err = vcSig.Deserialize(vcMsg.ViewchangeSig)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("ParseViewChangeMessage failed to deserialize the viewchange signature")
		return nil, err
	}

	vcSig1 := bls.Sign{}
	err = vcSig1.Deserialize(vcMsg.ViewidSig)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("ParseViewChangeMessage failed to deserialize the viewid signature")
		return nil, err
	}
	pbftMsg.SenderPubkey = pubKey
	pbftMsg.LeaderPubkey = leaderKey
	pbftMsg.ViewchangeSig = &vcSig
	pbftMsg.ViewidSig = &vcSig1
	return &pbftMsg, nil
}

// ParseNewViewMessage parses new view message into FBFTMessage structure
func (consensus *Consensus) ParseNewViewMessage(msg *msg_pb.Message) (*FBFTMessage, error) {
	FBFTMsg := FBFTMessage{}
	FBFTMsg.MessageType = msg.GetType()

	if FBFTMsg.MessageType != msg_pb.MessageType_NEWVIEW {
		return nil, fmt.Errorf("ParseNewViewMessage: incorrect message type %s", FBFTMsg.MessageType)
	}

	vcMsg := msg.GetViewchange()
	FBFTMsg.ViewID = vcMsg.ViewId
	FBFTMsg.BlockNum = vcMsg.BlockNum
	FBFTMsg.Payload = make([]byte, len(vcMsg.Payload))
	copy(FBFTMsg.Payload[:], vcMsg.Payload[:])

	pubKey, err := bls_cosi.BytesToBlsPublicKey(vcMsg.SenderPubkey)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("ParseViewChangeMessage failed to parse senderpubkey")
		return nil, err
	}
	FBFTMsg.SenderPubkey = pubKey

	if len(vcMsg.M3Aggsigs) > 0 {
		m3Sig := bls.Sign{}
		err = m3Sig.Deserialize(vcMsg.M3Aggsigs)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("ParseViewChangeMessage failed to deserialize the multi signature for M3 viewID signature")
			return nil, err
		}
		m3mask, err := bls_cosi.NewMask(consensus.Decider.Participants(), nil)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("ParseViewChangeMessage failed to create mask for multi signature")
			return nil, err
		}
		m3mask.SetMask(vcMsg.M3Bitmap)
		FBFTMsg.M3AggSig = &m3Sig
		FBFTMsg.M3Bitmap = m3mask
	}

	if len(vcMsg.M2Aggsigs) > 0 {
		m2Sig := bls.Sign{}
		err = m2Sig.Deserialize(vcMsg.M2Aggsigs)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("ParseViewChangeMessage failed to deserialize the multi signature for M2 aggregated signature")
			return nil, err
		}
		m2mask, err := bls_cosi.NewMask(consensus.Decider.Participants(), nil)
		if err != nil {
			utils.Logger().Warn().Err(err).Msg("ParseViewChangeMessage failed to create mask for multi signature")
			return nil, err
		}
		m2mask.SetMask(vcMsg.M2Bitmap)
		FBFTMsg.M2AggSig = &m2Sig
		FBFTMsg.M2Bitmap = m2mask
	}

	return &FBFTMsg, nil
}
