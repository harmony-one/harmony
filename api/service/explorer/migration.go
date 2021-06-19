package explorer

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/abool"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

const (
	// flush to db when batch reached 1 MiB
	writeThreshold      = 1 * 1024 * 1024
	legAddressPrefixLen = 3
)

func (s *storage) migrateToV100() error {
	m := &migrationV100{
		db:                s.db,
		bc:                s.bc,
		btc:               s.db.NewBatch(),
		isMigrateFinished: abool.New(),
		log: utils.Logger().With().
			Str("module", "explorer DB migration to 1.0.0").Logger(),
		finishedC: make(chan struct{}),
		closeC:    s.closeC,
	}
	return m.do()
}

var errInterrupted = errors.New("migration interrupted")

type migrationV100 struct {
	db  database
	bc  blockChainTxIndexer
	btc batch

	// progress
	migratedNum       uint64
	checkedNum        uint64
	totalNum          uint64
	isMigrateFinished *abool.AtomicBool

	log       zerolog.Logger
	finishedC chan struct{}
	closeC    chan struct{}
}

func (m *migrationV100) do() error {
	err := m.estimateTotalAddressCount()
	if err != nil {
		return errors.Wrap(err, "failed to estimate total number")
	}

	go m.progressReportLoop()
	defer close(m.finishedC)

	m.log.Info().Str("progress", fmt.Sprintf("%v / %v", 0, m.totalNum)).
		Msg("Start migration")
	if err := m.doMigration(); err != nil {
		return errors.Wrap(err, "failed to migrate to V1.0.0")
	}

	m.log.Info().Msg("Finished migration. Starting value check")
	m.isMigrateFinished.Set()

	if err := m.checkResult(); err != nil {
		return errors.Wrap(err, "check result failed")
	}
	m.log.Info().Msg("Finished result checking. Start writing version")
	if err := writeVersion(m.db, versionV100); err != nil {
		return errors.Wrap(err, "write version")
	}
	m.log.Info().Msg("Finished migration")
	return nil
}

func (m *migrationV100) progressReportLoop() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if m.isMigrateFinished.IsSet() {
				checked := atomic.LoadUint64(&m.checkedNum)
				m.log.Info().Str("progress", fmt.Sprintf("%v / %v", checked, m.totalNum)).
					Msg("checking in progress")
			} else {
				migrated := atomic.LoadUint64(&m.migratedNum)
				m.log.Info().Str("progress", fmt.Sprintf("%v / %v", migrated, m.totalNum)).
					Msg("migration in progress")
			}

		case <-m.finishedC:
			m.log.Info().Msg("migration to 1.0.0 finished")
			return

		case <-m.closeC:
			m.log.Info().Msg("Migration interrupted")
			return
		}
	}
}

func (m *migrationV100) doMigration() error {
	err := m.forEachLegacyAddressInfo(func(addr oneAddress, addrInfo *Address) error {
		if err := m.flushDBIfBatchFull(); err != nil {
			return err
		}
		select {
		case <-m.closeC:
			return errInterrupted
		default:
		}
		if err := m.migrateLegacyAddressToBatch(addr, addrInfo); err != nil {
			return err
		}
		atomic.AddUint64(&m.migratedNum, 1)
		return nil
	})
	if err != nil {
		if errors.Is(err, errInterrupted) {
			return m.btc.Write()
		}
		return err
	}
	return m.btc.Write()
}

func (m *migrationV100) estimateTotalAddressCount() error {
	m.totalNum = 0
	err := m.forEachLegacyAddressInfo(func(addr oneAddress, addrInfo *Address) error {
		m.totalNum++
		return nil
	})
	return err
}

func (m *migrationV100) forEachLegacyAddressInfo(f func(addr oneAddress, addrInfo *Address) error) error {
	it := m.db.NewPrefixIterator([]byte(LegAddressPrefix))
	defer it.Release()

	for it.Next() {
		var (
			key = string(it.Key())
			val = it.Value()
		)
		if len(key) < legAddressPrefixLen {
			return fmt.Errorf("address prefix len smaller than 3: %v", key)
		}
		addr := oneAddress(key[legAddressPrefixLen:])
		var addrInfo *Address
		if err := rlp.DecodeBytes(val, &addrInfo); err != nil {
			return errors.Wrapf(err, "address %v", addr)
		}
		if err := f(addr, addrInfo); err != nil {
			return err
		}
	}
	return nil
}

// migrateLegacyAddress migrate the legacy Address info to the new schema.
// The data is cached at batch.
func (m *migrationV100) migrateLegacyAddressToBatch(addr oneAddress, addrInfo *Address) error {
	written, err := isAddressWritten(m.db, addr)
	if written || err != nil {
		return err
	}
	for _, legTx := range addrInfo.TXs {
		if err := m.migrateLegacyNormalTx(addr, legTx); err != nil {
			return errors.Wrapf(err, "failed to migrate normal tx [%v]", legTx.Hash)
		}
	}
	for _, legTx := range addrInfo.StakingTXs {
		if err := m.migrateLegacyStakingTx(addr, legTx); err != nil {
			return errors.Wrapf(err, "failed to migrate staking tx [%v]", legTx.Hash)
		}
	}
	_ = writeAddressEntry(m.btc, addr)

	return nil
}

func (m *migrationV100) migrateLegacyNormalTx(addr oneAddress, legTx *LegTxRecord) error {
	txHash := common.HexToHash(legTx.Hash)
	_, bn, index := m.bc.ReadTxLookupEntry(txHash)
	tx, tt, err := legTxRecordToTxRecord(legTx)
	if err != nil {
		return err
	}
	_ = writeNormalTxnIndex(m.btc, normalTxnIndex{
		addr:        addr,
		blockNumber: bn,
		txnIndex:    index,
		txnHash:     txHash,
	}, tt)
	_ = writeTxn(m.btc, txHash, tx)

	return m.flushDBIfBatchFull()
}

func (m *migrationV100) migrateLegacyStakingTx(addr oneAddress, legTx *LegTxRecord) error {
	txHash := common.HexToHash(legTx.Hash)
	_, bn, index := m.bc.ReadTxLookupEntry(txHash)

	tx, tt, err := legTxRecordToTxRecord(legTx)
	if err != nil {
		return err
	}
	_ = writeStakingTxnIndex(m.btc, stakingTxnIndex{
		addr:        addr,
		blockNumber: bn,
		txnIndex:    index,
		txnHash:     txHash,
	}, tt)
	_ = writeTxn(m.btc, txHash, tx)

	return m.flushDBIfBatchFull()
}

func (m *migrationV100) flushDBIfBatchFull() error {
	if m.btc.ValueSize() > writeThreshold {
		if err := m.btc.Write(); err != nil {
			return err
		}
		m.btc = m.db.NewBatch()
	}
	return nil
}

func (m *migrationV100) checkResult() error {
	err := m.forEachLegacyAddressInfo(func(addr oneAddress, addrInfo *Address) error {
		select {
		case <-m.closeC:
			return errInterrupted
		default:
		}
		if err := m.checkMigratedAddress(addr, addrInfo); err != nil {
			return err
		}
		atomic.AddUint64(&m.checkedNum, 1)
		return nil
	})
	return err
}

func (m *migrationV100) checkMigratedAddress(addr oneAddress, addrInfo *Address) error {
	if addr == "" {
		return nil // Contract creation. Skipping
	}
	txns, _, err := getNormalTxnHashesByAccount(m.db, addr)
	if err != nil {
		return errors.Wrapf(err, "get normal txn hashes: %v", addr)
	}
	if err := m.checkHashes(txns, addrInfo.TXs); err != nil {
		return errors.Wrapf(err, "check normal tx hashes: %v", addr)
	}

	stks, _, err := getStakingTxnHashesByAccount(m.db, addr)
	if err != nil {
		return errors.Wrapf(err, "get staking txn hashes: %v", addr)
	}
	if err := m.checkHashes(stks, addrInfo.StakingTXs); err != nil {
		return errors.Wrapf(err, "check staking tx hashes: %v", addr)
	}

	for _, legTX := range append(addrInfo.TXs, addrInfo.StakingTXs...) {
		if err := m.checkMigratedTx(legTX); err != nil {
			return errors.Wrapf(err, "check tx %v for address %v", legTX.Hash, addr)
		}
	}
	return nil
}

func (m *migrationV100) checkHashes(newHashes []common.Hash, oldTxs []*LegTxRecord) error {
	oldHashMap := make(map[common.Hash]struct{})
	for _, tx := range oldTxs {
		h := common.HexToHash(tx.Hash)
		oldHashMap[h] = struct{}{}
	}
	if len(newHashes) != len(oldHashMap) {
		return fmt.Errorf("transaction number not expected: %v / %v", len(newHashes), len(oldHashMap))
	}
	for _, h := range newHashes {
		if _, ok := oldHashMap[h]; !ok {
			return fmt.Errorf("transaction %v not found", h.String())
		}
	}
	return nil
}

func (m *migrationV100) checkMigratedTx(legTx *LegTxRecord) error {
	txHash := common.HexToHash(legTx.Hash)
	newTx, err := readTxnByHash(m.db, txHash)
	if err != nil {
		return errors.Wrapf(err, "read transaction: %v", txHash.String())
	}
	if newTx.Hash.String() != legTx.Hash {
		return fmt.Errorf("hash not equal: %v / %v", newTx.Hash.String(), legTx.Hash)
	}
	if fmt.Sprintf("%v", newTx.Timestamp.Unix()) != legTx.Timestamp {
		return fmt.Errorf("timestamp not equal: %v / %v", newTx.Timestamp.Unix(), legTx.Timestamp)
	}
	return nil
}
