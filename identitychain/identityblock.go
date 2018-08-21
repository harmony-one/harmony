package identitychain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"time"

	"github.com/simple-rules/harmony-benchmark/utils"
	"github.com/simple-rules/harmony-benchmark/waitnode"
)

// IdentityBlock has the information of one node
type IdentityBlock struct {
	Timestamp     int64
	PrevBlockHash [32]byte
	NumIdentities int32
	Identities    []*waitnode.WaitNode
}

// Serialize serializes the block
func (b *IdentityBlock) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}

// DeserializeBlock deserializes a block
func DeserializeBlock(d []byte) *IdentityBlock {
	var block IdentityBlock
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}
	return &block
}

// NewBlock creates and returns a new block.
func NewBlock(Identities []*waitnode.WaitNode, prevBlockHash [32]byte) *IdentityBlock {
	block := &IdentityBlock{Timestamp: time.Now().Unix(), PrevBlockHash: prevBlockHash, NumIdentities: int32(len(Identities)), Identities: Identities}
	return &block
}

// CalculateBlockHash returns a hash of the block
func (b *IdentityBlock) CalculateBlockHash() []byte {
	var hashes [][]byte
	var blockHash [32]byte
	hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(b.Timestamp))
	hashes = append(hashes, b.PrevBlockHash[:])
	for _, id := range b.Identities {
		hashes = append(hashes, id)
	}
	hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(b.ShardId))
	blockHash = sha256.Sum256(bytes.Join(hashes, []byte{}))
	return blockHash[:]
}

// NewGenesisBlock creates and returns genesis Block.
func NewGenesisBlock() *IdentityBlock {
	numTxs := 0
	var Ids []*waitnode.WaitNode
	block := &IdentityBlock{Timestamp: time.Now().Unix(), PrevBlockHash: [32]byte{}, NumIdentities: 1, Identities: Ids}
	return block
}
