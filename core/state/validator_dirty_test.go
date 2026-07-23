package state

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/numeric"
	staketest "github.com/harmony-one/harmony/staking/types/test"
)

func TestFinaliseWritesOnlyDirtyValidators(t *testing.T) {
	db, _ := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()), nil)

	cleanAddr := common.BytesToAddress([]byte("validator-clean"))
	dirtyAddr := common.BytesToAddress([]byte("validator-dirty"))

	clean := staketest.GetDefaultValidatorWrapper()
	clean.Address = cleanAddr
	clean.Delegations[0].DelegatorAddress = cleanAddr
	dirty := staketest.GetDefaultValidatorWrapper()
	dirty.Address = dirtyAddr
	dirty.Delegations[0].DelegatorAddress = dirtyAddr

	if err := db.UpdateValidatorWrapper(cleanAddr, &clean); err != nil {
		t.Fatalf("UpdateValidatorWrapper clean: %v", err)
	}
	if err := db.UpdateValidatorWrapper(dirtyAddr, &dirty); err != nil {
		t.Fatalf("UpdateValidatorWrapper dirty: %v", err)
	}
	db.Finalise(false)

	cleanCodeBefore := append([]byte(nil), db.GetCode(cleanAddr)...)
	dirtyCodeBefore := append([]byte(nil), db.GetCode(dirtyAddr)...)
	if len(cleanCodeBefore) == 0 || len(dirtyCodeBefore) == 0 {
		t.Fatal("expected validator code to be present after finalize")
	}

	if _, err := db.ValidatorWrapper(cleanAddr, true, false); err != nil {
		t.Fatalf("load clean: %v", err)
	}
	dirtyWrapper, err := db.ValidatorWrapper(dirtyAddr, true, false)
	if err != nil {
		t.Fatalf("load dirty: %v", err)
	}
	if _, ok := db.stateValidatorsDirty[cleanAddr]; ok {
		t.Fatal("read-only load marked clean validator dirty")
	}
	if _, ok := db.stateValidatorsDirty[dirtyAddr]; ok {
		t.Fatal("read-only load marked dirty validator dirty")
	}

	dirtyWrapper.BlockReward = big.NewInt(12345)
	db.MarkValidatorWrapperDirty(dirtyAddr)

	db.Finalise(false)

	if !bytes.Equal(db.GetCode(cleanAddr), cleanCodeBefore) {
		t.Fatal("clean validator was re-encoded on finalize")
	}
	if bytes.Equal(db.GetCode(dirtyAddr), dirtyCodeBefore) {
		t.Fatal("dirty validator was not written on finalize")
	}
	if _, ok := db.stateValidatorsDirty[dirtyAddr]; ok {
		t.Fatal("successful finalize left validator dirty")
	}

	got, err := db.ValidatorWrapper(dirtyAddr, true, false)
	if err != nil {
		t.Fatalf("reload dirty: %v", err)
	}
	if got.BlockReward.Cmp(big.NewInt(12345)) != 0 {
		t.Fatalf("dirty BlockReward = %v, want 12345", got.BlockReward)
	}
}

func TestAddRewardMarksValidatorDirty(t *testing.T) {
	db, _ := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()), nil)

	addr := common.BytesToAddress([]byte("validator-reward"))
	wrapper := staketest.GetDefaultValidatorWrapper()
	wrapper.Address = addr
	wrapper.Delegations[0].DelegatorAddress = addr
	if err := db.UpdateValidatorWrapper(addr, &wrapper); err != nil {
		t.Fatalf("UpdateValidatorWrapper: %v", err)
	}
	db.Finalise(false)

	snapshot := staketest.GetDefaultValidatorWrapper()
	snapshot.Address = addr
	snapshot.Delegations[0].DelegatorAddress = addr
	shares := map[common.Address]numeric.Dec{
		addr: numeric.NewDec(1),
	}
	if err := db.AddReward(&snapshot, big.NewInt(1000), shares); err != nil {
		t.Fatalf("AddReward: %v", err)
	}
	if _, ok := db.stateValidatorsDirty[addr]; !ok {
		t.Fatal("AddReward did not mark validator dirty")
	}
}

func TestCopyPreservesValidatorCacheAndDirtyFlags(t *testing.T) {
	db, _ := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()), nil)

	cleanAddr := common.BytesToAddress([]byte("copy-clean"))
	dirtyAddr := common.BytesToAddress([]byte("copy-dirty"))

	clean := staketest.GetDefaultValidatorWrapper()
	clean.Address = cleanAddr
	clean.Delegations[0].DelegatorAddress = cleanAddr
	dirty := staketest.GetDefaultValidatorWrapper()
	dirty.Address = dirtyAddr
	dirty.Delegations[0].DelegatorAddress = dirtyAddr

	if err := db.UpdateValidatorWrapper(cleanAddr, &clean); err != nil {
		t.Fatalf("UpdateValidatorWrapper clean: %v", err)
	}
	if err := db.UpdateValidatorWrapper(dirtyAddr, &dirty); err != nil {
		t.Fatalf("UpdateValidatorWrapper dirty: %v", err)
	}
	db.Finalise(false)

	if _, err := db.ValidatorWrapper(cleanAddr, true, false); err != nil {
		t.Fatalf("load clean: %v", err)
	}
	dirtyWrapper, err := db.ValidatorWrapper(dirtyAddr, true, false)
	if err != nil {
		t.Fatalf("load dirty: %v", err)
	}
	dirtyWrapper.BlockReward = big.NewInt(7)
	db.MarkValidatorWrapperDirty(dirtyAddr)

	copied := db.Copy()
	if _, ok := copied.stateValidators[cleanAddr]; !ok {
		t.Fatal("copy missing clean validator wrapper")
	}
	if _, ok := copied.stateValidators[dirtyAddr]; !ok {
		t.Fatal("copy missing dirty validator wrapper")
	}
	if _, ok := copied.stateValidatorsDirty[dirtyAddr]; !ok {
		t.Fatal("copy missing dirty flag")
	}
	if _, ok := copied.stateValidatorsDirty[cleanAddr]; ok {
		t.Fatal("copy marked clean validator dirty")
	}
	if copied.stateValidators[dirtyAddr].BlockReward.Cmp(big.NewInt(7)) != 0 {
		t.Fatal("copy did not preserve dirty wrapper mutation")
	}
	if copied.stateValidators[dirtyAddr] == db.stateValidators[dirtyAddr] {
		t.Fatal("copy shared wrapper pointer with original")
	}
}

func TestCachedValidatorAddressesSurvivesFinaliseAndCopy(t *testing.T) {
	db, _ := New(common.Hash{}, NewDatabase(rawdb.NewMemoryDatabase()), nil)

	addr := common.BytesToAddress([]byte("cached-validator"))
	wrapper := staketest.GetDefaultValidatorWrapper()
	wrapper.Address = addr
	wrapper.Delegations[0].DelegatorAddress = addr
	db.SetValidatorFlag(addr)
	if err := db.UpdateValidatorWrapper(addr, &wrapper); err != nil {
		t.Fatalf("UpdateValidatorWrapper: %v", err)
	}
	db.Finalise(false)

	addrs := db.CachedValidatorAddresses()
	if len(addrs) != 1 || addrs[0] != addr {
		t.Fatalf("CachedValidatorAddresses after Finalise = %v, want [%s]", addrs, addr.Hex())
	}

	copied := db.Copy()
	copiedAddrs := copied.CachedValidatorAddresses()
	if len(copiedAddrs) != 1 || copiedAddrs[0] != addr {
		t.Fatalf("CachedValidatorAddresses on Copy = %v, want [%s]", copiedAddrs, addr.Hex())
	}
}
