package rawdb

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// ReadShardState retrieves shard state of a specific epoch.
func ReadShardState(
	db DatabaseReader, epoch *big.Int,
) (*shard.State, error) {
	data, err := db.Get(shardStateKey(epoch))
	if err != nil {
		return nil, errors.Errorf(
			MsgNoShardStateFromDB, "epoch: %d", epoch.Uint64(),
		)
	}
	ss, err2 := shard.DecodeWrapper(data)
	if err2 != nil {
		return nil, errors.Wrapf(
			err2, "cannot decode sharding state",
		)
	}
	return ss, nil
}

// WriteShardStateBytes stores sharding state into database.
func WriteShardStateBytes(db DatabaseWriter, epoch *big.Int, data []byte) error {
	if err := db.Put(shardStateKey(epoch), data); err != nil {
		return errors.Wrapf(
			err, "cannot write sharding state",
		)
	}
	utils.Logger().Info().
		Str("epoch", epoch.String()).
		Int("size", len(data)).Msgf("wrote sharding state, epoch %d", epoch.Uint64())
	return nil
}

// ReadCrossLinkShardBlock retrieves the blockHash given shardID and blockNum
func ReadCrossLinkShardBlock(
	db DatabaseReader, shardID uint32, blockNum uint64,
) ([]byte, error) {
	return db.Get(crosslinkKey(shardID, blockNum))
}

// WriteCrossLinkShardBlock stores the blockHash given shardID and blockNum
func WriteCrossLinkShardBlock(db DatabaseWriter, shardID uint32, blockNum uint64, data []byte) error {
	return db.Put(crosslinkKey(shardID, blockNum), data)
}

// DeleteCrossLinkShardBlock deletes the blockHash given shardID and blockNum
func DeleteCrossLinkShardBlock(db DatabaseDeleter, shardID uint32, blockNum uint64) error {
	return db.Delete(crosslinkKey(shardID, blockNum))
}

// ReadShardLastCrossLink read the last cross link of a shard
func ReadShardLastCrossLink(db DatabaseReader, shardID uint32) ([]byte, error) {
	return db.Get(shardLastCrosslinkKey(shardID))
}

// WriteShardLastCrossLink stores the last cross link of a shard
func WriteShardLastCrossLink(db DatabaseWriter, shardID uint32, data []byte) error {
	return db.Put(shardLastCrosslinkKey(shardID), data)
}

// ReadPendingCrossLinks retrieves last pending crosslinks.
func ReadPendingCrossLinks(db DatabaseReader) ([]byte, error) {
	return db.Get(pendingCrosslinkKey)
}

// WritePendingCrossLinks stores last pending crosslinks into database.
func WritePendingCrossLinks(db DatabaseWriter, bytes []byte) error {
	return db.Put(pendingCrosslinkKey, bytes)
}

// WritePendingSlashingCandidates stores last pending slashing candidates into database.
func WritePendingSlashingCandidates(db DatabaseWriter, bytes []byte) error {
	return db.Put(pendingSlashingKey, bytes)
}

// ReadCXReceipts retrieves all the transactions of receipts given destination shardID, number and blockHash
func ReadCXReceipts(db DatabaseReader, shardID uint32, number uint64, hash common.Hash) (types.CXReceipts, error) {
	data, err := db.Get(cxReceiptKey(shardID, number, hash))
	if err != nil || len(data) == 0 {
		utils.Logger().Error().Err(err).Uint64("number", number).Int("dataLen", len(data)).Msg("ReadCXReceipts")
		return nil, err
	}
	cxReceipts := types.CXReceipts{}
	if err := rlp.DecodeBytes(data, &cxReceipts); err != nil {
		return nil, err
	}
	return cxReceipts, nil
}

// WriteCXReceipts stores all the transaction receipts given destination shardID, blockNumber and blockHash
func WriteCXReceipts(db DatabaseWriter, shardID uint32, number uint64, hash common.Hash, receipts types.CXReceipts) error {
	bytes, err := rlp.EncodeToBytes(receipts)
	if err != nil {
		utils.Logger().Error().Msg("[WriteCXReceipts] Failed to encode cross shard tx receipts")
	}
	// Store the receipt slice
	if err := db.Put(cxReceiptKey(shardID, number, hash), bytes); err != nil {
		utils.Logger().Error().Msg("[WriteCXReceipts] Failed to store cxreceipts")
	}
	return err
}

// ReadCXReceiptsProofSpent check whether a CXReceiptsProof is unspent
func ReadCXReceiptsProofSpent(db DatabaseReader, shardID uint32, number uint64) (byte, error) {
	data, err := db.Get(cxReceiptSpentKey(shardID, number))
	if err != nil || len(data) == 0 {
		return NAByte, errors.New("[ReadCXReceiptsProofSpent] Cannot find the key")
	}
	return data[0], nil
}

// WriteCXReceiptsProofSpent write CXReceiptsProof as spent into database
func WriteCXReceiptsProofSpent(dbw DatabaseWriter, cxp *types.CXReceiptsProof) error {
	shardID := cxp.MerkleProof.ShardID
	blockNum := cxp.MerkleProof.BlockNum.Uint64()
	if err := dbw.Put(cxReceiptSpentKey(shardID, blockNum), []byte{SpentByte}); err != nil {
		utils.Logger().Error().Msg("Failed to write CX receipt proof")
		return err
	}
	return nil
}

// DeleteCXReceiptsProofSpent removes unspent indicator of a given blockHash
func DeleteCXReceiptsProofSpent(db DatabaseDeleter, shardID uint32, number uint64) error {
	if err := db.Delete(cxReceiptSpentKey(shardID, number)); err != nil {
		utils.Logger().Error().Msg("Failed to delete receipts unspent indicator")
		return err
	}
	return nil
}

// ReadValidatorSnapshot retrieves validator's snapshot by its address
func ReadValidatorSnapshot(
	db DatabaseReader, addr common.Address, epoch *big.Int,
) (*staking.ValidatorSnapshot, error) {
	data, err := db.Get(validatorSnapshotKey(addr, epoch))
	if err != nil || len(data) == 0 {
		utils.Logger().Info().Err(err).Msg("ReadValidatorSnapshot")
		return nil, err
	}
	v := staking.ValidatorWrapper{}
	if err := rlp.DecodeBytes(data, &v); err != nil {
		utils.Logger().Error().Err(err).
			Str("address", addr.Hex()).
			Msg("Unable to decode validator snapshot from database")
		return nil, err
	}
	s := staking.ValidatorSnapshot{Validator: &v, Epoch: epoch}
	return &s, nil
}

// WriteValidatorSnapshot stores validator's snapshot by its address
func WriteValidatorSnapshot(batch DatabaseWriter, v *staking.ValidatorWrapper, epoch *big.Int) error {
	bytes, err := rlp.EncodeToBytes(v)
	if err != nil {
		utils.Logger().Error().Msg("[WriteValidatorSnapshot] Failed to encode")
		return err
	}
	if err := batch.Put(validatorSnapshotKey(v.Address, epoch), bytes); err != nil {
		utils.Logger().Error().Msg("[WriteValidatorSnapshot] Failed to store to database")
		return err
	}
	return err
}

// DeleteValidatorSnapshot removes the validator's snapshot by its address
func DeleteValidatorSnapshot(db DatabaseDeleter, addr common.Address, epoch *big.Int) error {
	if err := db.Delete(validatorSnapshotKey(addr, epoch)); err != nil {
		utils.Logger().Error().Msg("Failed to delete snapshot of a validator")
		return err
	}
	return nil
}

func IteratorValidatorSnapshot(iterator DatabaseIterator, cb func(addr common.Address, epoch *big.Int) bool) (minKey []byte, maxKey []byte) {
	iter := iterator.NewIterator(validatorSnapshotPrefix, nil)
	defer iter.Release()

	minKey = validatorSnapshotPrefix
	for iter.Next() {
		// validatorSnapshotKey = validatorSnapshotPrefix + addr bytes (20 bytes) + epoch bytes
		key := iter.Key()

		maxKey = key
		addressBytes := key[len(validatorSnapshotPrefix) : len(validatorSnapshotPrefix)+20]
		epochBytes := key[len(validatorSnapshotPrefix)+20:]

		if !cb(common.BytesToAddress(addressBytes), big.NewInt(0).SetBytes(epochBytes)) {
			return
		}
	}

	return
}

func IteratorCXReceipt(iterator DatabaseIterator, cb func(it ethdb.Iterator, shardID uint32, number uint64, hash common.Hash) bool) {
	preifxKey := cxReceiptPrefix
	iter := iterator.NewIterator(preifxKey, nil)
	defer iter.Release()
	shardOffset := len(preifxKey)
	numberOffset := shardOffset + 4
	hashOffset := numberOffset + 8

	for iter.Next() {
		// validatorSnapshotKey = validatorSnapshotPrefix + addr bytes (20 bytes) + epoch bytes
		key := iter.Key()
		shardID := binary.BigEndian.Uint32(key[shardOffset : shardOffset+4])
		number := binary.BigEndian.Uint64(key[numberOffset : numberOffset+8])
		hash := common.BytesToHash(key[hashOffset : hashOffset+20])
		if !cb(iter, shardID, number, hash) {
			return
		}
	}
}

func IteratorCXReceiptsProofSpent(iterator DatabaseIterator, cb func(it ethdb.Iterator, shardID uint32, number uint64) bool) {
	preifxKey := cxReceiptSpentPrefix
	iter := iterator.NewIterator(preifxKey, nil)
	defer iter.Release()
	shardOffset := len(preifxKey)
	numberOffset := shardOffset + 4

	for iter.Next() {
		// validatorSnapshotKey = validatorSnapshotPrefix + addr bytes (20 bytes) + epoch bytes
		key := iter.Key()
		shardID := binary.BigEndian.Uint32(key[shardOffset : shardOffset+4])
		number := binary.BigEndian.Uint64(key[numberOffset : numberOffset+8])
		if !cb(iter, shardID, number) {
			return
		}
	}
}
func IteratorValidatorStats(iterator DatabaseIterator, cb func(it ethdb.Iterator, addr common.Address) bool) {
	preifxKey := validatorStatsPrefix
	iter := iterator.NewIterator(preifxKey, nil)
	defer iter.Release()
	addrOffset := len(preifxKey)

	for iter.Next() {
		// validatorSnapshotKey = validatorSnapshotPrefix + addr bytes (20 bytes) + epoch bytes
		key := iter.Key()
		addr := common.BytesToAddress(key[addrOffset : addrOffset+20])
		if !cb(iter, addr) {
			return
		}
	}
}
func IteratorDelegatorDelegations(iterator DatabaseIterator, cb func(it ethdb.Iterator, delegator common.Address) bool) {
	preifxKey := delegatorValidatorListPrefix
	iter := iterator.NewIterator(preifxKey, nil)
	defer iter.Release()
	addrOffset := len(preifxKey)

	for iter.Next() {
		// validatorSnapshotKey = validatorSnapshotPrefix + addr bytes (20 bytes) + epoch bytes
		key := iter.Key()
		addr := common.BytesToAddress(key[addrOffset : addrOffset+20])
		if !cb(iter, addr) {
			return
		}
	}
}

// DeleteValidatorStats ..
func DeleteValidatorStats(db DatabaseDeleter, addr common.Address) error {
	if err := db.Delete(validatorStatsKey(addr)); err != nil {
		utils.Logger().Error().Msg("Failed to delete stats of a validator")
		return err
	}
	return nil
}

// ReadValidatorStats retrieves validator's stats by its address,
func ReadValidatorStats(
	db DatabaseReader, addr common.Address,
) (*staking.ValidatorStats, error) {
	data, err := db.Get(validatorStatsKey(addr))
	if err != nil {
		return nil, err
	}
	stats := staking.ValidatorStats{}
	if err := rlp.DecodeBytes(data, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// WriteValidatorStats stores validator's stats by its address
func WriteValidatorStats(
	batch DatabaseWriter, addr common.Address, stats *staking.ValidatorStats,
) error {
	bytes, err := rlp.EncodeToBytes(stats)
	if err != nil {
		utils.Logger().Error().Msg("[WriteValidatorStats] Failed to encode")
		return err
	}
	if err := batch.Put(validatorStatsKey(addr), bytes); err != nil {
		utils.Logger().Error().Msg("[WriteValidatorStats] Failed to store to database")
		return err
	}
	return err
}

// ReadValidatorList retrieves all staking validators by its address
func ReadValidatorList(db DatabaseReader) ([]common.Address, error) {
	key := validatorListKey
	data, err := db.Get(key)
	if err != nil || len(data) == 0 {
		return []common.Address{}, nil
	}
	addrs := []common.Address{}
	if err := rlp.DecodeBytes(data, &addrs); err != nil {
		utils.Logger().Error().Err(err).Msg("Unable to Decode validator List from database")
		return nil, err
	}
	return addrs, nil
}

// WriteValidatorList stores all staking validators by its address
func WriteValidatorList(
	db DatabaseWriter, addrs []common.Address,
) error {
	key := validatorListKey

	bytes, err := rlp.EncodeToBytes(addrs)
	if err != nil {
		utils.Logger().Error().Msg("[WriteValidatorList] Failed to encode")
	}
	return db.Put(key, bytes)
}

// ReadDelegationsByDelegator retrieves the list of validators delegated by a delegator
// Returns empty results instead of error if there is not data found.
func ReadDelegationsByDelegator(db DatabaseReader, delegator common.Address) (staking.DelegationIndexes, error) {
	data, err := db.Get(delegatorValidatorListKey(delegator))
	if err != nil || len(data) == 0 {
		return staking.DelegationIndexes{}, nil
	}
	addrs := staking.DelegationIndexes{}
	if err := rlp.DecodeBytes(data, &addrs); err != nil {
		utils.Logger().Error().Err(err).Msg("Unable to Decode delegations from database")
		return nil, err
	}
	return addrs, nil
}

// WriteDelegationsByDelegator stores the list of validators delegated by a delegator
func WriteDelegationsByDelegator(db DatabaseWriter, delegator common.Address, indexes staking.DelegationIndexes) error {
	bytes, err := rlp.EncodeToBytes(indexes)
	if err != nil {
		utils.Logger().Error().Msg("[writeDelegationsByDelegator] Failed to encode")
	}
	if err := db.Put(delegatorValidatorListKey(delegator), bytes); err != nil {
		utils.Logger().Error().Msg("[writeDelegationsByDelegator] Failed to store to database")
	}
	return err
}

// ReadBlockRewardAccumulator ..
func ReadBlockRewardAccumulator(db DatabaseReader, number uint64) (*big.Int, error) {
	data, err := db.Get(blockRewardAccumKey(number))
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(data), nil
}

// WriteBlockRewardAccumulator ..
func WriteBlockRewardAccumulator(db DatabaseWriter, newAccum *big.Int, number uint64) error {
	return db.Put(blockRewardAccumKey(number), newAccum.Bytes())
}

// ReadBlockCommitSig retrieves the signature signed on a block.
func ReadBlockCommitSig(db DatabaseReader, blockNum uint64) ([]byte, error) {
	var data []byte
	data, err := db.Get(blockCommitSigKey(blockNum))
	if err != nil {
		// TODO: remove this extra seeking of sig after the mainnet is fully upgraded.
		//       this is only needed for the compatibility in the migration moment.
		data, err = db.Get(lastCommitsKey)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("cannot read commit sig for block: %d ", blockNum))
		}
	}
	return data, nil
}

// WriteBlockCommitSig ..
func WriteBlockCommitSig(db DatabaseWriter, blockNum uint64, sigAndBitmap []byte) error {
	return db.Put(blockCommitSigKey(blockNum), sigAndBitmap)
}

//// Resharding ////

// ReadEpochBlockNumber retrieves the epoch block number for the given epoch,
// or nil if the given epoch is not found in the database.
func ReadEpochBlockNumber(db DatabaseReader, epoch *big.Int) (*big.Int, error) {
	data, err := db.Get(epochBlockNumberKey(epoch))
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(data), nil
}

// WriteEpochBlockNumber stores the given epoch-number-to-epoch-block-number in the database.
func WriteEpochBlockNumber(db DatabaseWriter, epoch, blockNum *big.Int) error {
	return db.Put(epochBlockNumberKey(epoch), blockNum.Bytes())
}

// ReadEpochVrfBlockNums retrieves the VRF block numbers for the given epoch
func ReadEpochVrfBlockNums(db DatabaseReader, epoch *big.Int) ([]byte, error) {
	return db.Get(epochVrfBlockNumbersKey(epoch))
}

// WriteEpochVrfBlockNums stores the VRF block numbers for the given epoch
func WriteEpochVrfBlockNums(db DatabaseWriter, epoch *big.Int, data []byte) error {
	return db.Put(epochVrfBlockNumbersKey(epoch), data)
}

// ReadEpochVdfBlockNum retrieves the VDF block number for the given epoch
func ReadEpochVdfBlockNum(db DatabaseReader, epoch *big.Int) ([]byte, error) {
	return db.Get(epochVdfBlockNumberKey(epoch))
}

// WriteEpochVdfBlockNum stores the VDF block number for the given epoch
func WriteEpochVdfBlockNum(db DatabaseWriter, epoch *big.Int, data []byte) error {
	return db.Put(epochVdfBlockNumberKey(epoch), data)
}

//// Resharding ////
