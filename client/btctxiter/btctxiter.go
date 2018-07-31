package btctxiter

import (
	"log"

	"github.com/piotrnar/gocoin/lib/btc"
	"github.com/piotrnar/gocoin/lib/others/blockdb"
)

type BTCTXIterator struct {
	index         int
	blockDatabase *blockdb.BlockDB
}

func (iter *BTCTXIterator) Init() {
	// Set real Bitcoin network
	Magic := [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

	// Specify blocks directory
	iter.blockDatabase = blockdb.NewBlockDB("/Users/ricl/Library/Application Support/Bitcoin/blocks", Magic)
	iter.index = -1
}

func (iter *BTCTXIterator) IterateBTCTX() *btc.Block {
	iter.index++
	dat, err := iter.blockDatabase.FetchNextBlock()
	if dat == nil || err != nil {
		log.Println("END of DB file")
	}
	blk, err := btc.NewBlock(dat[:])

	if err != nil {
		println("Block inconsistent:", err.Error())
	}

	blk.BuildTxList()
	return blk
}
