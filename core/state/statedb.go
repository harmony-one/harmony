// package state ...
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
	"github.com/harmony-one/harmony/core/state"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core/types"
)

type revision struct {
	id           int
	journalIndex int
}

var (
	// emptyState is the known hash of an empty state trie entry.
	emptyState = crypto.Keccak256Hash(nil)

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

type proofList [][]byte

func (n *proofList) Put(key []byte, value []byte) error {
	*n = append(*n, value)
	return nil
}

// StateDB within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type StateDB struct {
	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[common.Address]*stateObject
	stateObjectsDirty map[common.Address]struct{}

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
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

	lock sync.Mutex
}

// New creates a new state from a given trie.
func New(root common.Hash, db Database) (*StateDB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		db:                db,
		trie:              tr,
		stateObjects:      make(map[common.Address]*stateObject),
		stateObjectsDirty: make(map[common.Address]struct{}),
		logs:              make(map[common.Hash][]*types.Log),
		preimages:         make(map[common.Hash][]byte),
		journal:           newJournal(),
	}, nil
}

// setError remembers the first non-nil error it is called with.
func (stateDB *StateDB) setError(err error) {
	if stateDB.dbErr == nil {
		stateDB.dbErr = err
	}
}

func (stateDB *StateDB) Error() error {
	return stateDB.dbErr
}

// Reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (stateDB *StateDB) Reset(root common.Hash) error {
	tr, err := stateDB.db.OpenTrie(root)
	if err != nil {
		return err
	}
	stateDB.trie = tr
	stateDB.stateObjects = make(map[common.Address]*stateObject)
	stateDB.stateObjectsDirty = make(map[common.Address]struct{})
	stateDB.thash = common.Hash{}
	stateDB.bhash = common.Hash{}
	stateDB.txIndex = 0
	stateDB.logs = make(map[common.Hash][]*types.Log)
	stateDB.logSize = 0
	stateDB.preimages = make(map[common.Hash][]byte)
	stateDB.clearJournalAndRefund()
	return nil
}

// AddLog adds logs into stateDB
func (stateDB *StateDB) AddLog(log *types.Log) {
	stateDB.journal.append(addLogChange{txhash: stateDB.thash})

	log.TxHash = stateDB.thash
	log.BlockHash = stateDB.bhash
	log.TxIndex = uint(stateDB.txIndex)
	log.Index = stateDB.logSize
	stateDB.logs[stateDB.thash] = append(stateDB.logs[stateDB.thash], log)
	stateDB.logSize++
}

// GetLogs gets logs from stateDB given a hash
func (stateDB *StateDB) GetLogs(hash common.Hash) []*types.Log {
	return stateDB.logs[hash]
}

// Logs returns a list of Log.
func (stateDB *StateDB) Logs() []*types.Log {
	var logs []*types.Log
	for _, lgs := range stateDB.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// AddPreimage records a SHA3 preimage seen by the VM.
func (stateDB *StateDB) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := stateDB.preimages[hash]; !ok {
		stateDB.journal.append(addPreimageChange{hash: hash})
		pi := make([]byte, len(preimage))
		copy(pi, preimage)
		stateDB.preimages[hash] = pi
	}
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (stateDB *StateDB) Preimages() map[common.Hash][]byte {
	return stateDB.preimages
}

// AddRefund adds gas to the refund counter
func (stateDB *StateDB) AddRefund(gas uint64) {
	stateDB.journal.append(refundChange{prev: stateDB.refund})
	stateDB.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (stateDB *StateDB) SubRefund(gas uint64) {
	stateDB.journal.append(refundChange{prev: stateDB.refund})
	if gas > stateDB.refund {
		panic("Refund counter below zero")
	}
	stateDB.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (stateDB *StateDB) Exist(addr common.Address) bool {
	return stateDB.getStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (stateDB *StateDB) Empty(addr common.Address) bool {
	so := stateDB.getStateObject(addr)
	return so == nil || so.empty()
}

// GetBalance retrieves the balance from the given address or 0 if object not found
func (stateDB *StateDB) GetBalance(addr common.Address) *big.Int {
	stateObject := stateDB.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return common.Big0
}

// GetNonce returns the nonce of the given address.
func (stateDB *StateDB) GetNonce(addr common.Address) uint64 {
	stateObject := stateDB.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}

	return 0
}

// GetCode returns code of a given address.
func (stateDB *StateDB) GetCode(addr common.Address) []byte {
	stateObject := stateDB.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code(stateDB.db)
	}
	return nil
}

// GetCodeSize returns code size of a given address in stateDB.
func (stateDB *StateDB) GetCodeSize(addr common.Address) int {
	stateObject := stateDB.getStateObject(addr)
	if stateObject == nil {
		return 0
	}
	if stateObject.code != nil {
		return len(stateObject.code)
	}
	size, err := stateDB.db.ContractCodeSize(stateObject.addrHash, common.BytesToHash(stateObject.CodeHash()))
	if err != nil {
		stateDB.setError(err)
	}
	return size
}

// GetCodeHash returns code hash of a given address.
func (stateDB *StateDB) GetCodeHash(addr common.Address) common.Hash {
	stateObject := stateDB.getStateObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return common.BytesToHash(stateObject.CodeHash())
}

// GetState retrieves a value from the given account's storage trie.
func (stateDB *StateDB) GetState(addr common.Address, hash common.Hash) common.Hash {
	stateObject := stateDB.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetState(stateDB.db, hash)
	}
	return common.Hash{}
}

// GetProof returns the MerkleProof for a given Account
func (stateDB *StateDB) GetProof(a common.Address) ([][]byte, error) {
	var proof proofList
	err := stateDB.trie.Prove(crypto.Keccak256(a.Bytes()), 0, &proof)
	return [][]byte(proof), err
}

// GetStorageProof returns the StorageProof for given key
func (stateDB *StateDB) GetStorageProof(a common.Address, key common.Hash) ([][]byte, error) {
	var proof proofList
	trie := stateDB.StorageTrie(a)
	if trie == nil {
		return proof, errors.New("storage trie for requested address does not exist")
	}
	err := trie.Prove(crypto.Keccak256(key.Bytes()), 0, &proof)
	return [][]byte(proof), err
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (stateDB *StateDB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	stateObject := stateDB.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetCommittedState(stateDB.db, hash)
	}
	return common.Hash{}
}

// Database retrieves the low level database supporting the lower level trie ops.
func (stateDB *StateDB) Database() Database {
	return stateDB.db
}

// StorageTrie returns the storage trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (stateDB *StateDB) StorageTrie(addr common.Address) Trie {
	stateObject := stateDB.getStateObject(addr)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(stateDB)
	return cpy.updateTrie(stateDB.db)
}

// HasSuicided checks if the state object of the given addr is suicided.
func (stateDB *StateDB) HasSuicided(addr common.Address) bool {
	stateObject := stateDB.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr.
func (stateDB *StateDB) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := stateDB.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

// SubBalance subtracts amount from the account associated with addr.
func (stateDB *StateDB) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := stateDB.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

// SetBalance sets balance of an address.
func (stateDB *StateDB) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := stateDB.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

// SetNonce sets nonce of a given address.
func (stateDB *StateDB) SetNonce(addr common.Address, nonce uint64) {
	stateObject := stateDB.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

// SetCode sets code of a given address.
func (stateDB *StateDB) SetCode(addr common.Address, code []byte) {
	stateObject := stateDB.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

// SetState sets hash value of a given address.
func (stateDB *StateDB) SetState(addr common.Address, key, value common.Hash) {
	stateObject := stateDB.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(stateDB.db, key, value)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (stateDB *StateDB) Suicide(addr common.Address) bool {
	stateObject := stateDB.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	stateDB.journal.append(suicideChange{
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
func (stateDB *StateDB) updateStateObject(stateObject *stateObject) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	stateDB.setError(stateDB.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (stateDB *StateDB) deleteStateObject(stateObject *stateObject) {
	stateObject.deleted = true
	addr := stateObject.Address()
	stateDB.setError(stateDB.trie.TryDelete(addr[:]))
}

// Retrieve a state object given by the address. Returns nil if not found.
func (stateDB *StateDB) getStateObject(addr common.Address) (stateObject *stateObject) {
	// Prefer 'live' objects.
	if obj := stateDB.stateObjects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
	enc, err := stateDB.trie.TryGet(addr[:])
	if len(enc) == 0 {
		stateDB.setError(err)
		return nil
	}
	var data Account
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		log.Error("Failed to decode state object", "addr", addr, "err", err)
		return nil
	}
	// Insert into the live set.
	obj := newObject(stateDB, addr, data)
	stateDB.setStateObject(obj)
	return obj
}

func (stateDB *StateDB) setStateObject(object *stateObject) {
	stateDB.stateObjects[object.Address()] = object
}

// GetOrNewStateObject retrieves a state object or create a new state object if nil.
func (stateDB *StateDB) GetOrNewStateObject(addr common.Address) *StateObject {
	stateObject := stateDB.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = stateDB.createObject(addr)
	}
	return stateObject
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (stateDB *StateDB) createObject(addr common.Address) (newobj, prev *stateObject) {
	prev = stateDB.getStateObject(addr)
	newobj = newObject(stateDB, addr, Account{})
	newobj.setNonce(0) // sets the object to dirty
	if prev == nil {
		stateDB.journal.append(createObjectChange{account: &addr})
	} else {
		stateDB.journal.append(resetObjectChange{prev: prev})
	}
	stateDB.setStateObject(newobj)
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
func (stateDB *StateDB) CreateAccount(addr common.Address) {
	new, prev := stateDB.createObject(addr)
	if prev != nil {
		new.setBalance(prev.data.Balance)
	}
}

// ForEachStorage runs a function on every item in state DB.
func (stateDB *StateDB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) {
	so := stateDB.getStateObject(addr)
	if so == nil {
		return
	}
	it := trie.NewIterator(so.getTrie(stateDB.db).NodeIterator(nil))
	for it.Next() {
		key := common.BytesToHash(stateDB.trie.GetKey(it.Key))
		if value, dirty := so.dirtyStorage[key]; dirty {
			cb(key, value)
			continue
		}
		cb(key, common.BytesToHash(it.Value))
	}
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (stateDB *StateDB) Copy() *StateDB {
	stateDB.lock.Lock()
	defer stateDB.lock.Unlock()

	// Copy all the basic fields, initialize the memory ones
	state := &StateDB{
		db:                stateDB.db,
		trie:              stateDB.db.CopyTrie(stateDB.trie),
		stateObjects:      make(map[common.Address]*stateObject, len(stateDB.journal.dirties)),
		stateObjectsDirty: make(map[common.Address]struct{}, len(stateDB.journal.dirties)),
		refund:            stateDB.refund,
		logs:              make(map[common.Hash][]*types.Log, len(stateDB.logs)),
		logSize:           stateDB.logSize,
		preimages:         make(map[common.Hash][]byte),
		journal:           newJournal(),
	}
	// Copy the dirty states, logs, and preimages
	for addr := range stateDB.journal.dirties {
		// As documented [here](https://github.com/ethereum/go-ethereum/pull/16485#issuecomment-380438527),
		// and in the Finalise-method, there is a case where an object is in the journal but not
		// in the stateObjects: OOG after touch on ripeMD prior to Byzantium. Thus, we need to check for
		// nil
		if object, exist := stateDB.stateObjects[addr]; exist {
			state.stateObjects[addr] = object.deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}
	// Above, we don't copy the actual journal. This means that if the copy is copied, the
	// loop above will be a no-op, since the copy's journal is empty.
	// Thus, here we iterate over stateObjects, to enable copies of copies
	for addr := range stateDB.stateObjectsDirty {
		if _, exist := state.stateObjects[addr]; !exist {
			state.stateObjects[addr] = stateDB.stateObjects[addr].deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}
	for hash, logs := range stateDB.logs {
		cpy := make([]*types.Log, len(logs))
		for i, l := range logs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		state.logs[hash] = cpy
	}
	for hash, preimage := range stateDB.preimages {
		state.preimages[hash] = preimage
	}
	return state
}

// Snapshot returns an identifier for the current revision of the state.
func (stateDB *StateDB) Snapshot() int {
	id := stateDB.nextRevisionID
	stateDB.nextRevisionID++
	stateDB.validRevisions = append(stateDB.validRevisions, revision{id, stateDB.journal.length()})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (stateDB *StateDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(stateDB.validRevisions), func(i int) bool {
		return stateDB.validRevisions[i].id >= revid
	})
	if idx == len(stateDB.validRevisions) || stateDB.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := stateDB.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	stateDB.journal.revert(stateDB, snapshot)
	stateDB.validRevisions = stateDB.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
func (stateDB *StateDB) GetRefund() uint64 {
	return stateDB.refund
}

// Finalise finalises the state by removing the self destructed objects
// and clears the journal as well as the refunds.
func (stateDB *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr := range stateDB.journal.dirties {
		stateObject, exist := stateDB.stateObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `s.journal.dirties` but not in `s.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}

		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			s.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot(s.db)
			s.updateStateObject(stateObject)
		}
		s.stateObjectsDirty[addr] = struct{}{}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (stateDB *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	s.Finalise(deleteEmptyObjects)
	return s.trie.Hash()
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the EVM emits new state logs.
func (stateDB *StateDB) Prepare(thash, bhash common.Hash, ti int) {
	stateDB.thash = thash
	stateDB.bhash = bhash
	stateDB.txIndex = ti
}

func (stateDB *StateDB) clearJournalAndRefund() {
	s.journal = newJournal()
	s.validRevisions = s.validRevisions[:0]
	s.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (stateDB *StateDB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	defer s.clearJournalAndRefund()

	for addr := range s.journal.dirties {
		s.stateObjectsDirty[addr] = struct{}{}
	}
	// Commit objects to the trie.
	for addr, stateObject := range s.stateObjects {
		_, isDirty := s.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			s.deleteStateObject(stateObject)
		case isDirty:
			// Write any contract code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				s.db.TrieDB().InsertBlob(common.BytesToHash(stateObject.CodeHash()), stateObject.code)
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if err := stateObject.CommitTrie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			s.updateStateObject(stateObject)
		}
		delete(s.stateObjectsDirty, addr)
	}
	// Write trie changes.
	root, err = s.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		if account.Root != emptyState {
			s.db.TrieDB().Reference(account.Root, parent)
		}
		code := common.BytesToHash(account.CodeHash)
		if code != emptyCode {
			s.db.TrieDB().Reference(code, parent)
		}
		return nil
	})
	log.Debug("Trie cache stats after commit", "misses", trie.CacheMisses(), "unloads", trie.CacheUnloads())
	return root, err
}
