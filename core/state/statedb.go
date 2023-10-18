// Copyright 2014 The go-ethereum Authors
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

// Package state provides a caching layer atop the Ethereum state trie.
package state

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state/snapshot"

	types2 "github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking"
	"github.com/harmony-one/harmony/staking/effective"
	stk "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"
	"github.com/pkg/errors"
)

type revision struct {
	id           int
	journalIndex int
}

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

type proofList [][]byte

func (n *proofList) Put(key []byte, value []byte) error {
	*n = append(*n, value)
	return nil
}

func (n *proofList) Delete(key []byte) error {
	panic("not supported")
}

// DB structs within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type DB struct {
	db         Database
	prefetcher *triePrefetcher
	trie       Trie
	hasher     crypto.KeccakState

	// originalRoot is the pre-state root, before any changes were made.
	// It will be updated when the Commit is called.
	originalRoot common.Hash

	snaps        *snapshot.Tree
	snap         snapshot.Snapshot
	snapAccounts map[common.Hash][]byte
	snapStorage  map[common.Hash]map[common.Hash][]byte

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects         map[common.Address]*Object
	stateObjectsPending  map[common.Address]struct{} // State objects finalized but not yet written to the trie
	stateObjectsDirty    map[common.Address]struct{} // State objects modified in the current execution
	stateObjectsDestruct map[common.Address]struct{} // State objects destructed in the block
	stateValidators      map[common.Address]*stk.ValidatorWrapper

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by DB.Commit.
	dbErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	thash, bhash common.Hash // thash means hmy tx hash
	ethTxHash    common.Hash // ethTxHash is eth tx hash, use by tracer
	txIndex      int
	logs         map[common.Hash][]*types2.Log
	logSize      uint

	preimages map[common.Hash][]byte

	// Per-transaction access list
	accessList *accessList

	// Transient storage
	transientStorage transientStorage

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionId int

	// Measurements gathered during execution for debugging purposes
	AccountReads         time.Duration
	AccountHashes        time.Duration
	AccountUpdates       time.Duration
	AccountCommits       time.Duration
	StorageReads         time.Duration
	StorageHashes        time.Duration
	StorageUpdates       time.Duration
	StorageCommits       time.Duration
	SnapshotAccountReads time.Duration
	SnapshotStorageReads time.Duration
	SnapshotCommits      time.Duration
	TrieDBCommits        time.Duration

	AccountUpdated int
	StorageUpdated int
	AccountDeleted int
	StorageDeleted int
}

// New creates a new state from a given trie.
func New(root common.Hash, db Database, snaps *snapshot.Tree) (*DB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	sdb := &DB{
		db:                   db,
		trie:                 tr,
		originalRoot:         root,
		snaps:                snaps,
		stateObjects:         make(map[common.Address]*Object),
		stateObjectsPending:  make(map[common.Address]struct{}),
		stateObjectsDirty:    make(map[common.Address]struct{}),
		stateObjectsDestruct: make(map[common.Address]struct{}),
		stateValidators:      make(map[common.Address]*stk.ValidatorWrapper),
		logs:                 make(map[common.Hash][]*types2.Log),
		preimages:            make(map[common.Hash][]byte),
		journal:              newJournal(),
		accessList:           newAccessList(),
		transientStorage:     newTransientStorage(),
		hasher:               crypto.NewKeccakState(),
	}
	if sdb.snaps != nil {
		if sdb.snap = sdb.snaps.Snapshot(root); sdb.snap != nil {
			sdb.snapAccounts = make(map[common.Hash][]byte)
			sdb.snapStorage = make(map[common.Hash]map[common.Hash][]byte)
		}
	}
	return sdb, nil
}

// StartPrefetcher initializes a new trie prefetcher to pull in nodes from the
// state trie concurrently while the state is mutated so that when we reach the
// commit phase, most of the needed data is already hot.
func (db *DB) StartPrefetcher(namespace string) {
	if db.prefetcher != nil {
		db.prefetcher.close()
		db.prefetcher = nil
	}
	if db.snap != nil {
		db.prefetcher = newTriePrefetcher(db.db, db.originalRoot, namespace)
	}
}

// StopPrefetcher terminates a running prefetcher and reports any leftover stats
// from the gathered metrics.
func (db *DB) StopPrefetcher() {
	if db.prefetcher != nil {
		db.prefetcher.close()
		db.prefetcher = nil
	}
}

// setError remembers the first non-nil error it is called with.
func (db *DB) setError(err error) {
	if db.dbErr == nil {
		db.dbErr = err
	}
}

func (db *DB) Error() error {
	return db.dbErr
}

// Reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (db *DB) Reset(root common.Hash) error {
	tr, err := db.db.OpenTrie(root)
	if err != nil {
		return err
	}
	db.trie = tr
	db.stateObjects = make(map[common.Address]*Object)
	db.stateObjectsPending = make(map[common.Address]struct{})
	db.stateObjectsDirty = make(map[common.Address]struct{})
	db.stateValidators = make(map[common.Address]*stk.ValidatorWrapper)
	db.thash = common.Hash{}
	db.bhash = common.Hash{}
	db.ethTxHash = common.Hash{}
	db.txIndex = 0
	db.logs = make(map[common.Hash][]*types2.Log)
	db.logSize = 0
	db.preimages = make(map[common.Hash][]byte)
	db.clearJournalAndRefund()
	return nil
}

func (db *DB) AddLog(log *types2.Log) {
	db.journal.append(addLogChange{txhash: db.thash})

	log.TxHash = db.thash
	log.BlockHash = db.bhash
	log.TxIndex = uint(db.txIndex)
	log.Index = db.logSize
	db.logs[db.thash] = append(db.logs[db.thash], log)
	db.logSize++
}

// GetLogs returns the logs matching the specified transaction hash, and annotates
// them with the given blockNumber and blockHash.
func (db *DB) GetLogs(hash common.Hash, blockNumber uint64, blockHash common.Hash) []*types2.Log {
	logs := db.logs[hash]
	for _, l := range logs {
		l.BlockNumber = blockNumber
		l.BlockHash = blockHash
	}
	return logs
}

func (db *DB) Logs() []*types2.Log {
	var logs []*types2.Log
	for _, lgs := range db.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// AddPreimage records a SHA3 preimage seen by the VM.
func (db *DB) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := db.preimages[hash]; !ok {
		db.journal.append(addPreimageChange{hash: hash})
		pi := make([]byte, len(preimage))
		copy(pi, preimage)
		db.preimages[hash] = pi
	}
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (db *DB) Preimages() map[common.Hash][]byte {
	return db.preimages
}

// AddRefund adds gas to the refund counter
func (db *DB) AddRefund(gas uint64) {
	db.journal.append(refundChange{prev: db.refund})
	db.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (db *DB) SubRefund(gas uint64) {
	db.journal.append(refundChange{prev: db.refund})
	if gas > db.refund {
		panic(fmt.Sprintf("Refund counter below zero (gas: %d > refund: %d)", gas, db.refund))
	}
	db.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (db *DB) Exist(addr common.Address) bool {
	return db.getStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (db *DB) Empty(addr common.Address) bool {
	so := db.getStateObject(addr)
	return so == nil || so.empty()
}

// GetBalance retrieves the balance from the given address or 0 if object not found
func (db *DB) GetBalance(addr common.Address) *big.Int {
	Object := db.getStateObject(addr)
	if Object != nil {
		return Object.Balance()
	}
	return common.Big0
}

func (db *DB) GetNonce(addr common.Address) uint64 {
	Object := db.getStateObject(addr)
	if Object != nil {
		return Object.Nonce()
	}

	return 0
}

// TxIndex returns the current transaction index set by Prepare.
func (db *DB) TxIndex() int {
	return db.txIndex
}

func (db *DB) TxHash() common.Hash {
	return db.thash
}

func (db *DB) TxHashETH() common.Hash {
	return db.ethTxHash
}

// BlockHash returns the current block hash set by Prepare.
func (db *DB) BlockHash() common.Hash {
	return db.bhash
}

func (db *DB) GetCode(addr common.Address) []byte {
	Object := db.getStateObject(addr)
	if Object != nil {
		return Object.Code(db.db)
	}
	return nil
}

func (db *DB) GetCodeSize(addr common.Address) int {
	Object := db.getStateObject(addr)
	if Object != nil {
		return Object.CodeSize(db.db)
	}
	return 0
}

func (db *DB) GetCodeHash(addr common.Address) common.Hash {
	Object := db.getStateObject(addr)
	if Object == nil {
		return common.Hash{}
	}
	return common.BytesToHash(Object.CodeHash())
}

// GetState retrieves a value from the given account's storage trie.
func (db *DB) GetState(addr common.Address, hash common.Hash) common.Hash {
	Object := db.getStateObject(addr)
	if Object != nil {
		return Object.GetState(db.db, hash)
	}
	return common.Hash{}
}

// GetProof returns the Merkle proof for a given account.
func (db *DB) GetProof(addr common.Address) ([][]byte, error) {
	return db.GetProofByHash(crypto.Keccak256Hash(addr.Bytes()))
}

// GetProofByHash returns the Merkle proof for a given account.
func (db *DB) GetProofByHash(addrHash common.Hash) ([][]byte, error) {
	var proof proofList
	err := db.trie.Prove(addrHash[:], 0, &proof)
	return proof, err
}

// GetStorageProof returns the Merkle proof for given storage slot.
func (db *DB) GetStorageProof(a common.Address, key common.Hash) ([][]byte, error) {
	trie, err := db.StorageTrie(a)
	if err != nil {
		return nil, err
	}
	if trie == nil {
		return nil, errors.New("storage trie for requested address does not exist")
	}
	var proof proofList
	err = trie.Prove(crypto.Keccak256(key.Bytes()), 0, &proof)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (db *DB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	Object := db.getStateObject(addr)
	if Object != nil {
		return Object.GetCommittedState(db.db, hash)
	}
	return common.Hash{}
}

// Database retrieves the low level database supporting the lower level trie ops.
func (db *DB) Database() Database {
	return db.db
}

// StorageTrie returns the storage trie of an account. The return value is a copy
// and is nil for non-existent accounts. An error will be returned if storage trie
// is existent but can't be loaded correctly.
func (db *DB) StorageTrie(addr common.Address) (Trie, error) {
	Object := db.getStateObject(addr)
	if Object == nil {
		return nil, nil
	}
	cpy := Object.deepCopy(db)
	if _, err := cpy.updateTrie(db.db); err != nil {
		return nil, err
	}
	return cpy.getTrie(db.db)
}

func (db *DB) HasSuicided(addr common.Address) bool {
	Object := db.getStateObject(addr)
	if Object != nil {
		return Object.suicided
	}
	return false
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr.
func (db *DB) AddBalance(addr common.Address, amount *big.Int) {
	Object := db.GetOrNewStateObject(addr)
	if Object != nil {
		Object.AddBalance(amount)
	}
}

// SubBalance subtracts amount from the account associated with addr.
func (db *DB) SubBalance(addr common.Address, amount *big.Int) {
	Object := db.GetOrNewStateObject(addr)
	if Object != nil {
		Object.SubBalance(amount)
	}
}

func (db *DB) SetBalance(addr common.Address, amount *big.Int) {
	Object := db.GetOrNewStateObject(addr)
	if Object != nil {
		Object.SetBalance(amount)
	}
}

func (db *DB) SetNonce(addr common.Address, nonce uint64) {
	Object := db.GetOrNewStateObject(addr)
	if Object != nil {
		Object.SetNonce(nonce)
	}
}

func (db *DB) SetCode(addr common.Address, code []byte, isValidatorCode bool) {
	Object := db.GetOrNewStateObject(addr)
	if Object != nil {
		Object.SetCode(crypto.Keccak256Hash(code), code, isValidatorCode)
	}
}

func (db *DB) SetState(addr common.Address, key, value common.Hash) {
	Object := db.GetOrNewStateObject(addr)
	if Object != nil {
		Object.SetState(db.db, key, value)
	}
}

// SetStorage replaces the entire storage for the specified account with given
// storage. This function should only be used for debugging.
func (db *DB) SetStorage(addr common.Address, storage map[common.Hash]common.Hash) {
	// SetStorage needs to wipe existing storage. We achieve this by pretending
	// that the account self-destructed earlier in this block, by flagging
	// it in stateObjectsDestruct. The effect of doing so is that storage lookups
	// will not hit disk, since it is assumed that the disk-data is belonging
	// to a previous incarnation of the object.
	db.stateObjectsDestruct[addr] = struct{}{}
	Object := db.GetOrNewStateObject(addr)
	for k, v := range storage {
		Object.SetState(db.db, k, v)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (db *DB) Suicide(addr common.Address) bool {
	Object := db.getStateObject(addr)
	if Object == nil {
		return false
	}
	db.journal.append(suicideChange{
		account:     &addr,
		prev:        Object.suicided,
		prevbalance: new(big.Int).Set(Object.Balance()),
	})
	Object.markSuicided()
	Object.data.Balance = new(big.Int)

	return true
}

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (db *DB) SetTransientState(addr common.Address, key, value common.Hash) {
	prev := db.GetTransientState(addr, key)
	if prev == value {
		return
	}

	db.journal.append(transientStorageChange{
		account:  &addr,
		key:      key,
		prevalue: prev,
	})

	db.setTransientState(addr, key, value)
}

// setTransientState is a lower level setter for transient storage. It
// is called during a revert to prevent modifications to the journal.
func (db *DB) setTransientState(addr common.Address, key, value common.Hash) {
	db.transientStorage.Set(addr, key, value)
}

// GetTransientState gets transient storage for a given account.
func (db *DB) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return db.transientStorage.Get(addr, key)
}

//
// Setting, updating & deleting state object methods.
//

// updateStateObject writes the given object to the trie.
func (db *DB) updateStateObject(obj *Object) {
	// Track the amount of time wasted on updating the account from the trie
	if metrics.EnabledExpensive {
		defer func(start time.Time) { db.AccountUpdates += time.Since(start) }(time.Now())
	}
	// Encode the account and update the account trie
	addr := obj.Address()
	if err := db.trie.TryUpdateAccount(addr, &obj.data); err != nil {
		db.setError(fmt.Errorf("updateStateObject (%x) error: %v", addr[:], err))
	}

	// If state snapshotting is active, cache the data til commit. Note, this
	// update mechanism is not symmetric to the deletion, because whereas it is
	// enough to track account updates at commit time, deletions need tracking
	// at transaction boundary level to ensure we capture state clearing.
	if db.snap != nil {
		db.snapAccounts[obj.addrHash] = snapshot.SlimAccountRLP(obj.data.Nonce, obj.data.Balance, obj.data.Root, obj.data.CodeHash)
	}
}

// deleteStateObject removes the given object from the state trie.
func (db *DB) deleteStateObject(obj *Object) {
	// Track the amount of time wasted on deleting the account from the trie
	if metrics.EnabledExpensive {
		defer func(start time.Time) { db.AccountUpdates += time.Since(start) }(time.Now())
	}
	// Delete the account from the trie
	addr := obj.Address()
	if err := db.trie.TryDeleteAccount(addr); err != nil {
		db.setError(fmt.Errorf("deleteStateObject (%x) error: %v", addr[:], err))
	}
}

// getStateObject retrieves a state object given by the address, returning nil if
// the object is not found or was deleted in this execution context. If you need
// to differentiate between non-existent/just-deleted, use getDeletedStateObject.
func (db *DB) getStateObject(addr common.Address) *Object {
	if obj := db.getDeletedStateObject(addr); obj != nil && !obj.deleted {
		return obj
	}
	return nil
}

// getDeletedStateObject is similar to getStateObject, but instead of returning
// nil for a deleted state object, it returns the actual object with the deleted
// flag set. This is needed by the state journal to revert to the correct s-
// destructed object instead of wiping all knowledge about the state object.
func (db *DB) getDeletedStateObject(addr common.Address) *Object {
	// Prefer live objects if any is available
	if obj := db.stateObjects[addr]; obj != nil {
		return obj
	}
	// If no live objects are available, attempt to use snapshots
	var data *types.StateAccount
	if db.snap != nil {
		start := time.Now()
		acc, err := db.snap.Account(crypto.HashData(db.hasher, addr.Bytes()))
		if metrics.EnabledExpensive {
			db.SnapshotAccountReads += time.Since(start)
		}
		if err == nil {
			if acc == nil {
				return nil
			}
			data = &types.StateAccount{
				Nonce:    acc.Nonce,
				Balance:  acc.Balance,
				CodeHash: acc.CodeHash,
				Root:     common.BytesToHash(acc.Root),
			}
			if len(data.CodeHash) == 0 {
				data.CodeHash = types.EmptyCodeHash.Bytes()
			}
			if data.Root == (common.Hash{}) {
				data.Root = types.EmptyRootHash
			}
		}
	}
	// If snapshot unavailable or reading from it failed, load from the database
	if data == nil {
		start := time.Now()
		var err error
		data, err = db.trie.TryGetAccount(addr)
		if metrics.EnabledExpensive {
			db.AccountReads += time.Since(start)
		}
		if err != nil {
			db.setError(fmt.Errorf("getDeleteStateObject (%x) error: %w", addr.Bytes(), err))
			return nil
		}
		if data == nil {
			return nil
		}
	}
	// Insert into the live set
	obj := newObject(db, addr, *data)
	db.setStateObject(obj)
	return obj
}

func (db *DB) setStateObject(object *Object) {
	db.stateObjects[object.Address()] = object
}

// GetOrNewStateObject retrieves a state object or create a new state object if nil.
func (db *DB) GetOrNewStateObject(addr common.Address) *Object {
	Object := db.getStateObject(addr)
	if Object == nil {
		Object, _ = db.createObject(addr)
	}
	return Object
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (db *DB) createObject(addr common.Address) (newobj, prev *Object) {
	prev = db.getDeletedStateObject(addr) // Note, prev might have been deleted, we need that!

	var prevdestruct bool
	if prev != nil {
		_, prevdestruct = db.stateObjectsDestruct[prev.address]
		if !prevdestruct {
			db.stateObjectsDestruct[prev.address] = struct{}{}
		}
	}
	newobj = newObject(db, addr, types.StateAccount{})
	if prev == nil {
		db.journal.append(createObjectChange{account: &addr})
	} else {
		db.journal.append(resetObjectChange{prev: prev, prevdestruct: prevdestruct})
	}
	db.setStateObject(newobj)
	if prev != nil && !prev.deleted {
		return newobj, prev
	}
	return newobj, nil
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a contract does the following:
//
//  1. sends funds to sha(account ++ (nonce + 1))
//  2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that Ether doesn't disappear.
func (db *DB) CreateAccount(addr common.Address) {
	newObj, prev := db.createObject(addr)
	if prev != nil {
		newObj.setBalance(prev.data.Balance)
	}
}

func (db *DB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) error {
	so := db.getStateObject(addr)
	if so == nil {
		return nil
	}
	tr, err := so.getTrie(db.db)
	if err != nil {
		return err
	}
	it := trie.NewIterator(tr.NodeIterator(nil))

	for it.Next() {
		key := common.BytesToHash(db.trie.GetKey(it.Key))
		if value, dirty := so.dirtyStorage[key]; dirty {
			if !cb(key, value) {
				return nil
			}
			continue
		}

		if len(it.Value) > 0 {
			_, content, _, err := rlp.Split(it.Value)
			if err != nil {
				return err
			}
			if !cb(key, common.BytesToHash(content)) {
				return nil
			}
		}
	}
	return nil
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (db *DB) Copy() *DB {
	// Copy all the basic fields, initialize the memory ones
	state := &DB{
		db:                   db.db,
		trie:                 db.db.CopyTrie(db.trie),
		originalRoot:         db.originalRoot,
		stateObjects:         make(map[common.Address]*Object, len(db.journal.dirties)),
		stateObjectsPending:  make(map[common.Address]struct{}, len(db.stateObjectsPending)),
		stateObjectsDirty:    make(map[common.Address]struct{}, len(db.journal.dirties)),
		stateObjectsDestruct: make(map[common.Address]struct{}, len(db.stateObjectsDestruct)),
		stateValidators:      make(map[common.Address]*stk.ValidatorWrapper),
		refund:               db.refund,
		logs:                 make(map[common.Hash][]*types2.Log, len(db.logs)),
		logSize:              db.logSize,
		preimages:            make(map[common.Hash][]byte, len(db.preimages)),
		journal:              newJournal(),
		hasher:               crypto.NewKeccakState(),
	}
	// Copy the dirty states, logs, and preimages
	for addr := range db.journal.dirties {
		// As documented [here](https://github.com/ethereum/go-ethereum/pull/16485#issuecomment-380438527),
		// and in the Finalise-method, there is a case where an object is in the journal but not
		// in the stateObjects: OOG after touch on ripeMD prior to Byzantium. Thus, we need to check for
		// nil
		if object, exist := db.stateObjects[addr]; exist {
			// Even though the original object is dirty, we are not copying the journal,
			// so we need to make sure that any side-effect the journal would have caused
			// during a commit (or similar op) is already applied to the copy.
			state.stateObjects[addr] = object.deepCopy(state)

			state.stateObjectsDirty[addr] = struct{}{}   // Mark the copy dirty to force internal (code/state) commits
			state.stateObjectsPending[addr] = struct{}{} // Mark the copy pending to force external (account) commits
		}
	}
	// Above, we don't copy the actual journal. This means that if the copy
	// is copied, the loop above will be a no-op, since the copy's journal
	// is empty. Thus, here we iterate over stateObjects, to enable copies
	// of copies.
	for addr := range db.stateObjectsPending {
		if _, exist := state.stateObjects[addr]; !exist {
			state.stateObjects[addr] = db.stateObjects[addr].deepCopy(state)
		}
		state.stateObjectsPending[addr] = struct{}{}
	}
	for addr := range db.stateObjectsDirty {
		if _, exist := state.stateObjects[addr]; !exist {
			state.stateObjects[addr] = db.stateObjects[addr].deepCopy(state)
		}
		state.stateObjectsDirty[addr] = struct{}{}
	}
	// Deep copy the destruction flag.
	for addr := range db.stateObjectsDestruct {
		state.stateObjectsDestruct[addr] = struct{}{}
	}
	for addr, wrapper := range db.stateValidators {
		copied := staketest.CopyValidatorWrapper(*wrapper)
		state.stateValidators[addr] = &copied
	}
	for hash, logs := range db.logs {
		cpy := make([]*types2.Log, len(logs))
		for i, l := range logs {
			cpy[i] = new(types2.Log)
			*cpy[i] = *l
		}
		state.logs[hash] = cpy
	}
	for hash, preimage := range db.preimages {
		state.preimages[hash] = preimage
	}
	// Do we need to copy the access list and transient storage?
	// In practice: No. At the start of a transaction, these two lists are empty.
	// In practice, we only ever copy state _between_ transactions/blocks, never
	// in the middle of a transaction. However, it doesn't cost us much to copy
	// empty lists, so we do it anyway to not blow up if we ever decide copy them
	// in the middle of a transaction.
	state.accessList = db.accessList.Copy()
	state.transientStorage = db.transientStorage.Copy()

	// If there's a prefetcher running, make an inactive copy of it that can
	// only access data but does not actively preload (since the user will not
	// know that they need to explicitly terminate an active copy).
	if db.prefetcher != nil {
		state.prefetcher = db.prefetcher.copy()
	}
	if db.snaps != nil {
		// In order for the miner to be able to use and make additions
		// to the snapshot tree, we need to copy that as well.
		// Otherwise, any block mined by ourselves will cause gaps in the tree,
		// and force the miner to operate trie-backed only
		state.snaps = db.snaps
		state.snap = db.snap

		// deep copy needed
		state.snapAccounts = make(map[common.Hash][]byte)
		for k, v := range db.snapAccounts {
			state.snapAccounts[k] = v
		}
		state.snapStorage = make(map[common.Hash]map[common.Hash][]byte)
		for k, v := range db.snapStorage {
			temp := make(map[common.Hash][]byte)
			for kk, vv := range v {
				temp[kk] = vv
			}
			state.snapStorage[k] = temp
		}
	}
	return state
}

// Snapshot returns an identifier for the current revision of the state.
func (db *DB) Snapshot() int {
	id := db.nextRevisionId
	db.nextRevisionId++
	db.validRevisions = append(db.validRevisions, revision{id, db.journal.length()})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (db *DB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(db.validRevisions), func(i int) bool {
		return db.validRevisions[i].id >= revid
	})
	if idx == len(db.validRevisions) || db.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := db.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	db.journal.revert(db, snapshot)
	db.validRevisions = db.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
func (db *DB) GetRefund() uint64 {
	return db.refund
}

// Finalise finalises the state by removing the destructed objects and clears
// the journal as well as the refunds. Finalise, however, will not push any updates
// into the tries just yet. Only IntermediateRoot or Commit will do that.
func (db *DB) Finalise(deleteEmptyObjects bool) {
	// Commit validator changes in cache to stateObjects
	// TODO: remove validator cache after commit
	for addr, wrapper := range db.stateValidators {
		db.UpdateValidatorWrapper(addr, wrapper)
	}
	addressesToPrefetch := make([][]byte, 0, len(db.journal.dirties))
	for addr := range db.journal.dirties {
		obj, exist := db.stateObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `db.journal.dirties` but not in `db.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}
		if obj.suicided || (deleteEmptyObjects && obj.empty()) {
			obj.deleted = true

			// We need to maintain account deletions explicitly (will remain
			// set indefinitely).
			db.stateObjectsDestruct[obj.address] = struct{}{}

			// If state snapshotting is active, also mark the destruction there.
			// Note, we can't do this only at the end of a block because multiple
			// transactions within the same block might self destruct and then
			// resurrect an account; but the snapshotter needs both events.
			if db.snap != nil {
				delete(db.snapAccounts, obj.addrHash) // Clear out any previously updated account data (may be recreated via a resurrect)
				delete(db.snapStorage, obj.addrHash)  // Clear out any previously updated storage data (may be recreated via a resurrect)
			}
		} else {
			obj.finalise(true) // Prefetch slots in the background
		}
		db.stateObjectsPending[addr] = struct{}{}
		db.stateObjectsDirty[addr] = struct{}{}

		// At this point, also ship the address off to the precacher. The precacher
		// will start loading tries, and when the change is eventually committed,
		// the commit-phase will be a lot faster
		addressesToPrefetch = append(addressesToPrefetch, common.CopyBytes(addr[:])) // Copy needed for closure
	}
	if db.prefetcher != nil && len(addressesToPrefetch) > 0 {
		db.prefetcher.prefetch(common.Hash{}, db.originalRoot, addressesToPrefetch)
	}
	// Invalidate journal because reverting across transactions is not allowed.
	db.clearJournalAndRefund()
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (db *DB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	// Finalise all the dirty storage states and write them into the tries
	db.Finalise(deleteEmptyObjects)

	// If there was a trie prefetcher operating, it gets aborted and irrevocably
	// modified after we start retrieving tries. Remove it from the statedb after
	// this round of use.
	//
	// This is weird pre-byzantium since the first tx runs with a prefetcher and
	// the remainder without, but pre-byzantium even the initial prefetcher is
	// useless, so no sleep lost.
	prefetcher := db.prefetcher
	if db.prefetcher != nil {
		defer func() {
			db.prefetcher.close()
			db.prefetcher = nil
		}()
	}
	// Although naively it makes sense to retrieve the account trie and then do
	// the contract storage and account updates sequentially, that short circuits
	// the account prefetcher. Instead, let's process all the storage updates
	// first, giving the account prefetches just a few more milliseconds of time
	// to pull useful data from disk.
	for addr := range db.stateObjectsPending {
		if obj := db.stateObjects[addr]; !obj.deleted {
			obj.updateRoot(db.db)
		}
	}
	// Now we're about to start to write changes to the trie. The trie is so far
	// _untouched_. We can check with the prefetcher, if it can give us a trie
	// which has the same root, but also has some content loaded into it.
	if prefetcher != nil {
		if trie := prefetcher.trie(common.Hash{}, db.originalRoot); trie != nil {
			db.trie = trie
		}
	}
	usedAddrs := make([][]byte, 0, len(db.stateObjectsPending))
	for addr := range db.stateObjectsPending {
		if obj := db.stateObjects[addr]; obj.deleted {
			db.deleteStateObject(obj)
			db.AccountDeleted += 1
		} else {
			db.updateStateObject(obj)
			db.AccountUpdated += 1
		}
		usedAddrs = append(usedAddrs, common.CopyBytes(addr[:])) // Copy needed for closure
	}
	if prefetcher != nil {
		prefetcher.used(common.Hash{}, db.originalRoot, usedAddrs)
	}
	if len(db.stateObjectsPending) > 0 {
		db.stateObjectsPending = make(map[common.Address]struct{})
	}
	// Track the amount of time wasted on hashing the account trie
	if metrics.EnabledExpensive {
		defer func(start time.Time) { db.AccountHashes += time.Since(start) }(time.Now())
	}
	return db.trie.Hash()
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the EVM emits new state logs.
func (db *DB) Prepare(thash, bhash common.Hash, ti int) {
	db.thash = thash
	db.bhash = bhash
	db.txIndex = ti
}

// SetTxContext sets the current transaction hash and index which are
// used when the EVM emits new state logs. It should be invoked before
// transaction execution.
func (db *DB) SetTxContext(thash common.Hash, ti int) {
	db.thash = thash
	db.txIndex = ti
}

func (db *DB) clearJournalAndRefund() {
	if len(db.journal.entries) > 0 {
		db.journal = newJournal()
		db.refund = 0
	}
	db.validRevisions = db.validRevisions[:0] // Snapshots can be created without journal entries
}

func (db *DB) SetTxHashETH(ethTxHash common.Hash) {
	db.ethTxHash = ethTxHash
}

// Commit writes the state to the underlying in-memory trie database.
func (db *DB) Commit(deleteEmptyObjects bool) (common.Hash, error) {
	if db.dbErr != nil {
		return common.Hash{}, fmt.Errorf("commit aborted due to earlier error: %v", db.dbErr)
	}
	// Finalize any pending changes and merge everything into the tries
	db.IntermediateRoot(deleteEmptyObjects)

	// Commit objects to the trie, measuring the elapsed time
	var (
		accountTrieNodesUpdated int
		accountTrieNodesDeleted int
		storageTrieNodesUpdated int
		storageTrieNodesDeleted int
		nodes                   = trie.NewMergedNodeSet()
	)
	codeWriter := db.db.DiskDB().NewBatch()
	for addr := range db.stateObjectsDirty {
		if obj := db.stateObjects[addr]; !obj.deleted {
			// Write any contract code associated with the state object
			if obj.code != nil && obj.dirtyCode {
				if obj.validatorWrapper {
					rawdb.WriteValidatorCode(codeWriter, common.BytesToHash(obj.CodeHash()), obj.code)
				} else {
					rawdb.WriteCode(codeWriter, common.BytesToHash(obj.CodeHash()), obj.code)
				}
				obj.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie
			set, err := obj.commitTrie(db.db)
			if err != nil {
				return common.Hash{}, err
			}
			// Merge the dirty nodes of storage trie into global set
			if set != nil {
				if err := nodes.Merge(set); err != nil {
					return common.Hash{}, err
				}
				updates, deleted := set.Size()
				storageTrieNodesUpdated += updates
				storageTrieNodesDeleted += deleted
			}
		}
		// If the contract is destructed, the storage is still left in the
		// database as dangling data. Theoretically it's should be wiped from
		// database as well, but in hash-based-scheme it's extremely hard to
		// determine that if the trie nodes are also referenced by other storage,
		// and in path-based-scheme some technical challenges are still unsolved.
		// Although it won't affect the correctness but please fix it TODO(rjl493456442).
	}
	if len(db.stateObjectsDirty) > 0 {
		db.stateObjectsDirty = make(map[common.Address]struct{})
	}
	if codeWriter.ValueSize() > 0 {
		if err := codeWriter.Write(); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to commit dirty codes")
		}
	}
	// Write the account trie changes, measuring the amount of wasted time
	var start time.Time
	if metrics.EnabledExpensive {
		start = time.Now()
	}
	root, set := db.trie.Commit(true)
	// Merge the dirty nodes of account trie into global set
	if set != nil {
		if err := nodes.Merge(set); err != nil {
			return common.Hash{}, err
		}
		accountTrieNodesUpdated, accountTrieNodesDeleted = set.Size()
	}
	if metrics.EnabledExpensive {
		db.AccountCommits += time.Since(start)

		accountUpdatedMeter.Mark(int64(db.AccountUpdated))
		storageUpdatedMeter.Mark(int64(db.StorageUpdated))
		accountDeletedMeter.Mark(int64(db.AccountDeleted))
		storageDeletedMeter.Mark(int64(db.StorageDeleted))
		accountTrieUpdatedMeter.Mark(int64(accountTrieNodesUpdated))
		accountTrieDeletedMeter.Mark(int64(accountTrieNodesDeleted))
		storageTriesUpdatedMeter.Mark(int64(storageTrieNodesUpdated))
		storageTriesDeletedMeter.Mark(int64(storageTrieNodesDeleted))
		db.AccountUpdated, db.AccountDeleted = 0, 0
		db.StorageUpdated, db.StorageDeleted = 0, 0
	}
	// If snapshotting is enabled, update the snapshot tree with this new version
	if db.snap != nil {
		start := time.Now()
		// Only update if there's a state transition (skip empty Clique blocks)
		if parent := db.snap.Root(); parent != root {
			if err := db.snaps.Update(root, parent, db.convertAccountSet(db.stateObjectsDestruct), db.snapAccounts, db.snapStorage); err != nil {
				utils.Logger().Warn().Err(err).
					Interface("from", parent).
					Interface("to", root).
					Msg("Failed to update snapshot tree")
			}
			// Keep 128 diff layers in the memory, persistent layer is 129th.
			// - head layer is paired with HEAD state
			// - head-1 layer is paired with HEAD-1 state
			// - head-127 layer(bottom-most diff layer) is paired with HEAD-127 state
			if err := db.snaps.Cap(root, 128); err != nil {
				utils.Logger().Warn().Err(err).
					Interface("root", root).
					Uint16("layers", 128).
					Msg("Failed to cap snapshot tree")
			}
		}
		if metrics.EnabledExpensive {
			db.SnapshotCommits += time.Since(start)
		}
		db.snap, db.snapAccounts, db.snapStorage = nil, nil, nil
	}
	if len(db.stateObjectsDestruct) > 0 {
		db.stateObjectsDestruct = make(map[common.Address]struct{})
	}
	if root == (common.Hash{}) {
		root = types.EmptyRootHash
	}
	origin := db.originalRoot
	if origin == (common.Hash{}) {
		origin = types.EmptyRootHash
	}
	if root != origin {
		start := time.Now()
		if err := db.db.TrieDB().Update(nodes); err != nil {
			return common.Hash{}, err
		}
		db.originalRoot = root
		if metrics.EnabledExpensive {
			db.TrieDBCommits += time.Since(start)
		}
	}
	return root, nil
}

// AddAddressToAccessList adds the given address to the access list
func (db *DB) AddAddressToAccessList(addr common.Address) {
	if db.accessList.AddAddress(addr) {
		db.journal.append(accessListAddAccountChange{&addr})
	}
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (db *DB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	addrMod, slotMod := db.accessList.AddSlot(addr, slot)
	if addrMod {
		// In practice, this should not happen, since there is no way to enter the
		// scope of 'address' without having the 'address' become already added
		// to the access list (via call-variant, create, etc).
		// Better safe than sorry, though
		db.journal.append(accessListAddAccountChange{&addr})
	}
	if slotMod {
		db.journal.append(accessListAddSlotChange{
			address: &addr,
			slot:    &slot,
		})
	}
}

// AddressInAccessList returns true if the given address is in the access list.
func (db *DB) AddressInAccessList(addr common.Address) bool {
	return db.accessList.ContainsAddress(addr)
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (db *DB) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	return db.accessList.Contains(addr, slot)
}

// convertAccountSet converts a provided account set from address keyed to hash keyed.
func (db *DB) convertAccountSet(set map[common.Address]struct{}) map[common.Hash]struct{} {
	ret := make(map[common.Hash]struct{})
	for addr := range set {
		obj, exist := db.stateObjects[addr]
		if !exist {
			ret[crypto.Keccak256Hash(addr[:])] = struct{}{}
		} else {
			ret[obj.addrHash] = struct{}{}
		}
	}
	return ret
}

var (
	ErrAddressNotPresent = errors.New("address not present in state")
)

// ValidatorWrapper retrieves the existing validator in the cache, if sendOriginal
// else it will return a copy of the wrapper - which needs to be explicitly committed
// with UpdateValidatorWrapper.
// To conserve memory, the copy can optionally avoid deep copying delegations.
// Revert in UpdateValidatorWrapper does not work if sendOriginal == true.
func (db *DB) ValidatorWrapper(
	addr common.Address,
	sendOriginal bool,
	copyDelegations bool,
) (*stk.ValidatorWrapper, error) {
	// if cannot revert and ask for a copy
	if sendOriginal && copyDelegations {
		panic("'Cannot revert' must not expect copy of delegations")
	}

	// Read cache first
	cached, ok := db.stateValidators[addr]
	if ok {
		return copyValidatorWrapperIfNeeded(cached, sendOriginal, copyDelegations), nil
	}

	by := db.GetCode(addr)
	if len(by) == 0 {
		return nil, ErrAddressNotPresent
	}

	val := stk.ValidatorWrapper{}
	if err := rlp.DecodeBytes(by, &val); err != nil {
		return nil, errors.Wrapf(
			err,
			"could not decode for %s",
			common2.MustAddressToBech32(addr),
		)
	}
	// populate cache because the validator is not in it
	db.stateValidators[addr] = &val
	return copyValidatorWrapperIfNeeded(&val, sendOriginal, copyDelegations), nil
}

func copyValidatorWrapperIfNeeded(
	wrapper *stk.ValidatorWrapper,
	sendOriginal bool,
	copyDelegations bool,
) *stk.ValidatorWrapper {
	if sendOriginal {
		return wrapper
	} else {
		if copyDelegations {
			copied := staketest.CopyValidatorWrapper(*wrapper)
			return &copied
		} else {
			copied := staketest.CopyValidatorWrapperNoDelegations(*wrapper)
			return &copied
		}
	}
}

// UpdateValidatorWrapper updates staking information of
// a given validator (including delegation info)
func (db *DB) UpdateValidatorWrapper(
	addr common.Address, val *stk.ValidatorWrapper,
) error {
	if err := val.SanityCheck(); err != nil {
		return err
	}
	by, err := rlp.EncodeToBytes(val)
	if err != nil {
		return err
	}
	// has revert in-built for the code field
	db.SetCode(addr, by, true)
	// update cache
	db.stateValidators[addr] = val
	return nil
}

// UpdateValidatorWrapperWithRevert updates staking information of
// a given validator (including delegation info)
func (db *DB) UpdateValidatorWrapperWithRevert(
	addr common.Address, val *stk.ValidatorWrapper,
) error {
	if err := val.SanityCheck(); err != nil {
		return err
	}
	// a copy of the existing store can be used for revert
	// since we are replacing the existing with the new anyway
	prev, err := db.ValidatorWrapper(addr, true, false)
	if err != nil && err != ErrAddressNotPresent {
		return err
	}
	if err := db.UpdateValidatorWrapper(addr, val); err != nil {
		return err
	}
	// this will save the ValidatorWrapper as prev in validatorWrapperChange object
	db.journal.append(validatorWrapperChange{
		address: &addr,
		prev:    prev,
	})
	return nil
}

// SetValidatorFirstElectionEpoch sets the epoch when the validator is first elected
func (db *DB) SetValidatorFirstElectionEpoch(addr common.Address, epoch *big.Int) {
	firstEpoch := db.GetValidatorFirstElectionEpoch(addr)
	if firstEpoch.Uint64() == 0 {
		// Set only when it's not set (or it's 0)
		bytes := common.BigToHash(epoch)
		db.SetState(addr, staking.FirstElectionEpochKey, bytes)
	}
}

// GetValidatorFirstElectionEpoch gets the epoch when the validator was first elected
func (db *DB) GetValidatorFirstElectionEpoch(addr common.Address) *big.Int {
	so := db.getStateObject(addr)
	value := so.GetState(db.db, staking.FirstElectionEpochKey)
	return value.Big()
}

// SetValidatorFlag checks whether it is a validator object
func (db *DB) SetValidatorFlag(addr common.Address) {
	db.SetState(addr, staking.IsValidatorKey, staking.IsValidator)
}

// UnsetValidatorFlag checks whether it is a validator object
func (db *DB) UnsetValidatorFlag(addr common.Address) {
	db.SetState(addr, staking.IsValidatorKey, common.Hash{})
}

// IsValidator checks whether it is a validator object
func (db *DB) IsValidator(addr common.Address) bool {
	so := db.getStateObject(addr)
	if so == nil {
		return false
	}
	return so.IsValidator(db.db)
}

var (
	zero = numeric.ZeroDec()
)

// AddReward distributes the reward to all the delegators based on stake percentage.
func (db *DB) AddReward(
	snapshot *stk.ValidatorWrapper,
	reward *big.Int,
	shareLookup map[common.Address]numeric.Dec,
) error {
	if reward.Cmp(common.Big0) == 0 {
		utils.Logger().Info().RawJSON("validator", []byte(snapshot.String())).
			Msg("0 given as reward")
		return nil
	}

	curValidator, err := db.ValidatorWrapper(snapshot.Address, true, false)
	if err != nil {
		return errors.Wrapf(err, "failed to distribute rewards: validator does not exist")
	}

	if curValidator.Status == effective.Banned {
		utils.Logger().Info().
			RawJSON("slashed-validator", []byte(curValidator.String())).
			Msg("cannot add reward to banned validator")
		return nil
	}

	rewardPool := big.NewInt(0).Set(reward)
	curValidator.BlockReward.Add(curValidator.BlockReward, reward)
	// Payout commission
	if r := snapshot.Validator.CommissionRates.Rate; r.GT(zero) {
		commissionInt := r.MulInt(reward).RoundInt()
		curValidator.Delegations[0].Reward.Add(
			curValidator.Delegations[0].Reward,
			commissionInt,
		)
		rewardPool.Sub(rewardPool, commissionInt)
	}

	// Payout each delegator's reward pro-rata
	totalRewardForDelegators := big.NewInt(0).Set(rewardPool)
	for i := range snapshot.Delegations {
		delegation := snapshot.Delegations[i]
		percentage, ok := shareLookup[delegation.DelegatorAddress]

		if !ok {
			return errors.Wrapf(err, "missing delegation shares for reward distribution")
		}

		rewardInt := percentage.MulInt(totalRewardForDelegators).RoundInt()
		curDelegation := curValidator.Delegations[i]
		curDelegation.Reward.Add(curDelegation.Reward, rewardInt)
		rewardPool.Sub(rewardPool, rewardInt)
	}

	// The last remaining bit belongs to the validator (remember the validator's self delegation is
	// always at index 0)
	if rewardPool.Cmp(common.Big0) > 0 {
		curValidator.Delegations[0].Reward.Add(curValidator.Delegations[0].Reward, rewardPool)
	}

	return nil
}
