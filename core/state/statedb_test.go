// Copyright 2016 The go-ethereum Authors
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

package state

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/quick"

	"github.com/ethereum/go-ethereum/core/rawdb"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/numeric"
	stk "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/common/denominations"
)

// Tests that updating a state trie does not leak any database writes prior to
// actually committing the state.
func TestUpdateLeaks(t *testing.T) {
	// Create an empty state database
	db := rawdb.NewMemoryDatabase()
	state, _ := New(common.Hash{}, NewDatabase(db))

	// Update it with some accounts
	for i := byte(0); i < 255; i++ {
		addr := common.BytesToAddress([]byte{i})
		state.AddBalance(addr, big.NewInt(int64(11*i)))
		state.SetNonce(addr, uint64(42*i))
		if i%2 == 0 {
			state.SetState(addr, common.BytesToHash([]byte{i, i, i}), common.BytesToHash([]byte{i, i, i, i}))
		}
		if i%3 == 0 {
			state.SetCode(addr, []byte{i, i, i, i, i})
		}
	}

	root := state.IntermediateRoot(false)
	if err := state.Database().TrieDB().Commit(root, false); err != nil {
		t.Errorf("can not commit trie %v to persistent database", root.Hex())
	}

	// Ensure that no data was leaked into the database
	it := db.NewIterator()
	for it.Next() {
		t.Errorf("State leaked into database: %x -> %x", it.Key(), it.Value())
	}
	it.Release()
}

// Tests that no intermediate state of an object is stored into the database,
// only the one right before the commit.
func TestIntermediateLeaks(t *testing.T) {
	// Create two state databases, one transitioning to the final state, the other final from the beginning
	transDb := rawdb.NewMemoryDatabase()
	finalDb := rawdb.NewMemoryDatabase()
	transState, _ := New(common.Hash{}, NewDatabase(transDb))
	finalState, _ := New(common.Hash{}, NewDatabase(finalDb))

	modify := func(state *DB, addr common.Address, i, tweak byte) {
		state.SetBalance(addr, big.NewInt(int64(11*i)+int64(tweak)))
		state.SetNonce(addr, uint64(42*i+tweak))
		if i%2 == 0 {
			state.SetState(addr, common.Hash{i, i, i, 0}, common.Hash{})
			state.SetState(addr, common.Hash{i, i, i, tweak}, common.Hash{i, i, i, i, tweak})
		}
		if i%3 == 0 {
			state.SetCode(addr, []byte{i, i, i, i, i, tweak})
		}
	}

	// Modify the transient state.
	for i := byte(0); i < 255; i++ {
		modify(transState, common.Address{i}, i, 0)
	}
	// Write modifications to trie.
	transState.IntermediateRoot(false)

	// Overwrite all the data with new values in the transient database.
	for i := byte(0); i < 255; i++ {
		modify(transState, common.Address{i}, i, 99)
		modify(finalState, common.Address{i}, i, 99)
	}

	// Commit and cross check the databases.
	transRoot, err := transState.Commit(false)
	if err != nil {
		t.Fatalf("failed to commit transition state: %v", err)
	}
	if err = transState.Database().TrieDB().Commit(transRoot, false); err != nil {
		t.Errorf("can not commit trie %v to persistent database", transRoot.Hex())
	}

	finalRoot, err := finalState.Commit(false)
	if err != nil {
		t.Fatalf("failed to commit final state: %v", err)
	}
	if err = finalState.Database().TrieDB().Commit(finalRoot, false); err != nil {
		t.Errorf("can not commit trie %v to persistent database", finalRoot.Hex())
	}

	it := finalDb.NewIterator()
	for it.Next() {
		key, fvalue := it.Key(), it.Value()
		tvalue, err := transDb.Get(key)
		if err != nil {
			t.Errorf("entry missing from the transition database: %x -> %x", key, fvalue)
		}
		if !bytes.Equal(fvalue, tvalue) {
			t.Errorf("the value associate key %x is mismatch,: %x in transition database ,%x in final database", key, tvalue, fvalue)
		}
	}
	it.Release()

	it = transDb.NewIterator()
	for it.Next() {
		key, tvalue := it.Key(), it.Value()
		fvalue, err := finalDb.Get(key)
		if err != nil {
			t.Errorf("extra entry in the transition database: %x -> %x", key, it.Value())
		}
		if !bytes.Equal(fvalue, tvalue) {
			t.Errorf("the value associate key %x is mismatch,: %x in transition database ,%x in final database", key, tvalue, fvalue)
		}
	}
}

// TestCopy tests that copying a statedb object indeed makes the original and
// the copy independent of each other. This test is a regression test against
// https://github.com/ethereum/go-ethereum/pull/15549.
func TestCopy(t *testing.T) {
	// Create a random state test to copy and modify "independently"
	orig, _ := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))

	for i := byte(0); i < 255; i++ {
		obj := orig.GetOrNewStateObject(common.BytesToAddress([]byte{i}))
		obj.AddBalance(big.NewInt(int64(i)))
		orig.updateStateObject(obj)

		validatorWrapper := makeValidValidatorWrapper(common.BytesToAddress([]byte{i}))
		validatorWrapper.Description.Name = "Original"
		err := orig.UpdateValidatorWrapper(common.BytesToAddress([]byte{i}), &validatorWrapper)
		if err != nil {
			t.Errorf("Couldn't update ValidatorWrapper %d with error %s", i, err)
		}
	}
	orig.Finalise(false)

	// Copy the state
	copy := orig.Copy()

	// Copy the copy state
	ccopy := copy.Copy()

	// modify all in memory
	for i := byte(0); i < 255; i++ {
		origObj := orig.GetOrNewStateObject(common.BytesToAddress([]byte{i}))
		copyObj := copy.GetOrNewStateObject(common.BytesToAddress([]byte{i}))
		ccopyObj := ccopy.GetOrNewStateObject(common.BytesToAddress([]byte{i}))

		origObj.AddBalance(big.NewInt(2 * int64(i)))
		copyObj.AddBalance(big.NewInt(3 * int64(i)))
		ccopyObj.AddBalance(big.NewInt(4 * int64(i)))

		orig.updateStateObject(origObj)
		copy.updateStateObject(copyObj)
		ccopy.updateStateObject(copyObj)

		origValWrap, err := orig.ValidatorWrapper(common.BytesToAddress([]byte{i}), false, true)
		if err != nil {
			t.Errorf("Couldn't get validatorWrapper with error: %s", err)
		}
		copyValWrap, err := copy.ValidatorWrapper(common.BytesToAddress([]byte{i}), false, true)
		if err != nil {
			t.Errorf("Couldn't get validatorWrapper with error: %s", err)
		}
		ccopyValWrap, err := ccopy.ValidatorWrapper(common.BytesToAddress([]byte{i}), false, true)
		if err != nil {
			t.Errorf("Couldn't get validatorWrapper with error: %s", err)
		}

		origValWrap.LastEpochInCommittee.SetInt64(1)
		copyValWrap.LastEpochInCommittee.SetInt64(2)
		ccopyValWrap.LastEpochInCommittee.SetInt64(3)

		origValWrap.MinSelfDelegation.Mul(big.NewInt(1e18), big.NewInt(10000))
		copyValWrap.MinSelfDelegation.Mul(big.NewInt(1e18), big.NewInt(20000))
		ccopyValWrap.MinSelfDelegation.Mul(big.NewInt(1e18), big.NewInt(30000))

		origValWrap.MaxTotalDelegation.Mul(big.NewInt(1e18), big.NewInt(10000))
		copyValWrap.MaxTotalDelegation.Mul(big.NewInt(1e18), big.NewInt(20000))
		ccopyValWrap.MaxTotalDelegation.Mul(big.NewInt(1e18), big.NewInt(30000))

		origValWrap.CreationHeight.SetInt64(1)
		copyValWrap.CreationHeight.SetInt64(2)
		ccopyValWrap.CreationHeight.SetInt64(3)

		origValWrap.UpdateHeight.SetInt64(1)
		copyValWrap.UpdateHeight.SetInt64(2)
		ccopyValWrap.UpdateHeight.SetInt64(3)

		origValWrap.Description.Name = "UpdatedOriginal" + string(i)
		copyValWrap.Description.Name = "UpdatedCopy" + string(i)
		ccopyValWrap.Description.Name = "UpdatedCCopy" + string(i)

		origValWrap.Delegations[0].Amount.SetInt64(1)
		copyValWrap.Delegations[0].Amount.SetInt64(2)
		ccopyValWrap.Delegations[0].Amount.SetInt64(3)

		origValWrap.Delegations[0].Reward.SetInt64(1)
		copyValWrap.Delegations[0].Reward.SetInt64(2)
		ccopyValWrap.Delegations[0].Reward.SetInt64(3)

		origValWrap.Delegations[0].Undelegations[0].Amount.SetInt64(1)
		copyValWrap.Delegations[0].Undelegations[0].Amount.SetInt64(2)
		ccopyValWrap.Delegations[0].Undelegations[0].Amount.SetInt64(3)

		origValWrap.Delegations[0].Undelegations[0].Epoch.SetInt64(1)
		copyValWrap.Delegations[0].Undelegations[0].Epoch.SetInt64(2)
		ccopyValWrap.Delegations[0].Undelegations[0].Epoch.SetInt64(3)

		origValWrap.Counters.NumBlocksToSign.SetInt64(1)
		copyValWrap.Counters.NumBlocksToSign.SetInt64(2)
		ccopyValWrap.Counters.NumBlocksToSign.SetInt64(3)

		origValWrap.Counters.NumBlocksSigned.SetInt64(1)
		copyValWrap.Counters.NumBlocksSigned.SetInt64(2)
		ccopyValWrap.Counters.NumBlocksSigned.SetInt64(3)

		origValWrap.BlockReward.SetInt64(1)
		copyValWrap.BlockReward.SetInt64(2)
		ccopyValWrap.BlockReward.SetInt64(3)

		err = orig.UpdateValidatorWrapper(common.BytesToAddress([]byte{i}), origValWrap)
		if err != nil {
			t.Errorf("Couldn't update ValidatorWrapper %d with error %s", i, err)
		}
		err = copy.UpdateValidatorWrapper(common.BytesToAddress([]byte{i}), copyValWrap)
		if err != nil {
			t.Errorf("Couldn't update ValidatorWrapper %d with error %s", i, err)
		}
		err = ccopy.UpdateValidatorWrapper(common.BytesToAddress([]byte{i}), ccopyValWrap)
		if err != nil {
			t.Errorf("Couldn't update ValidatorWrapper %d with error %s", i, err)
		}

	}

	// Finalise the changes on all concurrently
	finalise := func(wg *sync.WaitGroup, db *DB) {
		defer wg.Done()
		db.Finalise(true)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go finalise(&wg, orig)
	go finalise(&wg, copy)
	go finalise(&wg, ccopy)
	wg.Wait()

	// Verify that the three states have been updated independently
	for i := byte(0); i < 255; i++ {
		origObj := orig.GetOrNewStateObject(common.BytesToAddress([]byte{i}))
		copyObj := copy.GetOrNewStateObject(common.BytesToAddress([]byte{i}))
		ccopyObj := ccopy.GetOrNewStateObject(common.BytesToAddress([]byte{i}))

		if want := big.NewInt(3 * int64(i)); origObj.Balance().Cmp(want) != 0 {
			t.Errorf("orig obj %d: balance mismatch: have %v, want %v", i, origObj.Balance(), want)
		}
		if want := big.NewInt(4 * int64(i)); copyObj.Balance().Cmp(want) != 0 {
			t.Errorf("copy obj %d: balance mismatch: have %v, want %v", i, copyObj.Balance(), want)
		}
		if want := big.NewInt(5 * int64(i)); ccopyObj.Balance().Cmp(want) != 0 {
			t.Errorf("copy obj %d: balance mismatch: have %v, want %v", i, ccopyObj.Balance(), want)
		}

		origValWrap, err := orig.ValidatorWrapper(common.BytesToAddress([]byte{i}), true, false)
		if err != nil {
			t.Errorf("Couldn't get validatorWrapper %d with error: %s", i, err)
		}
		copyValWrap, err := copy.ValidatorWrapper(common.BytesToAddress([]byte{i}), true, false)
		if err != nil {
			t.Errorf("Couldn't get validatorWrapper %d with error: %s", i, err)
		}
		ccopyValWrap, err := ccopy.ValidatorWrapper(common.BytesToAddress([]byte{i}), true, false)
		if err != nil {
			t.Errorf("Couldn't get validatorWrapper %d with error: %s", i, err)
		}

		if origValWrap.LastEpochInCommittee.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("LastEpochInCommittee %d: balance mismatch: have %v, want %v", i, origValWrap.LastEpochInCommittee, big.NewInt(1))
		}
		if copyValWrap.LastEpochInCommittee.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("LastEpochInCommittee %d: balance mismatch: have %v, want %v", i, copyValWrap.LastEpochInCommittee, big.NewInt(2))
		}
		if ccopyValWrap.LastEpochInCommittee.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("LastEpochInCommittee %d: balance mismatch: have %v, want %v", i, ccopyValWrap.LastEpochInCommittee, big.NewInt(3))
		}

		if want := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(10000)); origValWrap.MinSelfDelegation.Cmp(want) != 0 {
			t.Errorf("MinSelfDelegation %d: balance mismatch: have %v, want %v", i, origValWrap.MinSelfDelegation, want)
		}
		if want := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(20000)); copyValWrap.MinSelfDelegation.Cmp(want) != 0 {
			t.Errorf("MinSelfDelegation %d: balance mismatch: have %v, want %v", i, copyValWrap.MinSelfDelegation, want)
		}
		if want := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(30000)); ccopyValWrap.MinSelfDelegation.Cmp(want) != 0 {
			t.Errorf("MinSelfDelegation %d: balance mismatch: have %v, want %v", i, ccopyValWrap.MinSelfDelegation, want)
		}

		if want := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(10000)); origValWrap.MaxTotalDelegation.Cmp(want) != 0 {
			t.Errorf("MaxTotalDelegation %d: balance mismatch: have %v, want %v", i, origValWrap.MaxTotalDelegation, want)
		}
		if want := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(20000)); copyValWrap.MaxTotalDelegation.Cmp(want) != 0 {
			t.Errorf("MaxTotalDelegation %d: balance mismatch: have %v, want %v", i, copyValWrap.MaxTotalDelegation, want)
		}
		if want := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(30000)); ccopyValWrap.MaxTotalDelegation.Cmp(want) != 0 {
			t.Errorf("MaxTotalDelegation %d: balance mismatch: have %v, want %v", i, ccopyValWrap.MaxTotalDelegation, want)
		}

		if origValWrap.CreationHeight.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("CreationHeight %d: balance mismatch: have %v, want %v", i, origValWrap.CreationHeight, big.NewInt(1))
		}
		if copyValWrap.CreationHeight.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("CreationHeight %d: balance mismatch: have %v, want %v", i, copyValWrap.CreationHeight, big.NewInt(2))
		}
		if ccopyValWrap.CreationHeight.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("CreationHeight %d: balance mismatch: have %v, want %v", i, ccopyValWrap.CreationHeight, big.NewInt(3))
		}

		if origValWrap.UpdateHeight.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("UpdateHeight %d: balance mismatch: have %v, want %v", i, origValWrap.UpdateHeight, big.NewInt(1))
		}
		if copyValWrap.UpdateHeight.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("UpdateHeight %d: balance mismatch: have %v, want %v", i, copyValWrap.UpdateHeight, big.NewInt(2))
		}
		if ccopyValWrap.UpdateHeight.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("UpdateHeight %d: balance mismatch: have %v, want %v", i, ccopyValWrap.UpdateHeight, big.NewInt(3))
		}

		if want := "UpdatedOriginal" + string(i); origValWrap.Description.Name != want {
			t.Errorf("originalValWrap %d: Incorrect Name: have %s, want %s", i, origValWrap.Description.Name, want)
		}
		if want := "UpdatedCopy" + string(i); copyValWrap.Description.Name != want {
			t.Errorf("originalValWrap %d: Incorrect Name: have %s, want %s", i, copyValWrap.Description.Name, want)
		}
		if want := "UpdatedCCopy" + string(i); ccopyValWrap.Description.Name != want {
			t.Errorf("originalValWrap %d: Incorrect Name: have %s, want %s", i, ccopyValWrap.Description.Name, want)
		}

		if origValWrap.Delegations[0].Amount.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("Delegations[0].Amount %d: balance mismatch: have %v, want %v", i, origValWrap.Delegations[0].Amount, big.NewInt(1))
		}
		if copyValWrap.Delegations[0].Amount.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("Delegations[0].Amount %d: balance mismatch: have %v, want %v", i, copyValWrap.Delegations[0].Amount, big.NewInt(2))
		}
		if ccopyValWrap.Delegations[0].Amount.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("Delegations[0].Amount %d: balance mismatch: have %v, want %v", i, ccopyValWrap.Delegations[0].Amount, big.NewInt(3))
		}

		if origValWrap.Delegations[0].Reward.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("Delegations[0].Reward %d: balance mismatch: have %v, want %v", i, origValWrap.Delegations[0].Reward, big.NewInt(1))
		}
		if copyValWrap.Delegations[0].Reward.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("Delegations[0].Reward %d: balance mismatch: have %v, want %v", i, copyValWrap.Delegations[0].Reward, big.NewInt(2))
		}
		if ccopyValWrap.Delegations[0].Reward.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("Delegations[0].Reward %d: balance mismatch: have %v, want %v", i, ccopyValWrap.Delegations[0].Reward, big.NewInt(3))
		}

		if origValWrap.Delegations[0].Undelegations[0].Amount.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("Delegations[0].Undelegations[0].Amount %d: balance mismatch: have %v, want %v", i, origValWrap.Delegations[0].Undelegations[0].Amount, big.NewInt(1))
		}
		if copyValWrap.Delegations[0].Undelegations[0].Amount.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("Delegations[0].Undelegations[0].Amount %d: balance mismatch: have %v, want %v", i, copyValWrap.Delegations[0].Undelegations[0].Amount, big.NewInt(2))
		}
		if ccopyValWrap.Delegations[0].Undelegations[0].Amount.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("Delegations[0].Undelegations[0].Amount %d: balance mismatch: have %v, want %v", i, ccopyValWrap.Delegations[0].Undelegations[0].Amount, big.NewInt(3))
		}

		if origValWrap.Delegations[0].Undelegations[0].Epoch.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("CreationHeight %d: balance mismatch: have %v, want %v", i, origValWrap.Delegations[0].Undelegations[0].Epoch, big.NewInt(1))
		}
		if copyValWrap.Delegations[0].Undelegations[0].Epoch.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("CreationHeight %d: balance mismatch: have %v, want %v", i, copyValWrap.Delegations[0].Undelegations[0].Epoch, big.NewInt(2))
		}
		if ccopyValWrap.Delegations[0].Undelegations[0].Epoch.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("CreationHeight %d: balance mismatch: have %v, want %v", i, ccopyValWrap.Delegations[0].Undelegations[0].Epoch, big.NewInt(3))
		}

		if origValWrap.Counters.NumBlocksToSign.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("Counters.NumBlocksToSign %d: balance mismatch: have %v, want %v", i, origValWrap.Counters.NumBlocksToSign, big.NewInt(1))
		}
		if copyValWrap.Counters.NumBlocksToSign.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("Counters.NumBlocksToSign %d: balance mismatch: have %v, want %v", i, copyValWrap.Counters.NumBlocksToSign, big.NewInt(2))
		}
		if ccopyValWrap.Counters.NumBlocksToSign.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("Counters.NumBlocksToSign %d: balance mismatch: have %v, want %v", i, ccopyValWrap.Counters.NumBlocksToSign, big.NewInt(3))
		}

		if origValWrap.Counters.NumBlocksSigned.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("Counters.NumBlocksSigned %d: balance mismatch: have %v, want %v", i, origValWrap.Counters.NumBlocksSigned, big.NewInt(1))
		}
		if copyValWrap.Counters.NumBlocksSigned.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("Counters.NumBlocksSigned %d: balance mismatch: have %v, want %v", i, copyValWrap.Counters.NumBlocksSigned, big.NewInt(2))
		}
		if ccopyValWrap.Counters.NumBlocksSigned.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("Counters.NumBlocksSigned %d: balance mismatch: have %v, want %v", i, ccopyValWrap.Counters.NumBlocksSigned, big.NewInt(3))
		}

		if origValWrap.BlockReward.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("Block Reward %d: balance mismatch: have %v, want %v", i, origValWrap.BlockReward, big.NewInt(1))
		}
		if copyValWrap.BlockReward.Cmp(big.NewInt(2)) != 0 {
			t.Errorf("Block Reward %d: balance mismatch: have %v, want %v", i, copyValWrap.BlockReward, big.NewInt(2))
		}
		if ccopyValWrap.BlockReward.Cmp(big.NewInt(3)) != 0 {
			t.Errorf("Block Reward %d: balance mismatch: have %v, want %v", i, ccopyValWrap.BlockReward, big.NewInt(3))
		}
	}
}

func TestSnapshotRandom(t *testing.T) {
	config := &quick.Config{MaxCount: 1000}
	err := quick.Check((*snapshotTest).run, config)
	if cerr, ok := err.(*quick.CheckError); ok {
		test := cerr.In[0].(*snapshotTest)
		t.Errorf("%v:\n%s", test.err, test)
	} else if err != nil {
		t.Error(err)
	}
}

// A snapshotTest checks that reverting StateDB snapshots properly undoes all changes
// captured by the snapshot. Instances of this test with pseudorandom content are created
// by Generate.
//
// The test works as follows:
//
// A new state is created and all actions are applied to it. Several snapshots are taken
// in between actions. The test then reverts each snapshot. For each snapshot the actions
// leading up to it are replayed on a fresh, empty state. The behaviour of all public
// accessor methods on the reverted state must match the return value of the equivalent
// methods on the replayed state.
type snapshotTest struct {
	addrs     []common.Address // all account addresses
	actions   []testAction     // modifications to the state
	snapshots []int            // actions indexes at which snapshot is taken
	err       error            // failure details are reported through this field
}

type testAction struct {
	name   string
	fn     func(testAction, *DB)
	args   []int64
	noAddr bool
}

// newTestAction creates a random action that changes state.
func newTestAction(addr common.Address, r *rand.Rand) testAction {
	actions := []testAction{
		{
			name: "SetBalance",
			fn: func(a testAction, s *DB) {
				s.SetBalance(addr, big.NewInt(a.args[0]))
			},
			args: make([]int64, 1),
		},
		{
			name: "AddBalance",
			fn: func(a testAction, s *DB) {
				s.AddBalance(addr, big.NewInt(a.args[0]))
			},
			args: make([]int64, 1),
		},
		{
			name: "SetNonce",
			fn: func(a testAction, s *DB) {
				s.SetNonce(addr, uint64(a.args[0]))
			},
			args: make([]int64, 1),
		},
		{
			name: "SetState",
			fn: func(a testAction, s *DB) {
				var key, val common.Hash
				binary.BigEndian.PutUint16(key[:], uint16(a.args[0]))
				binary.BigEndian.PutUint16(val[:], uint16(a.args[1]))
				s.SetState(addr, key, val)
			},
			args: make([]int64, 2),
		},
		{
			name: "SetCode",
			fn: func(a testAction, s *DB) {
				code := make([]byte, 16)
				binary.BigEndian.PutUint64(code, uint64(a.args[0]))
				binary.BigEndian.PutUint64(code[8:], uint64(a.args[1]))
				s.SetCode(addr, code)
			},
			args: make([]int64, 2),
		},
		{
			name: "CreateAccount",
			fn: func(a testAction, s *DB) {
				s.CreateAccount(addr)
			},
		},
		{
			name: "Suicide",
			fn: func(a testAction, s *DB) {
				s.Suicide(addr)
			},
		},
		{
			name: "AddRefund",
			fn: func(a testAction, s *DB) {
				s.AddRefund(uint64(a.args[0]))
			},
			args:   make([]int64, 1),
			noAddr: true,
		},
		{
			name: "AddLog",
			fn: func(a testAction, s *DB) {
				data := make([]byte, 2)
				binary.BigEndian.PutUint16(data, uint16(a.args[0]))
				s.AddLog(&types.Log{Address: addr, Data: data})
			},
			args: make([]int64, 1),
		},
		{
			name: "AddPreimage",
			fn: func(a testAction, s *DB) {
				preimage := []byte{1}
				hash := common.BytesToHash(preimage)
				s.AddPreimage(hash, preimage)
			},
			args: make([]int64, 1),
		},
	}
	action := actions[r.Intn(len(actions))]
	var nameargs []string
	if !action.noAddr {
		nameargs = append(nameargs, addr.Hex())
	}
	for i := range action.args {
		action.args[i] = rand.Int63n(100)
		nameargs = append(nameargs, fmt.Sprint(action.args[i]))
	}
	action.name += strings.Join(nameargs, ", ")
	return action
}

// Generate returns a new snapshot test of the given size. All randomness is
// derived from r.
func (*snapshotTest) Generate(r *rand.Rand, size int) reflect.Value {
	// Generate random actions.
	addrs := make([]common.Address, 50)
	for i := range addrs {
		addrs[i][0] = byte(i)
	}
	actions := make([]testAction, size)
	for i := range actions {
		addr := addrs[r.Intn(len(addrs))]
		actions[i] = newTestAction(addr, r)
	}
	// Generate snapshot indexes.
	nsnapshots := int(math.Sqrt(float64(size)))
	if size > 0 && nsnapshots == 0 {
		nsnapshots = 1
	}
	snapshots := make([]int, nsnapshots)
	snaplen := len(actions) / nsnapshots
	for i := range snapshots {
		// Try to place the snapshots some number of actions apart from each other.
		snapshots[i] = (i * snaplen) + r.Intn(snaplen)
	}
	return reflect.ValueOf(&snapshotTest{addrs, actions, snapshots, nil})
}

func (test *snapshotTest) String() string {
	out := new(bytes.Buffer)
	sindex := 0
	for i, action := range test.actions {
		if len(test.snapshots) > sindex && i == test.snapshots[sindex] {
			fmt.Fprintf(out, "---- snapshot %d ----\n", sindex)
			sindex++
		}
		fmt.Fprintf(out, "%4d: %s\n", i, action.name)
	}
	return out.String()
}

func (test *snapshotTest) run() bool {
	// Run all actions and create snapshots.
	var (
		state, _     = New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))
		snapshotRevs = make([]int, len(test.snapshots))
		sindex       = 0
	)
	for i, action := range test.actions {
		if len(test.snapshots) > sindex && i == test.snapshots[sindex] {
			snapshotRevs[sindex] = state.Snapshot()
			sindex++
		}
		action.fn(action, state)
	}
	// Revert all snapshots in reverse order. Each revert must yield a state
	// that is equivalent to fresh state with all actions up the snapshot applied.
	for sindex--; sindex >= 0; sindex-- {
		checkstate, _ := New(common.Hash{}, state.Database())
		for _, action := range test.actions[:test.snapshots[sindex]] {
			action.fn(action, checkstate)
		}
		state.RevertToSnapshot(snapshotRevs[sindex])
		if err := test.checkEqual(state, checkstate); err != nil {
			test.err = fmt.Errorf("state mismatch after revert to snapshot %d\n%v", sindex, err)
			return false
		}
	}
	return true
}

// checkEqual checks that methods of state and checkstate return the same values.
func (test *snapshotTest) checkEqual(state, checkstate *DB) error {
	for _, addr := range test.addrs {
		var err error
		checkeq := func(op string, a, b interface{}) bool {
			if err == nil && !reflect.DeepEqual(a, b) {
				err = fmt.Errorf("got %s(%s) == %v, want %v", op, addr.Hex(), a, b)
				return false
			}
			return true
		}
		// Check basic accessor methods.
		checkeq("Exist", state.Exist(addr), checkstate.Exist(addr))
		checkeq("HasSuicided", state.HasSuicided(addr), checkstate.HasSuicided(addr))
		checkeq("GetBalance", state.GetBalance(addr), checkstate.GetBalance(addr))
		checkeq("GetNonce", state.GetNonce(addr), checkstate.GetNonce(addr))
		checkeq("GetCode", state.GetCode(addr), checkstate.GetCode(addr))
		checkeq("GetCodeHash", state.GetCodeHash(addr), checkstate.GetCodeHash(addr))
		checkeq("GetCodeSize", state.GetCodeSize(addr), checkstate.GetCodeSize(addr))
		// Check storage.
		if obj := state.getStateObject(addr); obj != nil {
			state.ForEachStorage(addr, func(key, value common.Hash) bool {
				return checkeq("GetState("+key.Hex()+")", checkstate.GetState(addr, key), value)
			})
			checkstate.ForEachStorage(addr, func(key, value common.Hash) bool {
				return checkeq("GetState("+key.Hex()+")", checkstate.GetState(addr, key), value)
			})
		}
		if err != nil {
			return err
		}
	}

	if state.GetRefund() != checkstate.GetRefund() {
		return fmt.Errorf("got GetRefund() == %d, want GetRefund() == %d",
			state.GetRefund(), checkstate.GetRefund())
	}
	if !reflect.DeepEqual(state.GetLogs(common.Hash{}), checkstate.GetLogs(common.Hash{})) {
		return fmt.Errorf("got GetLogs(common.Hash{}) == %v, want GetLogs(common.Hash{}) == %v",
			state.GetLogs(common.Hash{}), checkstate.GetLogs(common.Hash{}))
	}
	return nil
}

func TestTouchDelete(t *testing.T) {
	s := newStateTest()
	s.state.GetOrNewStateObject(common.Address{})
	root, _ := s.state.Commit(false)
	s.state.Reset(root)

	snapshot := s.state.Snapshot()
	s.state.AddBalance(common.Address{}, new(big.Int))

	if len(s.state.journal.dirties) != 1 {
		t.Fatal("expected one dirty state object")
	}
	s.state.RevertToSnapshot(snapshot)
	if len(s.state.journal.dirties) != 0 {
		t.Fatal("expected no dirty state object")
	}
}

// TestCopyOfCopy tests that modified objects are carried over to the copy, and the copy of the copy.
// See https://github.com/ethereum/go-ethereum/pull/15225#issuecomment-380191512
func TestCopyOfCopy(t *testing.T) {
	state, _ := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))
	addr := common.HexToAddress("aaaa")
	state.SetBalance(addr, big.NewInt(42))

	if got := state.Copy().GetBalance(addr).Uint64(); got != 42 {
		t.Fatalf("1st copy fail, expected 42, got %v", got)
	}
	if got := state.Copy().Copy().GetBalance(addr).Uint64(); got != 42 {
		t.Fatalf("2nd copy fail, expected 42, got %v", got)
	}
}

// Tests a regression where committing a copy lost some internal meta information,
// leading to corrupted subsequent copies.
//
// See https://github.com/ethereum/go-ethereum/issues/20106.
func TestCopyCommitCopy(t *testing.T) {
	state, _ := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))

	// Create an account and check if the retrieved balance is correct
	addr := common.HexToAddress("0xaffeaffeaffeaffeaffeaffeaffeaffeaffeaffe")
	skey := common.HexToHash("aaa")
	sval := common.HexToHash("bbb")

	state.SetBalance(addr, big.NewInt(42)) // Change the account trie
	state.SetCode(addr, []byte("hello"))   // Change an external metadata
	state.SetState(addr, skey, sval)       // Change the storage trie

	if balance := state.GetBalance(addr); balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("initial balance mismatch: have %v, want %v", balance, 42)
	}
	if code := state.GetCode(addr); !bytes.Equal(code, []byte("hello")) {
		t.Fatalf("initial code mismatch: have %x, want %x", code, []byte("hello"))
	}
	if val := state.GetState(addr, skey); val != sval {
		t.Fatalf("initial non-committed storage slot mismatch: have %x, want %x", val, sval)
	}
	if val := state.GetCommittedState(addr, skey); val != (common.Hash{}) {
		t.Fatalf("initial committed storage slot mismatch: have %x, want %x", val, common.Hash{})
	}
	// Copy the non-committed state database and check pre/post commit balance
	copyOne := state.Copy()
	if balance := copyOne.GetBalance(addr); balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("first copy pre-commit balance mismatch: have %v, want %v", balance, 42)
	}
	if code := copyOne.GetCode(addr); !bytes.Equal(code, []byte("hello")) {
		t.Fatalf("first copy pre-commit code mismatch: have %x, want %x", code, []byte("hello"))
	}
	if val := copyOne.GetState(addr, skey); val != sval {
		t.Fatalf("first copy pre-commit non-committed storage slot mismatch: have %x, want %x", val, sval)
	}
	if val := copyOne.GetCommittedState(addr, skey); val != (common.Hash{}) {
		t.Fatalf("first copy pre-commit committed storage slot mismatch: have %x, want %x", val, common.Hash{})
	}

	copyOne.Commit(false)
	if balance := copyOne.GetBalance(addr); balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("first copy post-commit balance mismatch: have %v, want %v", balance, 42)
	}
	if code := copyOne.GetCode(addr); !bytes.Equal(code, []byte("hello")) {
		t.Fatalf("first copy post-commit code mismatch: have %x, want %x", code, []byte("hello"))
	}
	if val := copyOne.GetState(addr, skey); val != sval {
		t.Fatalf("first copy post-commit non-committed storage slot mismatch: have %x, want %x", val, sval)
	}
	if val := copyOne.GetCommittedState(addr, skey); val != sval {
		t.Fatalf("first copy post-commit committed storage slot mismatch: have %x, want %x", val, sval)
	}
	// Copy the copy and check the balance once more
	copyTwo := copyOne.Copy()
	if balance := copyTwo.GetBalance(addr); balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("second copy balance mismatch: have %v, want %v", balance, 42)
	}
	if code := copyTwo.GetCode(addr); !bytes.Equal(code, []byte("hello")) {
		t.Fatalf("second copy code mismatch: have %x, want %x", code, []byte("hello"))
	}
	if val := copyTwo.GetState(addr, skey); val != sval {
		t.Fatalf("second copy non-committed storage slot mismatch: have %x, want %x", val, sval)
	}
	if val := copyTwo.GetCommittedState(addr, skey); val != sval {
		t.Fatalf("second copy post-commit committed storage slot mismatch: have %x, want %x", val, sval)
	}
}

// Tests a regression where committing a copy lost some internal meta information,
// leading to corrupted subsequent copies.
//
// See https://github.com/ethereum/go-ethereum/issues/20106.
func TestCopyCopyCommitCopy(t *testing.T) {
	state, _ := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))

	// Create an account and check if the retrieved balance is correct
	addr := common.HexToAddress("0xaffeaffeaffeaffeaffeaffeaffeaffeaffeaffe")
	skey := common.HexToHash("aaa")
	sval := common.HexToHash("bbb")

	state.SetBalance(addr, big.NewInt(42)) // Change the account trie
	state.SetCode(addr, []byte("hello"))   // Change an external metadata
	state.SetState(addr, skey, sval)       // Change the storage trie

	if balance := state.GetBalance(addr); balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("initial balance mismatch: have %v, want %v", balance, 42)
	}
	if code := state.GetCode(addr); !bytes.Equal(code, []byte("hello")) {
		t.Fatalf("initial code mismatch: have %x, want %x", code, []byte("hello"))
	}
	if val := state.GetState(addr, skey); val != sval {
		t.Fatalf("initial non-committed storage slot mismatch: have %x, want %x", val, sval)
	}
	if val := state.GetCommittedState(addr, skey); val != (common.Hash{}) {
		t.Fatalf("initial committed storage slot mismatch: have %x, want %x", val, common.Hash{})
	}
	// Copy the non-committed state database and check pre/post commit balance
	copyOne := state.Copy()
	if balance := copyOne.GetBalance(addr); balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("first copy balance mismatch: have %v, want %v", balance, 42)
	}
	if code := copyOne.GetCode(addr); !bytes.Equal(code, []byte("hello")) {
		t.Fatalf("first copy code mismatch: have %x, want %x", code, []byte("hello"))
	}
	if val := copyOne.GetState(addr, skey); val != sval {
		t.Fatalf("first copy non-committed storage slot mismatch: have %x, want %x", val, sval)
	}
	if val := copyOne.GetCommittedState(addr, skey); val != (common.Hash{}) {
		t.Fatalf("first copy committed storage slot mismatch: have %x, want %x", val, common.Hash{})
	}
	// Copy the copy and check the balance once more
	copyTwo := copyOne.Copy()
	if balance := copyTwo.GetBalance(addr); balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("second copy pre-commit balance mismatch: have %v, want %v", balance, 42)
	}
	if code := copyTwo.GetCode(addr); !bytes.Equal(code, []byte("hello")) {
		t.Fatalf("second copy pre-commit code mismatch: have %x, want %x", code, []byte("hello"))
	}
	if val := copyTwo.GetState(addr, skey); val != sval {
		t.Fatalf("second copy pre-commit non-committed storage slot mismatch: have %x, want %x", val, sval)
	}
	if val := copyTwo.GetCommittedState(addr, skey); val != (common.Hash{}) {
		t.Fatalf("second copy pre-commit committed storage slot mismatch: have %x, want %x", val, common.Hash{})
	}
	copyTwo.Commit(false)
	if balance := copyTwo.GetBalance(addr); balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("second copy post-commit balance mismatch: have %v, want %v", balance, 42)
	}
	if code := copyTwo.GetCode(addr); !bytes.Equal(code, []byte("hello")) {
		t.Fatalf("second copy post-commit code mismatch: have %x, want %x", code, []byte("hello"))
	}
	if val := copyTwo.GetState(addr, skey); val != sval {
		t.Fatalf("second copy post-commit non-committed storage slot mismatch: have %x, want %x", val, sval)
	}
	if val := copyTwo.GetCommittedState(addr, skey); val != sval {
		t.Fatalf("second copy post-commit committed storage slot mismatch: have %x, want %x", val, sval)
	}
	// Copy the copy-copy and check the balance once more
	copyThree := copyTwo.Copy()
	if balance := copyThree.GetBalance(addr); balance.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("third copy balance mismatch: have %v, want %v", balance, 42)
	}
	if code := copyThree.GetCode(addr); !bytes.Equal(code, []byte("hello")) {
		t.Fatalf("third copy code mismatch: have %x, want %x", code, []byte("hello"))
	}
	if val := copyThree.GetState(addr, skey); val != sval {
		t.Fatalf("third copy non-committed storage slot mismatch: have %x, want %x", val, sval)
	}
	if val := copyThree.GetCommittedState(addr, skey); val != sval {
		t.Fatalf("third copy committed storage slot mismatch: have %x, want %x", val, sval)
	}
}

// TestDeleteCreateRevert tests a weird state transition corner case that we hit
// while changing the internals of statedb. The workflow is that a contract is
// self destructed, then in a followup transaction (but same block) it's created
// again and the transaction reverted.
//
// The original statedb implementation flushed dirty objects to the tries after
// each transaction, so this works ok. The rework accumulated writes in memory
// first, but the journal wiped the entire state object on create-revert.
func TestDeleteCreateRevert(t *testing.T) {
	// Create an initial state with a single contract
	state, _ := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))

	addr := common.BytesToAddress([]byte("so"))
	state.SetBalance(addr, big.NewInt(1))

	root, _ := state.Commit(false)
	state.Reset(root)

	// Simulate self-destructing in one transaction, then create-reverting in another
	state.Suicide(addr)
	state.Finalise(true)

	id := state.Snapshot()
	state.SetBalance(addr, big.NewInt(2))
	state.RevertToSnapshot(id)

	// Commit the entire state and make sure we don't crash and have the correct state
	root, _ = state.Commit(true)
	state.Reset(root)

	if state.getStateObject(addr) != nil {
		t.Fatalf("self-destructed contract came alive")
	}
}

func makeValidValidatorWrapper(addr common.Address) stk.ValidatorWrapper {
	cr := stk.CommissionRates{
		Rate:          numeric.ZeroDec(),
		MaxRate:       numeric.ZeroDec(),
		MaxChangeRate: numeric.ZeroDec(),
	}
	c := stk.Commission{cr, big.NewInt(300)}
	d := stk.Description{
		Name:     "Wayne",
		Identity: "wen",
		Website:  "harmony.one.wen",
		Details:  "best",
	}

	v := stk.Validator{
		Address:              addr,
		SlotPubKeys:          []bls.SerializedPublicKey{makeBLSPubSigPair().pub},
		LastEpochInCommittee: big.NewInt(20),
		MinSelfDelegation:    new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18)),
		MaxTotalDelegation:   new(big.Int).Mul(big.NewInt(12000), big.NewInt(1e18)),
		Commission:           c,
		Description:          d,
		CreationHeight:       big.NewInt(12306),
	}
	ds := stk.Delegations{
		stk.Delegation{
			DelegatorAddress: v.Address,
			Amount:           big.NewInt(0),
			Reward:           big.NewInt(0),
			Undelegations: stk.Undelegations{
				stk.Undelegation{
					Amount: big.NewInt(0),
					Epoch:  big.NewInt(0),
				},
			},
		},
	}
	br := big.NewInt(1)

	w := stk.ValidatorWrapper{
		Validator:   v,
		Delegations: ds,
		BlockReward: br,
	}
	w.Counters.NumBlocksSigned = big.NewInt(0)
	w.Counters.NumBlocksToSign = big.NewInt(0)
	return w
}

type blsPubSigPair struct {
	pub bls.SerializedPublicKey
	sig bls.SerializedSignature
}

func makeBLSPubSigPair() blsPubSigPair {
	blsPriv := bls.RandPrivateKey()
	blsPub := blsPriv.GetPublicKey()
	msgHash := hash.Keccak256([]byte(stk.BLSVerificationStr))
	sig := blsPriv.SignHash(msgHash)

	var shardPub bls.SerializedPublicKey
	copy(shardPub[:], blsPub.Serialize())

	var shardSig bls.SerializedSignature
	copy(shardSig[:], sig.Serialize())

	return blsPubSigPair{shardPub, shardSig}
}

func updateAndCheckValidator(t *testing.T, state *DB, wrapper stk.ValidatorWrapper) {
	// do not modify / link the original into the state object
	copied := staketest.CopyValidatorWrapper(wrapper)
	if err := state.UpdateValidatorWrapperWithRevert(copied.Address, &copied); err != nil {
		t.Fatalf("Could not update wrapper with revert %v\n", err)
	}

	// load a copy here to be safe
	loadedWrapper, err := state.ValidatorWrapper(copied.Address, false, true)
	if err != nil {
		t.Fatalf("Could not load wrapper %v\n", err)
	}

	if err := staketest.CheckValidatorWrapperEqual(wrapper, *loadedWrapper); err != nil {
		t.Fatalf("Wrappers are unequal %v\n", err)
	}
}

func verifyValidatorWrapperRevert(
	t *testing.T,
	state *DB,
	snapshot int,
	wrapperAddress common.Address, // if expectedWrapper is nil, this is needed
	allowErrAddressNotPresent bool,
	expectedWrapper *stk.ValidatorWrapper,
	stateToCompare *DB,
	modifiedAddresses []common.Address,
) {
	state.RevertToSnapshot(snapshot)
	loadedWrapper, err := state.ValidatorWrapper(wrapperAddress, true, false)
	if err != nil && !(err == errAddressNotPresent && allowErrAddressNotPresent) {
		t.Fatalf("Could not load wrapper %v\n", err)
	}
	if expectedWrapper != nil {
		if err := staketest.CheckValidatorWrapperEqual(*expectedWrapper, *loadedWrapper); err != nil {
			fmt.Printf("ExpectWrapper: %v\n", expectedWrapper)
			fmt.Printf("LoadedWrapper: %v\n", loadedWrapper)
			fmt.Printf("ExpectCounters: %v\n", expectedWrapper.Counters)
			fmt.Printf("LoadedCounters: %v\n", loadedWrapper.Counters)
			t.Fatalf("Loaded wrapper not equal to expected wrapper after revert %v\n", err)
		}
	} else if loadedWrapper != nil {
		t.Fatalf("Expected wrapper is nil but got loaded wrapper %v\n", loadedWrapper)
	}

	st := &snapshotTest{addrs: modifiedAddresses}
	if err := st.checkEqual(state, stateToCompare); err != nil {
		t.Fatalf("State not as expected after revert %v\n", err)
	}
}

func TestValidatorCreationRevert(t *testing.T) {
	state, err := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))
	emptyState := state.Copy()
	if err != nil {
		t.Fatalf("Could not instantiate state %v\n", err)
	}
	snapshot := state.Snapshot()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Could not generate key %v\n", err)
	}
	wrapper := makeValidValidatorWrapper(crypto.PubkeyToAddress(key.PublicKey))
	// step 1 is adding the validator, and checking that is it successfully added
	updateAndCheckValidator(t, state, wrapper)
	// step 2 is the revert check, the meat of the test
	verifyValidatorWrapperRevert(t,
		state,
		snapshot,
		wrapper.Address,
		true,
		nil,
		emptyState,
		[]common.Address{wrapper.Address},
	)
}

func TestValidatorAddDelegationRevert(t *testing.T) {
	state, err := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))
	if err != nil {
		t.Fatalf("Could not instantiate state %v\n", err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Could not generate key %v\n", err)
	}
	delegatorKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Could not generate key %v\n", err)
	}
	wrapper := makeValidValidatorWrapper(crypto.PubkeyToAddress(key.PublicKey))
	// always, step 1 is adding the validator, and checking that is it successfully added
	updateAndCheckValidator(t, state, wrapper)
	wrapperWithoutDelegation := staketest.CopyValidatorWrapper(wrapper) // for comparison later
	stateWithoutDelegation := state.Copy()
	// we will revert to the state without the delegation
	snapshot := state.Snapshot()
	// which is added here
	wrapper.Delegations = append(wrapper.Delegations, stk.NewDelegation(
		crypto.PubkeyToAddress(delegatorKey.PublicKey),
		new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100))),
	)
	// again, add and make sure added == sent
	updateAndCheckValidator(t, state, wrapper)
	// now the meat of the test
	verifyValidatorWrapperRevert(t,
		state,
		snapshot,
		wrapper.Address,
		false,
		&wrapperWithoutDelegation,
		stateWithoutDelegation,
		[]common.Address{wrapper.Address, wrapper.Delegations[1].DelegatorAddress},
	)
}

type expectedRevertItem struct {
	snapshot                   int
	expectedWrapperAfterRevert *stk.ValidatorWrapper
	expectedStateAfterRevert   *DB
	modifiedAddresses          []common.Address
}

func makeExpectedRevertItem(state *DB,
	wrapper *stk.ValidatorWrapper,
	modifiedAddresses []common.Address,
) expectedRevertItem {
	x := expectedRevertItem{
		snapshot:                 state.Snapshot(),
		expectedStateAfterRevert: state.Copy(),
		modifiedAddresses:        modifiedAddresses,
	}
	if wrapper != nil {
		copied := staketest.CopyValidatorWrapper(*wrapper)
		x.expectedWrapperAfterRevert = &copied
	}
	return x
}

func TestValidatorMultipleReverts(t *testing.T) {
	var expectedRevertItems []expectedRevertItem

	state, err := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))
	if err != nil {
		t.Fatalf("Could not instantiate state %v\n", err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Could not generate key %v\n", err)
	}
	delegatorKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Could not generate key %v\n", err)
	}
	validatorAddress := crypto.PubkeyToAddress(key.PublicKey)
	delegatorAddress := crypto.PubkeyToAddress(delegatorKey.PublicKey)
	modifiedAddresses := []common.Address{validatorAddress, delegatorAddress}
	// first we add a validator
	expectedRevertItems = append(expectedRevertItems,
		makeExpectedRevertItem(state, nil, modifiedAddresses))
	wrapper := makeValidValidatorWrapper(crypto.PubkeyToAddress(key.PublicKey))
	updateAndCheckValidator(t, state, wrapper)
	// then we add a delegation
	expectedRevertItems = append(expectedRevertItems,
		makeExpectedRevertItem(state, &wrapper, modifiedAddresses))
	wrapper.Delegations = append(wrapper.Delegations, stk.NewDelegation(
		crypto.PubkeyToAddress(delegatorKey.PublicKey),
		new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100))),
	)
	updateAndCheckValidator(t, state, wrapper)
	// then we have it sign blocks
	wrapper.Counters.NumBlocksToSign.Add(
		wrapper.Counters.NumBlocksToSign, common.Big1,
	)
	wrapper.Counters.NumBlocksSigned.Add(
		wrapper.Counters.NumBlocksSigned, common.Big1,
	)
	updateAndCheckValidator(t, state, wrapper)
	// then modify the name and the block reward
	expectedRevertItems = append(expectedRevertItems,
		makeExpectedRevertItem(state, &wrapper, modifiedAddresses))
	wrapper.BlockReward.SetInt64(1)
	wrapper.Validator.Description.Name = "Name"
	for i := len(expectedRevertItems) - 1; i >= 0; i-- {
		item := expectedRevertItems[i]
		verifyValidatorWrapperRevert(t,
			state,
			item.snapshot,
			wrapper.Address,
			i == 0,
			item.expectedWrapperAfterRevert,
			item.expectedStateAfterRevert,
			[]common.Address{wrapper.Address},
		)
	}
}

func TestValidatorWrapperPanic(t *testing.T) {
	defer func() { recover() }()

	state, err := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))
	if err != nil {
		t.Fatalf("Could not instantiate state %v\n", err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Could not generate key %v\n", err)
	}
	validatorAddress := crypto.PubkeyToAddress(key.PublicKey)
	// will panic because we are asking for Original with copy of delegations
	_, _ = state.ValidatorWrapper(validatorAddress, true, true)
	t.Fatalf("Did not panic")
}

func TestValidatorWrapperGetCode(t *testing.T) {
	state, err := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()))
	if err != nil {
		t.Fatalf("Could not instantiate state %v\n", err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Could not generate key %v\n", err)
	}
	wrapper := makeValidValidatorWrapper(crypto.PubkeyToAddress(key.PublicKey))
	updateAndCheckValidator(t, state, wrapper)
	// delete it from the cache so we can force it to use GetCode
	delete(state.stateValidators, wrapper.Address)
	loadedWrapper, err := state.ValidatorWrapper(wrapper.Address, false, false)
	if err := staketest.CheckValidatorWrapperEqual(wrapper, *loadedWrapper); err != nil {
		fmt.Printf("ExpectWrapper: %v\n", wrapper)
		fmt.Printf("LoadedWrapper: %v\n", loadedWrapper)
		fmt.Printf("ExpectCounters: %v\n", wrapper.Counters)
		fmt.Printf("LoadedCounters: %v\n", loadedWrapper.Counters)
		t.Fatalf("Loaded wrapper not equal to expected wrapper%v\n", err)
	}
}
