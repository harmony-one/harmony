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

	"github.com/ethereum/go-ethereum/metrics"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core/types"
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

// DB within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type DB struct {
	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects        map[common.Address]*Object
	stateObjectsPending map[common.Address]struct{} // State objects finalized but not yet written to the trie
	stateObjectsDirty   map[common.Address]struct{}
	stateValidators     map[common.Address]*stk.ValidatorWrapper

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by DB.Commit.
	dbErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	thash, bhash common.Hash
	txIndex      int
	logs         map[common.Hash][]*types.Log
	logSize      uint

	preimages map[common.Hash][]byte

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionID int

	// Measurements gathered during execution for debugging purposes
	AccountReads   time.Duration
	AccountHashes  time.Duration
	AccountUpdates time.Duration
	AccountCommits time.Duration
	StorageReads   time.Duration
	StorageHashes  time.Duration
	StorageUpdates time.Duration
	StorageCommits time.Duration
}

// New creates a new state from a given trie.
func New(root common.Hash, db Database) (*DB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &DB{
		db:                  db,
		trie:                tr,
		stateObjects:        make(map[common.Address]*Object),
		stateObjectsPending: make(map[common.Address]struct{}),
		stateObjectsDirty:   make(map[common.Address]struct{}),
		stateValidators:     make(map[common.Address]*stk.ValidatorWrapper),
		logs:                make(map[common.Hash][]*types.Log),
		preimages:           make(map[common.Hash][]byte),
		journal:             newJournal(),
	}, nil
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
	db.txIndex = 0
	db.logs = make(map[common.Hash][]*types.Log)
	db.logSize = 0
	db.preimages = make(map[common.Hash][]byte)
	db.clearJournalAndRefund()
	return nil
}

// AddLog ...
func (db *DB) AddLog(log *types.Log) {
	db.journal.append(addLogChange{txhash: db.thash})

	log.TxHash = db.thash
	log.BlockHash = db.bhash
	log.TxIndex = uint(db.txIndex)
	log.Index = db.logSize
	db.logs[db.thash] = append(db.logs[db.thash], log)
	db.logSize++
}

// GetLogs ...
func (db *DB) GetLogs(hash common.Hash) []*types.Log {
	return db.logs[hash]
}

// Logs ...
func (db *DB) Logs() []*types.Log {
	var logs []*types.Log
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
		panic("Refund counter below zero")
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
	stateObject := db.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return common.Big0
}

// GetNonce ...
func (db *DB) GetNonce(addr common.Address) uint64 {
	stateObject := db.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}

	return 0
}

// TxIndex returns the current transaction index set by Prepare.
func (db *DB) TxIndex() int {
	return db.txIndex
}

func (s *DB) TxHash() common.Hash {
	return s.thash
}

// BlockHash returns the current block hash set by Prepare.
func (db *DB) BlockHash() common.Hash {
	return db.bhash
}

// GetCode ...
func (db *DB) GetCode(addr common.Address) []byte {
	stateObject := db.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code(db.db)
	}
	return nil
}

// GetCodeSize ...
func (db *DB) GetCodeSize(addr common.Address) int {
	stateObject := db.getStateObject(addr)
	if stateObject == nil {
		return 0
	}
	if stateObject.code != nil {
		return len(stateObject.code)
	}
	size, err := db.db.ContractCodeSize(
		stateObject.addrHash, common.BytesToHash(stateObject.CodeHash()),
	)
	if err != nil {
		db.setError(err)
	}
	return size
}

// GetCodeHash ...
func (db *DB) GetCodeHash(addr common.Address) common.Hash {
	stateObject := db.getStateObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return common.BytesToHash(stateObject.CodeHash())
}

// GetState retrieves a value from the given account's storage trie.
func (db *DB) GetState(addr common.Address, hash common.Hash) common.Hash {
	stateObject := db.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetState(db.db, hash)
	}
	return common.Hash{}
}

// GetProof returns the MerkleProof for a given Account
func (db *DB) GetProof(a common.Address) ([][]byte, error) {
	var proof proofList
	err := db.trie.Prove(crypto.Keccak256(a.Bytes()), 0, &proof)
	return [][]byte(proof), err
}

// GetStorageProof returns the StorageProof for given key
func (db *DB) GetStorageProof(a common.Address, key common.Hash) ([][]byte, error) {
	var proof proofList
	trie := db.StorageTrie(a)
	if trie == nil {
		return proof, errors.New("storage trie for requested address does not exist")
	}
	err := trie.Prove(crypto.Keccak256(key.Bytes()), 0, &proof)
	return [][]byte(proof), err
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (db *DB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	stateObject := db.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetCommittedState(db.db, hash)
	}
	return common.Hash{}
}

// Database retrieves the low level database supporting the lower level trie ops.
func (db *DB) Database() Database {
	return db.db
}

// StorageTrie returns the storage trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (db *DB) StorageTrie(addr common.Address) Trie {
	stateObject := db.getStateObject(addr)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(db)
	return cpy.updateTrie(db.db)
}

// HasSuicided ...
func (db *DB) HasSuicided(addr common.Address) bool {
	stateObject := db.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr.
func (db *DB) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := db.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

// SubBalance subtracts amount from the account associated with addr.
func (db *DB) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := db.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

// SetBalance ...
func (db *DB) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := db.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

// SetNonce ...
func (db *DB) SetNonce(addr common.Address, nonce uint64) {
	stateObject := db.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

// SetCode ...
func (db *DB) SetCode(addr common.Address, code []byte) {
	stateObject := db.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

// SetState ...
func (db *DB) SetState(addr common.Address, key, value common.Hash) {
	stateObject := db.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(db.db, key, value)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (db *DB) Suicide(addr common.Address) bool {
	stateObject := db.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	db.journal.append(suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: new(big.Int).Set(stateObject.Balance()),
	})
	stateObject.markSuicided()
	stateObject.data.Balance = new(big.Int)

	return true
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

	data, err := rlp.EncodeToBytes(obj)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	db.setError(db.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (db *DB) deleteStateObject(obj *Object) {
	// Track the amount of time wasted on deleting the account from the trie
	if metrics.EnabledExpensive {
		defer func(start time.Time) { db.AccountUpdates += time.Since(start) }(time.Now())
	}
	// Delete the account from the trie
	addr := obj.Address()
	db.setError(db.trie.TryDelete(addr[:]))
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
	// Track the amount of time wasted on loading the object from the database
	if metrics.EnabledExpensive {
		defer func(start time.Time) { db.AccountReads += time.Since(start) }(time.Now())
	}
	// Load the object from the database
	enc, err := db.trie.TryGet(addr[:])
	if len(enc) == 0 {
		db.setError(err)
		return nil
	}
	var data Account
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		log.Error("Failed to decode state object", "addr", addr, "err", err)
		return nil
	}
	// Insert into the live set
	obj := newObject(db, addr, data)
	db.setStateObject(obj)
	return obj
}

func (db *DB) setStateObject(object *Object) {
	db.stateObjects[object.Address()] = object
}

// GetOrNewStateObject retrieves a state object or create a new state object if nil.
func (db *DB) GetOrNewStateObject(addr common.Address) *Object {
	stateObject := db.getStateObject(addr)
	if stateObject == nil {
		stateObject, _ = db.createObject(addr)
	}
	return stateObject
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (db *DB) createObject(addr common.Address) (newobj, prev *Object) {
	prev = db.getDeletedStateObject(addr) // Note, prev might have been deleted, we need that!

	newobj = newObject(db, addr, Account{})
	newobj.setNonce(0) // sets the object to dirty
	if prev == nil {
		db.journal.append(createObjectChange{account: &addr})
	} else {
		db.journal.append(resetObjectChange{prev: prev})
	}
	db.setStateObject(newobj)
	return newobj, prev
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a contract does the following:
//
//   1. sends funds to sha(account ++ (nonce + 1))
//   2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that Ether doesn't disappear.
func (db *DB) CreateAccount(addr common.Address) {
	newObj, prev := db.createObject(addr)
	if prev != nil {
		newObj.setBalance(prev.data.Balance)
	}
}

// ForEachStorage ...
func (db *DB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) error {
	so := db.getStateObject(addr)
	if so == nil {
		return nil
	}
	it := trie.NewIterator(so.getTrie(db.db).NodeIterator(nil))

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
		db:                  db.db,
		trie:                db.db.CopyTrie(db.trie),
		stateObjects:        make(map[common.Address]*Object, len(db.journal.dirties)),
		stateObjectsPending: make(map[common.Address]struct{}, len(db.stateObjectsPending)),
		stateObjectsDirty:   make(map[common.Address]struct{}, len(db.journal.dirties)),
		stateValidators:     make(map[common.Address]*stk.ValidatorWrapper),
		refund:              db.refund,
		logs:                make(map[common.Hash][]*types.Log, len(db.logs)),
		logSize:             db.logSize,
		preimages:           make(map[common.Hash][]byte),
		journal:             newJournal(),
	}
	// Copy the dirty states, logs, and preimages
	for addr := range db.journal.dirties {
		// As documented [here](https://github.com/ethereum/go-ethereum/pull/16485#issuecomment-380438527),
		// and in the Finalise-method, there is a case where an object is in the journal but not
		// in the stateObjects: OOG after touch on ripeMD prior to Byzantium. Thus, we need to check for
		// nil
		if object, exist := db.stateObjects[addr]; exist {
			// Even though the original object is dirty, we are not copying the journal,
			// so we need to make sure that anyside effect the journal would have caused
			// during a commit (or similar op) is already applied to the copy.
			state.stateObjects[addr] = object.deepCopy(state)

			state.stateObjectsDirty[addr] = struct{}{}   // Mark the copy dirty to force internal (code/state) commits
			state.stateObjectsPending[addr] = struct{}{} // Mark the copy pending to force external (account) commits
		}
	}
	// Above, we don't copy the actual journal. This means that if the copy is copied, the
	// loop above will be a no-op, since the copy's journal is empty.
	// Thus, here we iterate over stateObjects, to enable copies of copies
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
	for addr, wrapper := range db.stateValidators {
		copied := staketest.CopyValidatorWrapper(*wrapper)
		state.stateValidators[addr] = &copied
	}
	for hash, logs := range db.logs {
		cpy := make([]*types.Log, len(logs))
		for i, l := range logs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		state.logs[hash] = cpy
	}
	for hash, preimage := range db.preimages {
		state.preimages[hash] = preimage
	}
	return state
}

// Snapshot returns an identifier for the current revision of the state.
func (db *DB) Snapshot() int {
	id := db.nextRevisionID
	db.nextRevisionID++
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

// Finalise finalises the state by removing the db destructed objects
// and clears the journal as well as the refunds.
func (db *DB) Finalise(deleteEmptyObjects bool) {
	// Commit validator changes in cache to stateObjects
	// TODO: remove validator cache after commit
	for addr, wrapper := range db.stateValidators {
		db.UpdateValidatorWrapper(addr, wrapper)
	}

	for addr := range db.journal.dirties {
		obj, exist := db.stateObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `s.journal.dirties` but not in `s.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}
		if obj.suicided || (deleteEmptyObjects && obj.empty()) {
			obj.deleted = true
		} else {
			obj.finalise()
		}
		db.stateObjectsPending[addr] = struct{}{}
		db.stateObjectsDirty[addr] = struct{}{}
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

	for addr := range db.stateObjectsPending {
		obj := db.stateObjects[addr]
		if obj.deleted {
			db.deleteStateObject(obj)
		} else {
			obj.updateRoot(db.db)
			db.updateStateObject(obj)
		}
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

func (db *DB) clearJournalAndRefund() {
	if len(db.journal.entries) > 0 {
		db.journal = newJournal()
		db.refund = 0
	}
	db.validRevisions = db.validRevisions[:0] // Snapshots can be created without journal entires
}

// Commit writes the state to the underlying in-memory trie database.
func (db *DB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	// Finalize any pending changes and merge everything into the tries
	db.IntermediateRoot(deleteEmptyObjects)

	// Commit objects to the trie, measuring the elapsed time
	for addr := range db.stateObjectsDirty {
		if obj := db.stateObjects[addr]; !obj.deleted {
			// Write any contract code associated with the state object
			if obj.code != nil && obj.dirtyCode {
				db.db.TrieDB().InsertBlob(common.BytesToHash(obj.CodeHash()), obj.code)
				obj.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie
			if err := obj.CommitTrie(db.db); err != nil {
				return common.Hash{}, err
			}
		}
	}
	if len(db.stateObjectsDirty) > 0 {
		db.stateObjectsDirty = make(map[common.Address]struct{})
	}
	// Write the account trie changes, measuing the amount of wasted time
	if metrics.EnabledExpensive {
		defer func(start time.Time) { db.AccountCommits += time.Since(start) }(time.Now())
	}
	return db.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		if account.Root != emptyRoot {
			db.db.TrieDB().Reference(account.Root, parent)
		}
		code := common.BytesToHash(account.CodeHash)
		if code != emptyCode {
			db.db.TrieDB().Reference(code, parent)
		}
		return nil
	})
}

var (
	errAddressNotPresent = errors.New("address not present in state")
)

// ValidatorWrapper retrieves the existing validator in the cache, if sendOriginal
// else it will return a copy of the wrapper - which needs to be explicitly committed
// with UpdateValidatorWrapper
// to conserve memory, the copy can optionally avoid deep copying delegations
// Revert in UpdateValidatorWrapper does not work if sendOriginal == true
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
		return nil, errAddressNotPresent
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
	db.SetCode(addr, by)
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
	if err != nil && err != errAddressNotPresent {
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
func (db *DB) AddReward(snapshot *stk.ValidatorWrapper, reward *big.Int, shareLookup map[common.Address]numeric.Dec) error {
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
