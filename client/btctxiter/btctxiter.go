// Uses btcd node.
// Use `GetBlockVerboseTx` to get block and tx at once.
// This way is faster

package btctxiter

import (
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcutil"
)

// BTCTXIterator is a btc transaction iterator.
type BTCTXIterator struct {
	blockIndex int64
	block      *btcjson.GetBlockVerboseResult
	txIndex    int
	tx         *btcjson.TxRawResult
	client     *rpcclient.Client
}

// Init is an init function of BTCTXIterator.
func (iter *BTCTXIterator) Init() {
	btcdHomeDir := btcutil.AppDataDir("btcd", false)
	certs, err := ioutil.ReadFile(filepath.Join(btcdHomeDir, "rpc.cert"))
	if err != nil {
		log.Fatal(err)
	}
	connCfg := &rpcclient.ConnConfig{
		Host:         "localhost:8334", // This goes to btcd
		Endpoint:     "ws",
		User:         "",
		Pass:         "",
		Certificates: certs,
	}
	iter.client, err = rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}
	iter.blockIndex = 0 // the genesis block cannot retrieved. Skip it intentionally.
	iter.block = nil
	iter.nextBlock()
	// defer iter.client.Shutdown()
}

// NextTx is to move to the next transaction.
func (iter *BTCTXIterator) NextTx() *btcjson.TxRawResult {
	iter.txIndex++
	if iter.txIndex >= len(iter.block.RawTx) {
		iter.nextBlock()
		iter.txIndex++
	}
	iter.tx = &iter.block.RawTx[iter.txIndex]
	// log.Println(iter.blockIndex, iter.txIndex, hashes[iter.txIndex])
	return iter.tx
}

// GetBlockIndex gets the index/height of the current block
func (iter *BTCTXIterator) GetBlockIndex() int64 {
	return iter.blockIndex
}

// GetBlock gets the current block
func (iter *BTCTXIterator) GetBlock() *btcjson.GetBlockVerboseResult {
	return iter.block
}

// GetTxIndex gets the index of the current transaction
func (iter *BTCTXIterator) GetTxIndex() int {
	return iter.txIndex
}

// GetTx gets the current transaction
func (iter *BTCTXIterator) GetTx() *btcjson.TxRawResult {
	return iter.tx
}

func (iter *BTCTXIterator) resetTx() {
	iter.txIndex = -1
	iter.tx = nil
}

// Move to the next block
func (iter *BTCTXIterator) nextBlock() *btcjson.GetBlockVerboseResult {
	iter.blockIndex++
	hash, err := iter.client.GetBlockHash(iter.blockIndex)
	if err != nil {
		log.Panic("Failed to get block hash at", iter.blockIndex, err)
	}
	iter.block, err = iter.client.GetBlockVerboseTx(hash)
	if err != nil {
		log.Panic("Failed to get block", iter.blockIndex, err)
	}
	iter.resetTx()

	return iter.block
}

// IsCoinBaseTx returns true if tx is a coinbase tx.
func IsCoinBaseTx(tx *btcjson.TxRawResult) bool {
	// A coin base must only have one transaction input.
	if len(tx.Vin) != 1 {
		return false
	}

	return tx.Vin[0].IsCoinBase()
}
