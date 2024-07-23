package consensus

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
)

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
	Verified           bool
}

func (m *FBFTMessage) Hash() []byte {
	// Hash returns hash of the struct

	c := crc32.NewIEEE()
	c.Write([]byte(strconv.FormatUint(uint64(m.MessageType), 10)))
	c.Write([]byte(strconv.FormatUint(m.ViewID, 10)))
	c.Write([]byte(strconv.FormatUint(m.BlockNum, 10)))
	c.Write(m.BlockHash[:])
	c.Write(m.Block[:])
	c.Write(m.Payload[:])
	return c.Sum(nil)
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

// HasSingleSender returns whether the message has only a single sender
func (m *FBFTMessage) HasSingleSender() bool {
	return len(m.SenderPubkeys) == 1
}

const (
	idTypeBytes   = 4
	idViewIDBytes = 8
	idHashBytes   = common.HashLength
	idSenderBytes = bls.PublicKeySizeInBytes

	idBytes = idTypeBytes + idViewIDBytes + idHashBytes + idSenderBytes
)

type (
	// fbftMsgID is the id that uniquely defines a fbft message.
	fbftMsgID [idBytes]byte
)

// id return the ID of the FBFT message which uniquely identifies a FBFT message.
// The ID is a concatenation of MsgType, BlockHash, and sender key
func (m *FBFTMessage) id() fbftMsgID {
	var id fbftMsgID
	binary.LittleEndian.PutUint32(id[:], uint32(m.MessageType))
	binary.LittleEndian.PutUint64(id[idTypeBytes:], m.ViewID)
	copy(id[idTypeBytes+idViewIDBytes:], m.BlockHash[:])

	if m.HasSingleSender() {
		copy(id[idTypeBytes+idViewIDBytes+idHashBytes:], m.SenderPubkeys[0].Bytes[:])
	} else {
		// Currently this case is not reachable as only validator will use id() func
		// and validator won't receive message with multiple senders
		copy(id[idTypeBytes+idViewIDBytes+idHashBytes:], m.SenderPubkeyBitmap[:])
	}
	return id
}

type FBFT interface {
	GetMessagesByTypeSeq(typ msg_pb.MessageType, blockNum uint64) []*FBFTMessage
	IsBlockVerified(hash common.Hash) bool
	DeleteBlockByNumber(number uint64)
	GetBlockByHash(hash common.Hash) *types.Block
	AddVerifiedMessage(msg *FBFTMessage)
	AddBlock(block *types.Block)
	GetMessagesByTypeSeqHash(typ msg_pb.MessageType, blockNum uint64, blockHash common.Hash) []*FBFTMessage
}

// FBFTLog represents the log stored by a node during FBFT process
type FBFTLog struct {
	blocks         map[common.Hash]*types.Block // store blocks received in FBFT
	verifiedBlocks map[common.Hash]struct{}     // store block hashes for blocks that has already been verified
	messages       map[fbftMsgID]*FBFTMessage   // store messages received in FBFT
}

// NewFBFTLog returns new instance of FBFTLog
func NewFBFTLog() *FBFTLog {
	pbftLog := FBFTLog{
		blocks:         make(map[common.Hash]*types.Block),
		messages:       make(map[fbftMsgID]*FBFTMessage),
		verifiedBlocks: make(map[common.Hash]struct{}),
	}
	return &pbftLog
}

// AddBlock add a new block into the log
func (log *FBFTLog) AddBlock(block *types.Block) {
	log.blocks[block.Hash()] = block
}

// MarkBlockVerified marks the block as verified
func (log *FBFTLog) MarkBlockVerified(block *types.Block) {
	log.verifiedBlocks[block.Hash()] = struct{}{}
}

// IsBlockVerified checks whether the block is verified
func (log *FBFTLog) IsBlockVerified(hash common.Hash) bool {
	_, exist := log.verifiedBlocks[hash]
	return exist
}

// GetBlockByHash returns the block matches the given block hash
func (log *FBFTLog) GetBlockByHash(hash common.Hash) *types.Block {
	return log.blocks[hash]
}

// GetBlocksByNumber returns the blocks match the given block number
func (log *FBFTLog) GetBlocksByNumber(number uint64) []*types.Block {
	var blocks []*types.Block
	for _, block := range log.blocks {
		if block.NumberU64() == number {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// DeleteBlocksLessThan deletes blocks less than given block number
func (log *FBFTLog) deleteBlocksLessThan(number uint64) {
	for h, block := range log.blocks {
		if block.NumberU64() < number {
			delete(log.blocks, h)
			delete(log.verifiedBlocks, h)
		}
	}
}

// DeleteBlockByNumber deletes block of specific number
func (log *FBFTLog) DeleteBlockByNumber(number uint64) {
	for h, block := range log.blocks {
		if block.NumberU64() == number {
			delete(log.blocks, h)
			delete(log.verifiedBlocks, h)
		}
	}
}

// DeleteMessagesLessThan deletes messages less than given block number
func (log *FBFTLog) deleteMessagesLessThan(number uint64) {
	for h, msg := range log.messages {
		if msg.BlockNum < number {
			delete(log.messages, h)
		}
	}
}

// AddVerifiedMessage adds a signature verified pbft message into the log
func (log *FBFTLog) AddVerifiedMessage(msg *FBFTMessage) {
	msg.Verified = true

	log.messages[msg.id()] = msg
}

// AddNotVerifiedMessage adds a not signature verified pbft message into the log
func (log *FBFTLog) AddNotVerifiedMessage(msg *FBFTMessage) {
	msg.Verified = false

	log.messages[msg.id()] = msg
}

// GetNotVerifiedCommittedMessages returns not verified committed pbft messages with matching blockNum, viewID and blockHash
func (log *FBFTLog) GetNotVerifiedCommittedMessages(blockNum uint64, viewID uint64, blockHash common.Hash) []*FBFTMessage {
	var found []*FBFTMessage
	for _, msg := range log.messages {
		if msg.MessageType == msg_pb.MessageType_COMMITTED && msg.BlockNum == blockNum && msg.ViewID == viewID && msg.BlockHash == blockHash && !msg.Verified {
			found = append(found, msg)
		}
	}
	return found
}

// GetMessagesByTypeSeqViewHash returns pbft messages with matching type, blockNum, viewID and blockHash
func (log *FBFTLog) GetMessagesByTypeSeqViewHash(typ msg_pb.MessageType, blockNum uint64, viewID uint64, blockHash common.Hash) []*FBFTMessage {
	var found []*FBFTMessage
	for _, msg := range log.messages {
		if msg.MessageType == typ && msg.BlockNum == blockNum && msg.ViewID == viewID && msg.BlockHash == blockHash && msg.Verified {
			found = append(found, msg)
		}
	}
	return found
}

// GetMessagesByTypeSeq returns pbft messages with matching type, blockNum
func (log *FBFTLog) GetMessagesByTypeSeq(typ msg_pb.MessageType, blockNum uint64) []*FBFTMessage {
	var found []*FBFTMessage
	for _, msg := range log.messages {
		if msg.MessageType == typ && msg.BlockNum == blockNum && msg.Verified {
			found = append(found, msg)
		}
	}
	return found
}

// GetMessagesByTypeSeqHash returns pbft messages with matching type, blockNum
func (log *FBFTLog) GetMessagesByTypeSeqHash(typ msg_pb.MessageType, blockNum uint64, blockHash common.Hash) []*FBFTMessage {
	var found []*FBFTMessage
	for _, msg := range log.messages {
		if msg.MessageType == typ && msg.BlockNum == blockNum && msg.BlockHash == blockHash && msg.Verified {
			found = append(found, msg)
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
	var found []*FBFTMessage
	for _, msg := range log.messages {
		if msg.MessageType != typ || msg.BlockNum != blockNum || msg.ViewID != viewID && msg.Verified {
			continue
		}
		found = append(found, msg)
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
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	return consensus.parseFBFTMessage(msg)
}

func (consensus *Consensus) parseFBFTMessage(msg *msg_pb.Message) (*FBFTMessage, error) {
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
		pubKeys, err := consensus.multiSigBitmap.GetSignedPubKeysFromBitmap(pbftMsg.SenderPubkeyBitmap)
		if err != nil {
			return nil, err
		}
		pbftMsg.SenderPubkeys = pubKeys
	}

	return &pbftMsg, nil
}

var errFBFTLogNotFound = errors.New("FBFT log not found")

// GetCommittedBlockAndMsgsFromNumber get committed block and message starting from block number bn.
func (log *FBFTLog) GetCommittedBlockAndMsgsFromNumber(bn uint64, logger *zerolog.Logger) (*types.Block, *FBFTMessage, error) {
	msgs := log.GetMessagesByTypeSeq(
		msg_pb.MessageType_COMMITTED, bn,
	)
	if len(msgs) == 0 {
		return nil, nil, errFBFTLogNotFound
	}
	if len(msgs) > 1 {
		logger.Error().Int("numMsgs", len(msgs)).Err(errors.New("DANGER!!! multiple COMMITTED message in PBFT log observed"))
	}
	for i := range msgs {
		block := log.GetBlockByHash(msgs[i].BlockHash)
		if block == nil {
			logger.Debug().
				Uint64("blockNum", msgs[i].BlockNum).
				Uint64("viewID", msgs[i].ViewID).
				Str("blockHash", msgs[i].BlockHash.Hex()).
				Err(errors.New("failed finding a matching block for committed message"))
			continue
		}
		return block, msgs[i], nil
	}
	return nil, nil, errFBFTLogNotFound
}

// PruneCacheBeforeBlock prune all blocks before bn
func (log *FBFTLog) PruneCacheBeforeBlock(bn uint64) {
	log.deleteBlocksLessThan(bn - 1)
	log.deleteMessagesLessThan(bn - 1)
}

type threadsafeFBFTLog struct {
	log *FBFTLog
	mu  *sync.RWMutex
}

func (t threadsafeFBFTLog) GetMessagesByTypeSeq(typ msg_pb.MessageType, blockNum uint64) []*FBFTMessage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.log.GetMessagesByTypeSeq(typ, blockNum)
}

func (t threadsafeFBFTLog) IsBlockVerified(hash common.Hash) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.log.IsBlockVerified(hash)
}

func (t threadsafeFBFTLog) DeleteBlockByNumber(number uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.log.DeleteBlockByNumber(number)
}

func (t threadsafeFBFTLog) GetBlockByHash(hash common.Hash) *types.Block {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.log.GetBlockByHash(hash)
}

func (t threadsafeFBFTLog) AddVerifiedMessage(msg *FBFTMessage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.log.AddVerifiedMessage(msg)
}

func (t threadsafeFBFTLog) AddBlock(block *types.Block) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.log.AddBlock(block)
}

func (t threadsafeFBFTLog) GetMessagesByTypeSeqHash(typ msg_pb.MessageType, blockNum uint64, blockHash common.Hash) []*FBFTMessage {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.log.GetMessagesByTypeSeqHash(typ, blockNum, blockHash)
}
