package explorer

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/syndtr/goleveldb/leveldb"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/internal/utils"
	goversion "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"go.uber.org/zap/buffer"
)

const (
	AddressPrefix    = "ad"
	CheckpointPrefix = "dc"

	oneAddrByteLen = 42 // byte size of string "one1..."
)

// Common schema
// GetCheckpointKey ...
func GetCheckpointKey(blockNum *big.Int) []byte {
	return []byte(fmt.Sprintf("%s_%x", CheckpointPrefix, blockNum))
}

func (storage *Storage) isBlockComputedInDB(bn uint64) bool {
	blockCheckpoint := GetCheckpointKey(new(big.Int).SetUint64(bn))
	if _, err := storage.GetDB().Get(blockCheckpoint); err == nil {
		return true
	}
	return false
}

func (storage *Storage) writeCheckpointToBatch(b batch, bn uint64) error {
	blockCheckpoint := GetCheckpointKey(new(big.Int).SetUint64(bn))
	b.Put(blockCheckpoint, []byte{})
	return nil
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
	addrNormalTxnPrefix  = []byte("at")
	addrStakingTxnPrefix = []byte("stk")
	normalTxnPrefix      = []byte("tx")
)

// bPool is the sync pool for reusing the memory for allocating db keys
var bPool = buffer.NewPool()

// getTxnKey return the transaction key. It's simply a prefix with the transaction hash
func getTxnKey(txHash common.Hash) []byte {
	b := bPool.Get()
	defer b.Free()

	_, _ = b.Write(normalTxnPrefix)
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

	_, _ = b.Write(addrNormalTxnPrefix)
	_, _ = b.Write([]byte(index.addr))
	_ = binary.Write(b, binary.BigEndian, index.blockNumber)
	_ = binary.Write(b, binary.BigEndian, index.txnIndex)
	_, _ = b.Write(index.txnHash[:])
	return b.Bytes()
}

func normalTxnIndexPrefixByAddr(addr oneAddress) []byte {
	b := bPool.Get()
	defer b.Free()

	_, _ = b.Write(addrNormalTxnPrefix)
	_, _ = b.Write([]byte(addr))
	return b.Bytes()
}

func txnHashFromNormalTxnIndexKey(key []byte) (common.Hash, error) {
	txStart := len(addrNormalTxnPrefix) + oneAddrByteLen + 8 + 8
	expSize := txStart + common.HashLength
	if len(key) < expSize {
		return common.Hash{}, errors.New("unexpected key size")
	}
	var txHash common.Hash
	copy(txHash[:], key[txStart:expSize])
	return txHash, nil
}

func getNormalTxnHashesByAccount(db databaseReader, addr oneAddress) ([]common.Hash, error) {
	var txHashes []common.Hash
	prefix := normalTxnIndexPrefixByAddr(addr)
	err := forEachAtPrefix(db, prefix, func(key, _ []byte) error {
		txHash, err := txnHashFromNormalTxnIndexKey(key)
		if err != nil {
			return err
		}
		txHashes = append(txHashes, txHash)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return txHashes, nil
}

func writeNormalTxnIndex(db databaseWriter, entry normalTxnIndex) {
	key := entry.key()
	db.Put(key, []byte{})
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

	_, _ = b.Write(addrStakingTxnPrefix)
	_, _ = b.Write([]byte(index.addr))
	_ = binary.Write(b, binary.BigEndian, index.blockNumber)
	_ = binary.Write(b, binary.BigEndian, index.txnIndex)
	_, _ = b.Write(index.txnHash[:])
	return b.Bytes()
}

func stakingTxnIndexPrefixByAddr(addr oneAddress) []byte {
	b := bPool.Get()
	defer b.Free()

	_, _ = b.Write(addrStakingTxnPrefix)
	_, _ = b.Write([]byte(addr))
	return b.Bytes()
}

func txnHashFromStakingTxnIndexKey(key []byte) (common.Hash, error) {
	txStart := len(addrStakingTxnPrefix) + oneAddrByteLen + 8 + 8
	expSize := txStart + common.HashLength
	if len(key) < expSize {
		return common.Hash{}, errors.New("unexpected key size")
	}
	var txHash common.Hash
	copy(txHash[:], key[txStart:expSize])
	return txHash, nil
}

func getStakingTxnHashesByAccount(db databaseReader, addr oneAddress) ([]common.Hash, error) {
	var txHashes []common.Hash
	prefix := stakingTxnIndexPrefixByAddr(addr)
	err := forEachAtPrefix(db, prefix, func(key, _ []byte) error {
		txHash, err := txnHashFromNormalTxnIndexKey(key)
		if err != nil {
			return err
		}
		txHashes = append(txHashes, txHash)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return txHashes, nil
}

func writeStakingTxnIndexToBatch(db databaseWriter, entry stakingTxnIndex) {
	key := entry.key()
	db.Put(key, []byte{})
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
	return nil
}

// Legacy Schema

// GetAddressKey ...
func GetAddressKey(address oneAddress) []byte {
	return []byte(fmt.Sprintf("%s_%s", AddressPrefix, address))
}

func readAddressInfoFromDB(db databaseReader, addr oneAddress) (*Address, error) {
	key := GetAddressKey(addr)
	val, err := db.Get(key)
	if err != nil {
		return nil, err
	}
	var addrInfo *Address
	if err := rlp.DecodeBytes(val, &addrInfo); err != nil {
		return nil, err
	}
	return addrInfo, nil
}

func (storage *Storage) flushLocked() error {
	storage.lock.RLock()
	b := storage.db.NewBatch()
	for addr, addressInfo := range storage.dirty {
		key := GetAddressKey(addr)
		encoded, err := rlp.EncodeToBytes(addressInfo)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("error when flushing explorer")
			return err
		}
		b.Put(key, encoded)
	}
	for bn := range storage.dirtyBNs {
		key := GetCheckpointKey(new(big.Int).SetUint64(bn))
		b.Put(key, []byte{})
	}
	storage.lock.RUnlock()

	// Hack: during db write, dirty is read-only. We can release the lock for now.
	if err := b.Write(); err != nil {
		return errors.Wrap(err, "failed to write explorer data")
	}
	// clean up
	storage.lock.Lock()
	defer storage.lock.Unlock()
	for addr, addressInfo := range storage.dirty {
		storage.clean.Add(addr, addressInfo)
	}
	storage.dirty = make(map[oneAddress]*Address)
	storage.dirtyBNs = make(map[uint64]struct{})
	return nil
}
