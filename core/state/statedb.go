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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking"
	stk "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
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

// DB within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type DB struct {
	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[common.Address]*Object
	stateObjectsDirty map[common.Address]struct{}

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
}

// New creates a new state from a given trie.
func New(root common.Hash, db Database) (*DB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &DB{
		db:                db,
		trie:              tr,
		stateObjects:      make(map[common.Address]*Object),
		stateObjectsDirty: make(map[common.Address]struct{}),
		logs:              make(map[common.Hash][]*types.Log),
		preimages:         make(map[common.Hash][]byte),
		journal:           newJournal(),
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
	db.stateObjectsDirty = make(map[common.Address]struct{})
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
func (db *DB) updateStateObject(stateObject *Object) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	db.setError(db.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (db *DB) deleteStateObject(stateObject *Object) {
	stateObject.deleted = true
	addr := stateObject.Address()
	db.setError(db.trie.TryDelete(addr[:]))
}

// Retrieve a state object given by the address. Returns nil if not found.
func (db *DB) getStateObject(addr common.Address) (stateObject *Object) {
	// Prefer 'live' objects.
	if obj := db.stateObjects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
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
	// Insert into the live set.
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
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = db.createObject(addr)
	}
	return stateObject
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (db *DB) createObject(addr common.Address) (newobj, prev *Object) {
	prev = db.getStateObject(addr)
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
func (db *DB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) {
	so := db.getStateObject(addr)
	if so == nil {
		return
	}
	it := trie.NewIterator(so.getTrie(db.db).NodeIterator(nil))
	for it.Next() {
		key := common.BytesToHash(db.trie.GetKey(it.Key))
		if value, dirty := so.dirtyStorage[key]; dirty {
			cb(key, value)
			continue
		}
		cb(key, common.BytesToHash(it.Value))
	}
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (db *DB) Copy() *DB {
	// Copy all the basic fields, initialize the memory ones
	state := &DB{
		db:                db.db,
		trie:              db.db.CopyTrie(db.trie),
		stateObjects:      make(map[common.Address]*Object, len(db.journal.dirties)),
		stateObjectsDirty: make(map[common.Address]struct{}, len(db.journal.dirties)),
		refund:            db.refund,
		logs:              make(map[common.Hash][]*types.Log, len(db.logs)),
		logSize:           db.logSize,
		preimages:         make(map[common.Hash][]byte),
		journal:           newJournal(),
	}
	// Copy the dirty states, logs, and preimages
	for addr := range db.journal.dirties {
		// As documented [here](https://github.com/ethereum/go-ethereum/pull/16485#issuecomment-380438527),
		// and in the Finalise-method, there is a case where an object is in the journal but not
		// in the stateObjects: OOG after touch on ripeMD prior to Byzantium. Thus, we need to check for
		// nil
		if object, exist := db.stateObjects[addr]; exist {
			state.stateObjects[addr] = object.deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}
	// Above, we don't copy the actual journal. This means that if the copy is copied, the
	// loop above will be a no-op, since the copy's journal is empty.
	// Thus, here we iterate over stateObjects, to enable copies of copies
	for addr := range db.stateObjectsDirty {
		if _, exist := state.stateObjects[addr]; !exist {
			state.stateObjects[addr] = db.stateObjects[addr].deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
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
	for addr := range db.journal.dirties {
		stateObject, exist := db.stateObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `db.journal.dirties` but not in `db.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}

		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			db.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot(db.db)
			db.updateStateObject(stateObject)
		}
		db.stateObjectsDirty[addr] = struct{}{}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	db.clearJournalAndRefund()
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (db *DB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	db.Finalise(deleteEmptyObjects)
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
	db.journal = newJournal()
	db.validRevisions = db.validRevisions[:0]
	db.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (db *DB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	defer db.clearJournalAndRefund()

	for addr := range db.journal.dirties {
		db.stateObjectsDirty[addr] = struct{}{}
	}
	// Commit objects to the trie.
	for addr, stateObject := range db.stateObjects {
		_, isDirty := db.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			db.deleteStateObject(stateObject)
		case isDirty:
			// Write any contract code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				db.db.TrieDB().InsertBlob(common.BytesToHash(stateObject.CodeHash()), stateObject.code)
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if err := stateObject.CommitTrie(db.db); err != nil {
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			db.updateStateObject(stateObject)
		}
		delete(db.stateObjectsDirty, addr)
	}
	// Write trie changes.
	root, err = db.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		if account.Root != emptyState {
			db.db.TrieDB().Reference(account.Root, parent)
		}
		code := common.BytesToHash(account.CodeHash)
		if code != emptyCode {
			db.db.TrieDB().Reference(code, parent)
		}
		return nil
	})
	//log.Debug("Trie cache stats after commit", "misses", trie.CacheMisses(), "unloads", trie.CacheUnloads())
	return root, err
}

var (
	errAddressNotPresent = errors.New("address not present in state")
)

// ValidatorWrapper  ..
func (db *DB) ValidatorWrapper(
	addr common.Address,
) (*stk.ValidatorWrapper, error) {
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
	return &val, nil
}

const doNotEnforceMaxBLS = -1

// UpdateValidatorWrapper updates staking information of
// a given validator (including delegation info)
func (db *DB) UpdateValidatorWrapper(
	addr common.Address, val *stk.ValidatorWrapper,
) error {
	if err := val.SanityCheck(doNotEnforceMaxBLS); err != nil {
		return err
	}

	by, err := rlp.EncodeToBytes(val)
	if err != nil {
		return err
	}
	db.SetCode(addr, by)
	return nil
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

// AddReward distributes the reward to all the delegators based on stake percentage.
func (db *DB) AddReward(snapshot *stk.ValidatorWrapper, reward *big.Int) error {
	rewardPool := big.NewInt(0).Set(reward)
	curValidator, err := db.ValidatorWrapper(snapshot.Address)
	if err != nil {
		return errors.Wrapf(err, "failed to distribute rewards: validator does not exist")
	}

	curValidator.BlockReward.Add(curValidator.BlockReward, reward)
	// Payout commission
	commissionInt := snapshot.Validator.CommissionRates.Rate.MulInt(reward).RoundInt()
	curValidator.Delegations[0].Reward.Add(curValidator.Delegations[0].Reward, commissionInt)
	rewardPool.Sub(rewardPool, commissionInt)
	totalRewardForDelegators := big.NewInt(0).Set(rewardPool)
	// Payout each delegator's reward pro-rata
	totalDelegationDec := numeric.NewDecFromBigInt(snapshot.TotalDelegation())
	for i := range snapshot.Delegations {
		delegation := snapshot.Delegations[i]
		// NOTE percentage = <this_delegator_amount>/<total_delegation>
		percentage := numeric.NewDecFromBigInt(delegation.Amount).Quo(totalDelegationDec)
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

	return db.UpdateValidatorWrapper(curValidator.Validator.Address, curValidator)
}
