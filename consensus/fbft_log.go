package consensus

import (
	"fmt"

	"github.com/harmony-one/harmony/crypto/bls"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
)

// FBFTLog represents the log stored by a node during FBFT process
type FBFTLog struct {
	blocks     mapset.Set //store blocks received in FBFT
	messages   mapset.Set // store messages received in FBFT
	maxLogSize uint32
}

// FBFTMessage is the record of pbft messages received by a node during FBFT process
type FBFTMessage struct {
	MessageType        msg_pb.MessageType
	ViewID             uint64
	BlockNum           uint64
	BlockHash          common.Hash
	Block              []byte
	SenderPubkeys      []*bls.PublicKeyWrapper
	SenderPubkeyBitmap []byte
	LeaderPubkey       *bls.PublicKeyWrapper
	Payload            []byte
	ViewchangeSig      *bls_core.Sign
	ViewidSig          *bls_core.Sign
	M2AggSig           *bls_core.Sign
	M2Bitmap           *bls_cosi.Mask
	M3AggSig           *bls_core.Sign
	M3Bitmap           *bls_cosi.Mask
}

// String ..
func (m *FBFTMessage) String() string {
	sender := ""
	for _, key := range m.SenderPubkeys {
		if sender == "" {
			sender = key.Bytes.Hex()
		} else {
			sender = sender + ";" + key.Bytes.Hex()
		}
	}
	leader := ""
	if m.LeaderPubkey != nil {
		leader = m.LeaderPubkey.Bytes.Hex()
	}
	return fmt.Sprintf(
		"[Type:%s ViewID:%d Num:%d BlockHash:%s Sender:%s Leader:%s]",
		m.MessageType.String(),
		m.ViewID,
		m.BlockNum,
		m.BlockHash.Hex(),
		sender,
		leader,
	)
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
		if msg.(*FBFTMessage).MessageType == typ &&
			msg.(*FBFTMessage).BlockNum == blockNum &&
			msg.(*FBFTMessage).ViewID == viewID &&
			msg.(*FBFTMessage).BlockHash == blockHash {
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
		if msg.(*FBFTMessage).MessageType == typ &&
			msg.(*FBFTMessage).BlockNum == blockNum {
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
		if msg.(*FBFTMessage).MessageType == typ &&
			msg.(*FBFTMessage).BlockNum == blockNum &&
			msg.(*FBFTMessage).BlockHash == blockHash {
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
func (consensus *Consensus) ParseFBFTMessage(msg *msg_pb.Message) (*FBFTMessage, error) {
	// TODO Have this do sanity checks on the message please
	pbftMsg := FBFTMessage{}
	pbftMsg.MessageType = msg.GetType()
	consensusMsg := msg.GetConsensus()
	pbftMsg.ViewID = consensusMsg.ViewId
	pbftMsg.BlockNum = consensusMsg.BlockNum
	copy(pbftMsg.BlockHash[:], consensusMsg.BlockHash[:])
	pbftMsg.Payload = make([]byte, len(consensusMsg.Payload))
	copy(pbftMsg.Payload[:], consensusMsg.Payload[:])
	pbftMsg.Block = make([]byte, len(consensusMsg.Block))
	copy(pbftMsg.Block[:], consensusMsg.Block[:])
	pbftMsg.SenderPubkeyBitmap = make([]byte, len(consensusMsg.SenderPubkeyBitmap))
	copy(pbftMsg.SenderPubkeyBitmap[:], consensusMsg.SenderPubkeyBitmap[:])

	if len(consensusMsg.SenderPubkey) != 0 {
		// If SenderPubKey is populated, treat it as a single key message
		pubKey, err := bls_cosi.BytesToBLSPublicKey(consensusMsg.SenderPubkey)
		if err != nil {
			return nil, err
		}
		pbftMsg.SenderPubkeys = []*bls.PublicKeyWrapper{{Object: pubKey}}
		copy(pbftMsg.SenderPubkeys[0].Bytes[:], consensusMsg.SenderPubkey[:])
	} else {
		// else, it should be a multi-key message where the bitmap is populated
		consensus.multiSigMutex.RLock()
		pubKeys, err := consensus.multiSigBitmap.GetSignedPubKeysFromBitmap(pbftMsg.SenderPubkeyBitmap)
		consensus.multiSigMutex.RUnlock()
		if err != nil {
			return nil, err
		}
		pbftMsg.SenderPubkeys = pubKeys
	}

	return &pbftMsg, nil
}
