package btctxiter

import (
	"log"

	"github.com/piotrnar/gocoin/lib/btc"
	"github.com/piotrnar/gocoin/lib/others/blockdb"
)

type BTCTXIterator struct {
	blockIndex    int
	block         *btc.Block
	txIndex       int
	tx            *btc.Tx
	blockDatabase *blockdb.BlockDB
}

func (iter *BTCTXIterator) Init() {
	// Set real Bitcoin network
	Magic := [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

	// Specify blocks directory
	iter.blockDatabase = blockdb.NewBlockDB("/Users/ricl/Library/Application Support/Bitcoin/blocks", Magic)
	iter.blockIndex = -1
	iter.block = nil
	iter.nextBlock()
}

// Move to the next transaction
func (iter *BTCTXIterator) NextTx() *btc.Tx {
	iter.txIndex++
	if iter.txIndex >= iter.block.TxCount {
		iter.nextBlock()
		iter.txIndex++
	}
	iter.tx = iter.block.Txs[iter.txIndex]
	return iter.tx
}

// Gets the index/height of the current block
func (iter *BTCTXIterator) GetBlockIndex() int {
	return iter.blockIndex
}

// Gets the current block
func (iter *BTCTXIterator) GetBlock() *btc.Block {
	return iter.block
}

// Gets the index of the current transaction
func (iter *BTCTXIterator) GetTxIndex() int {
	return iter.txIndex
}

// Gets the current transaction
func (iter *BTCTXIterator) GetTx() *btc.Tx {
	return iter.tx
}

func (iter *BTCTXIterator) resetTx() {
	iter.txIndex = -1
	iter.tx = nil
}

// Move to the next block
func (iter *BTCTXIterator) nextBlock() *btc.Block {
	iter.blockIndex++
	dat, err := iter.blockDatabase.FetchNextBlock()
	if dat == nil || err != nil {
		log.Println("END of DB file")
	}
	iter.block, err = btc.NewBlock(dat[:])

	if err != nil {
		println("Block inconsistent:", err.Error())
	}

	iter.block.BuildTxList()

	iter.resetTx()

	return iter.block
}
