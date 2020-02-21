// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"io"
	"os"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
)

const (
	plainTxID   = uint64(1)
	stakingTxID = uint64(2)
)

// errNoActiveJournal is returned if a transaction is attempted to be inserted
// into the journal, but no such file is currently open.
var errNoActiveJournal = errors.New("no active journal")

// devNull is a WriteCloser that just discards anything written into it. Its
// goal is to allow the transaction journal to write into a fake journal when
// loading transactions on startup without printing warnings due to no file
// being read for write.
type devNull struct{}

func (*devNull) Write(p []byte) (n int, err error) { return len(p), nil }
func (*devNull) Close() error                      { return nil }

// txJournal is a rotating log of transactions with the aim of storing locally
// created transactions to allow non-executed ones to survive node restarts.
type txJournal struct {
	path   string         // Filesystem path to store the transactions at
	writer io.WriteCloser // Output stream to write new transactions into
}

// newTxJournal creates a new transaction journal to
func newTxJournal(path string) *txJournal {
	return &txJournal{
		path: path,
	}
}

// writeJournalTx writes a transaction journal tx to file with a leading uint64
// to identify the written transaction.
func writeJournalTx(writer io.WriteCloser, tx types.PoolTransaction) error {
	if _, ok := tx.(*types.Transaction); ok {
		if _, err := writer.Write([]byte{byte(plainTxID)}); err != nil {
			return err
		}
	} else if _, ok := tx.(*staking.StakingTransaction); ok {
		if _, err := writer.Write([]byte{byte(stakingTxID)}); err != nil {
			return err
		}
	} else {
		return types.ErrUnknownPoolTxType
	}
	return tx.EncodeRLP(writer)
}

// load parses a transaction journal dump from disk, loading its contents into
// the specified pool.
func (journal *txJournal) load(add func(types.PoolTransactions) []error) error {
	// Skip the parsing if the journal file doesn't exist at all
	if _, err := os.Stat(journal.path); os.IsNotExist(err) {
		return nil
	}
	// Open the journal for loading any past transactions
	input, err := os.Open(journal.path)
	if err != nil {
		return err
	}
	defer input.Close()

	// Temporarily discard any journal additions (don't double add on load)
	journal.writer = new(devNull)
	defer func() { journal.writer = nil }()

	// Inject all transactions from the journal into the pool
	stream := rlp.NewStream(input, 0)
	total, dropped := 0, 0
	batch := types.PoolTransactions{}

	// Create a method to load a limited batch of transactions and bump the
	// appropriate progress counters. Then use this method to load all the
	// journaled transactions in small-ish batches.
	loadBatch := func(txs types.PoolTransactions) {
		for _, err := range add(txs) {
			if err != nil {
				utils.Logger().Error().Err(err).Msg("Failed to add journaled transaction")
				dropped++
			}
		}
	}
	for {
		// Parse the next transaction and terminate on errors
		var tx types.PoolTransaction
		switch txType, err := stream.Uint(); txType {
		case plainTxID:
			tx = new(types.Transaction)
		case stakingTxID:
			tx = new(staking.StakingTransaction)
		default:
			if err != nil {
				if err == io.EOF { // reached end of journal file, exit with no error after loading batch
					err = nil
				} else {
					utils.Logger().Info().
						Int("transactions", total).
						Int("dropped", dropped).
						Msg("Loaded local transaction journal")
				}
				if batch.Len() > 0 {
					loadBatch(batch)
				}
				return err
			}
		}

		if err = stream.Decode(tx); err != nil {
			// should never hit EOF here with the leading ID journal tx encoding scheme.
			utils.Logger().Info().
				Int("transactions", total).
				Int("dropped", dropped).
				Msg("Loaded local transaction journal")
			if batch.Len() > 0 {
				loadBatch(batch)
			}
			return err
		}
		// New transaction parsed, queue up for later, import if threshold is reached
		total++

		if batch = append(batch, tx); batch.Len() > 1024 {
			loadBatch(batch)
			batch = batch[:0]
		}
	}
}

// insert adds the specified transaction to the local disk journal.
func (journal *txJournal) insert(tx types.PoolTransaction) error {
	if journal.writer == nil {
		return errNoActiveJournal
	}
	return writeJournalTx(journal.writer, tx)
}

// rotate regenerates the transaction journal based on the current contents of
// the transaction pool.
func (journal *txJournal) rotate(all map[common.Address]types.PoolTransactions) error {
	// Close the current journal (if any is open)
	if journal.writer != nil {
		if err := journal.writer.Close(); err != nil {
			return err
		}
		journal.writer = nil
	}
	// Generate a new journal with the contents of the current pool
	replacement, err := os.OpenFile(journal.path+".new", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	journaled := 0
	for _, txs := range all {
		for _, tx := range txs {
			if err = writeJournalTx(replacement, tx); err != nil {
				replacement.Close()
				return err
			}
		}
		journaled += len(txs)
	}
	replacement.Close()

	// Replace the live journal with the newly generated one
	if err = os.Rename(journal.path+".new", journal.path); err != nil {
		return err
	}
	sink, err := os.OpenFile(journal.path, os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		return err
	}
	journal.writer = sink
	utils.Logger().Info().
		Int("transactions", journaled).
		Int("accounts", len(all)).
		Msg("Regenerated local transaction journal")

	return nil
}

// close flushes the transaction journal contents to disk and closes the file.
func (journal *txJournal) close() error {
	var err error

	if journal.writer != nil {
		err = journal.writer.Close()
		journal.writer = nil
	}
	return err
}
