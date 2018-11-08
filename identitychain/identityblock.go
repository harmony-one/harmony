package identitychain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"time"

	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/utils"
)

// IdentityBlock has the information of one node
type IdentityBlock struct {
	Timestamp     int64
	PrevBlockHash [32]byte
	NumIdentities int32
	Identities    []*node.Node
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

// GetIdentities returns a list of identities.
func (b *IdentityBlock) GetIdentities() []*node.Node {
	return b.Identities
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
func NewBlock(Identities []*node.Node, prevBlockHash [32]byte) *IdentityBlock {
	block := &IdentityBlock{Timestamp: time.Now().Unix(), PrevBlockHash: prevBlockHash, NumIdentities: int32(len(Identities)), Identities: Identities}
	return block
}

// CalculateBlockHash returns a hash of the block
func (b *IdentityBlock) CalculateBlockHash() [32]byte {
	var hashes [][]byte
	var blockHash [32]byte
	hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(b.Timestamp))
	hashes = append(hashes, b.PrevBlockHash[:])
	for _, id := range b.Identities {
		hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(id))
	}
	hashes = append(hashes, utils.ConvertFixedDataIntoByteArray(b.NumIdentities))
	blockHash = sha256.Sum256(bytes.Join(hashes, []byte{}))
	return blockHash
}

// NewGenesisBlock creates and returns genesis Block.
func NewGenesisBlock() *IdentityBlock {
	var Ids []*node.Node
	block := &IdentityBlock{Timestamp: time.Now().Unix(), PrevBlockHash: [32]byte{}, NumIdentities: 1, Identities: Ids}
	return block
}
