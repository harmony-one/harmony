package btctxiter

import (
	"log"

	"github.com/btcsuite/btcutil"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
)

type BTCTXIterator struct {
	blockIndex int64
	block      *wire.MsgBlock
	txIndex    int
	tx         *btcutil.Tx
	client     *rpcclient.Client
}

func (iter *BTCTXIterator) Init() {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         "localhost:8332",
		User:         "ricl",
		Pass:         "123",
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	var err error
	iter.client, err = rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}
	iter.blockIndex = 0 // the genesis block cannot retrieved. Skip it intentionally.
	iter.block = nil
	iter.nextBlock()
	// defer iter.client.Shutdown()
}

// Move to the next transaction
func (iter *BTCTXIterator) NextTx() *btcutil.Tx {
	iter.txIndex++
	hashes, err := iter.block.TxHashes()
	if err != nil {
		log.Println("Failed to get tx hashes", iter.blockIndex, iter.txIndex, err)
		return nil
	}
	if iter.txIndex >= len(hashes) {
		iter.nextBlock()
		iter.txIndex++
	}
	iter.tx, err = iter.client.GetRawTransaction(&hashes[iter.txIndex])
	if err != nil {
		log.Println("Failed to get raw tx", iter.blockIndex, iter.txIndex, hashes[iter.txIndex], err)
		return nil
	}
	log.Println("get raw tx", iter.blockIndex, iter.txIndex, hashes[iter.txIndex], iter.tx)
	return iter.tx
}

// Gets the index/height of the current block
func (iter *BTCTXIterator) GetBlockIndex() int64 {
	return iter.blockIndex
}

// Gets the current block
func (iter *BTCTXIterator) GetBlock() *wire.MsgBlock {
	return iter.block
}

// Gets the index of the current transaction
func (iter *BTCTXIterator) GetTxIndex() int {
	return iter.txIndex
}

// Gets the current transaction
func (iter *BTCTXIterator) GetTx() *btcutil.Tx {
	return iter.tx
}

func (iter *BTCTXIterator) resetTx() {
	iter.txIndex = -1
	iter.tx = nil
}

// Move to the next block
func (iter *BTCTXIterator) nextBlock() *wire.MsgBlock {
	iter.blockIndex++
	hash, err := iter.client.GetBlockHash(iter.blockIndex)
	if err != nil {
		log.Println("Failed to get block hash at", iter.blockIndex)
	}
	iter.block, err = iter.client.GetBlock(hash)
	if err != nil {
		log.Println("Failed to get block", iter.blockIndex, iter.block)
	}
	iter.resetTx()

	return iter.block
}
