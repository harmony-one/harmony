package consensus

import (
	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
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
	msg       []byte
	signature *bls.Sign
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

// GetBlockByNumber returns teh block matches the given block number
func (log *PbftLog) GetBlockByNumber(id uint64) *types.Block {
	var found *types.Block
	it := log.Blocks().Iterator()
	for block := range it.C {
		if block.(*types.Block).NumberU64() == id {
			found = block.(*types.Block)
			it.Stop()
		}
	}
	return found
}

// AddMessage adds a pbft message into the log
func (log *PbftLog) AddMessage(msg *PbftMessage) {
	log.messages.Add(msg)
}
