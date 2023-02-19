package explorer

import (
	"encoding/binary"
	"fmt"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	goversion "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
	"go.uber.org/zap/buffer"
)

const (
	LegAddressPrefix = "ad_"
	CheckpointBitmap = "checkpoint_bitmap"
	TracePrefix      = "tr_"

	oneAddrByteLen = 42 // byte size of string "one1..."
)

// readCheckpointBitmap read explorer checkpoint bitmap from storage
func readCheckpointBitmap(db databaseReader) (*roaring64.Bitmap, error) {
	bitmapByte, err := db.Get([]byte(CheckpointBitmap))
	if err != nil {
		if err == leveldb.ErrNotFound {
			return roaring64.NewBitmap(), nil
		}
		return nil, err
	}

	rb := roaring64.NewBitmap()
	err = rb.UnmarshalBinary(bitmapByte)
	if err != nil {
		return nil, err
	}

	return rb, nil
}

// writeCheckpointBitmap write explorer checkpoint bitmap to storage
func writeCheckpointBitmap(db databaseWriter, rb Bitmap) error {
	bitmapByte, err := rb.MarshalBinary()
	if err != nil {
		return err
	}

	return db.Put([]byte(CheckpointBitmap), bitmapByte)
}

func getTraceResultKey(key []byte) []byte {
	return append([]byte(TracePrefix), key...)
}
func isTraceResultInDB(db databaseReader, key []byte) (bool, error) {
	key = getTraceResultKey(key)
	return db.Has(key)
}

func writeTraceResult(db databaseWriter, key []byte, data []byte) error {
	key = getTraceResultKey(key)
	return db.Put(key, data)
}

func getTraceResult(db databaseReader, key []byte) ([]byte, error) {
	key = getTraceResultKey(key)
	return db.Get(key)
}

// New schema

var (
	versionKey     = []byte("version")
	versionV100, _ = goversion.NewVersion("1.0.0")
)

// isVersionV100 return whether the version is larger than or equal to 1.0.0
func isVersionV100(db databaseReader) (bool, error) {
	curVer, err := readVersion(db)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return curVer.GreaterThanOrEqual(versionV100), nil
}

func readVersion(db databaseReader) (*goversion.Version, error) {
	key := versionKey
	val, err := db.Get(key)
	if err != nil {
		return nil, err
	}
	return goversion.NewVersion(string(val))
}

func writeVersion(db databaseWriter, ver *goversion.Version) error {
	key := versionKey
	val := []byte(ver.String())
	return db.Put(key, val)
}

var (
	addrPrefix                = []byte("addr")
	txnPrefix                 = []byte("tx")
	addrNormalTxnIndexPrefix  = []byte("at")
	addrStakingTxnIndexPrefix = []byte("stk")
)

// bPool is the sync pool for reusing the memory for allocating db keys
var bPool = buffer.NewPool()

func getAddressKey(addr oneAddress) []byte {
	b := bPool.Get()
	defer b.Free()

	_, _ = b.Write(addrPrefix)
	_, _ = b.Write([]byte(addr))
	return b.Bytes()
}

func getAddressFromAddressKey(key []byte) (oneAddress, error) {
	if len(key) < len(addrPrefix)+oneAddrByteLen {
		return "", errors.New("address key size unexpected")
	}
	addrBytes := key[len(addrPrefix) : len(addrPrefix)+oneAddrByteLen]
	return oneAddress(addrBytes), nil
}

func writeAddressEntry(db databaseWriter, addr oneAddress) error {
	key := getAddressKey(addr)
	return db.Put(key, []byte{})
}

func isAddressWritten(db databaseReader, addr oneAddress) (bool, error) {
	key := getAddressKey(addr)
	return db.Has(key)
}

func getAllAddresses(db databaseReader) ([]oneAddress, error) {
	var addrs []oneAddress
	it := db.NewPrefixIterator(addrPrefix)
	defer it.Release()
	for it.Next() {
		addr, err := getAddressFromAddressKey(it.Key())
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

func getAddressesInRange(db databaseReader, start oneAddress, size int) ([]oneAddress, error) {
	var addrs []oneAddress
	startKey := getAddressKey(start)
	it := db.NewSizedIterator(startKey, size)
	defer it.Release()

	for it.Next() {
		addr, err := getAddressFromAddressKey(it.Key())
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, it.Error()
}

// getTxnKey return the transaction key. It's simply a prefix with the transaction hash
func getTxnKey(txHash common.Hash) []byte {
	b := bPool.Get()
	defer b.Free()

	_, _ = b.Write(txnPrefix)
	_, _ = b.Write(txHash[:])
	return b.Bytes()
}

func readTxnByHash(db databaseReader, txHash common.Hash) (*TxRecord, error) {
	key := getTxnKey(txHash)
	b, err := db.Get(key)
	if err != nil {
		return nil, err
	}
	var tx *TxRecord
	if err := rlp.DecodeBytes(b, &tx); err != nil {
		return nil, err
	}
	return tx, nil
}

func writeTxn(db databaseWriter, txHash common.Hash, tx *TxRecord) error {
	key := getTxnKey(txHash)
	bs, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}
	return db.Put(key, bs)
}

// normalTxnIndex is a single entry of address-transaction index for normal transaction.
// The key of the entry in db is a combination of txnPrefix, account address, block
// number of the transaction, transaction index in the block, and transaction hash
type normalTxnIndex struct {
	addr        oneAddress
	blockNumber uint64
	txnIndex    uint64
	txnHash     common.Hash
}

func (index normalTxnIndex) key() []byte {
	b := bPool.Get()
	defer b.Free()

	_, _ = b.Write(addrNormalTxnIndexPrefix)
	_, _ = b.Write([]byte(index.addr))
	_ = binary.Write(b, binary.BigEndian, index.blockNumber)
	_ = binary.Write(b, binary.BigEndian, index.txnIndex)
	_, _ = b.Write(index.txnHash[:])
	return b.Bytes()
}

func normalTxnIndexPrefixByAddr(addr oneAddress) []byte {
	b := bPool.Get()
	defer b.Free()

	_, _ = b.Write(addrNormalTxnIndexPrefix)
	_, _ = b.Write([]byte(addr))
	return b.Bytes()
}

func txnHashFromNormalTxnIndexKey(key []byte) (common.Hash, error) {
	txStart := len(addrNormalTxnIndexPrefix) + oneAddrByteLen + 8 + 8
	expSize := txStart + common.HashLength
	if len(key) < expSize {
		return common.Hash{}, errors.New("unexpected key size")
	}
	var txHash common.Hash
	copy(txHash[:], key[txStart:expSize])
	return txHash, nil
}

func getNormalTxnHashesByAccount(db databaseReader, addr oneAddress) ([]common.Hash, []TxType, error) {
	var (
		txHashes []common.Hash
		tts      []TxType
	)
	prefix := normalTxnIndexPrefixByAddr(addr)
	err := forEachAtPrefix(db, prefix, func(key, val []byte) error {
		txHash, err := txnHashFromNormalTxnIndexKey(key)
		if err != nil {
			return err
		}
		if len(val) < 0 {
			return errors.New("val size not expected")
		}
		tts = append(tts, TxType(val[0]))
		txHashes = append(txHashes, txHash)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return txHashes, tts, nil
}

func writeNormalTxnIndex(db databaseWriter, entry normalTxnIndex, tt TxType) error {
	key := entry.key()
	return db.Put(key, []byte{byte(tt)})
}

type stakingTxnIndex struct {
	addr        oneAddress
	blockNumber uint64
	txnIndex    uint64
	txnHash     common.Hash
}

func (index stakingTxnIndex) key() []byte {
	b := bPool.Get()
	defer b.Free()

	_, _ = b.Write(addrStakingTxnIndexPrefix)
	_, _ = b.Write([]byte(index.addr))
	_ = binary.Write(b, binary.BigEndian, index.blockNumber)
	_ = binary.Write(b, binary.BigEndian, index.txnIndex)
	_, _ = b.Write(index.txnHash[:])
	return b.Bytes()
}

func stakingTxnIndexPrefixByAddr(addr oneAddress) []byte {
	b := bPool.Get()
	defer b.Free()

	_, _ = b.Write(addrStakingTxnIndexPrefix)
	_, _ = b.Write([]byte(addr))
	return b.Bytes()
}

func txnHashFromStakingTxnIndexKey(key []byte) (common.Hash, error) {
	txStart := len(addrStakingTxnIndexPrefix) + oneAddrByteLen + 8 + 8
	expSize := txStart + common.HashLength
	if len(key) < expSize {
		return common.Hash{}, errors.New("unexpected key size")
	}
	var txHash common.Hash
	copy(txHash[:], key[txStart:expSize])
	return txHash, nil
}

func getStakingTxnHashesByAccount(db databaseReader, addr oneAddress) ([]common.Hash, []TxType, error) {
	var (
		txHashes []common.Hash
		tts      []TxType
	)
	prefix := stakingTxnIndexPrefixByAddr(addr)
	err := forEachAtPrefix(db, prefix, func(key, val []byte) error {
		txHash, err := txnHashFromStakingTxnIndexKey(key)
		if err != nil {
			return err
		}
		if len(val) < 0 {
			return errors.New("val size not expected")
		}
		tts = append(tts, TxType(val[0]))
		txHashes = append(txHashes, txHash)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return txHashes, tts, nil
}

func writeStakingTxnIndex(db databaseWriter, entry stakingTxnIndex, tt TxType) error {
	key := entry.key()
	return db.Put(key, []byte{byte(tt)})
}

func forEachAtPrefix(db databaseReader, prefix []byte, f func(key, val []byte) error) error {
	it := db.NewPrefixIterator(prefix)
	defer it.Release()

	for it.Next() {
		err := f(it.Key(), it.Value())
		if err != nil {
			return err
		}
	}
	return it.Error()
}

// Legacy Schema

// LegGetAddressKey ...
func LegGetAddressKey(address oneAddress) []byte {
	return []byte(fmt.Sprintf("%s%s", LegAddressPrefix, address))
}
