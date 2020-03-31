// Copyright 2018 The go-ethereum Authors
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

package rawdb

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/harmony-one/bls/ffi/go/bls"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	testBLSPubKey = "30b2c38b1316da91e068ac3bd8751c0901ef6c02a1d58bc712104918302c6ed03d5894671d0c816dad2b4d303320f202"
	testBLSPrvKey = "c6d7603520311f7a4e6aac0b26701fc433b75b38df504cd416ef2b900cd66205"
)

// Tests that positional lookup metadata can be stored and retrieved.
func TestLookupStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	tx1 := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	tx2 := types.NewTransaction(2, common.BytesToAddress([]byte{0x22}), 0, big.NewInt(222), 2222, big.NewInt(22222), []byte{0x22, 0x22, 0x22})
	tx3 := types.NewTransaction(3, common.BytesToAddress([]byte{0x33}), 0, big.NewInt(333), 3333, big.NewInt(33333), []byte{0x33, 0x33, 0x33})
	txs := []*types.Transaction{tx1, tx2, tx3}

	block := types.NewBlock(blockfactory.NewTestHeader().With().Number(big.NewInt(314)).Header(), txs, types.Receipts{&types.Receipt{}, &types.Receipt{}, &types.Receipt{}}, nil, nil, nil)

	// Check that no transactions entries are in a pristine database
	for i, tx := range txs {
		if txn, _, _, _ := ReadTransaction(db, tx.Hash()); txn != nil {
			t.Fatalf("tx #%d [%x]: non existent transaction returned: %v", i, tx.Hash(), txn)
		}
	}
	// Insert all the transactions into the database, and verify contents
	WriteBlock(db, block)
	WriteTxLookupEntries(db, block)

	for i, tx := range txs {
		if txn, hash, number, index := ReadTransaction(db, tx.Hash()); txn == nil {
			t.Fatalf("tx #%d [%x]: transaction not found", i, tx.Hash())
		} else {
			if hash != block.Hash() || number != block.NumberU64() || index != uint64(i) {
				t.Fatalf("tx #%d [%x]: positional metadata mismatch: have %x/%d/%d, want %x/%v/%v", i, tx.Hash(), hash, number, index, block.Hash(), block.NumberU64(), i)
			}
			if tx.Hash() != txn.Hash() {
				t.Fatalf("tx #%d [%x]: transaction mismatch: have %v, want %v", i, tx.Hash(), txn, tx)
			}
		}
	}
	// Delete the transactions and check purge
	for i, tx := range txs {
		DeleteTxLookupEntry(db, tx.Hash())
		if txn, _, _, _ := ReadTransaction(db, tx.Hash()); txn != nil {
			t.Fatalf("tx #%d [%x]: deleted transaction returned: %v", i, tx.Hash(), txn)
		}
	}
}

// Test that staking tx hash does not find a plain tx hash (and visa versa) within the same block
func TestMixedLookupStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()
	tx := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	key, _ := crypto.GenerateKey()
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		p := &bls.PublicKey{}
		p.DeserializeHexStr(testBLSPubKey)
		pub := shard.BLSPublicKey{}
		pub.FromLibBLSPublicKey(p)
		messageBytes := []byte(staking.BLSVerificationStr)
		privateKey := &bls.SecretKey{}
		privateKey.DeserializeHexStr(testBLSPrvKey)
		msgHash := hash.Keccak256(messageBytes)
		signature := privateKey.SignHash(msgHash[:])
		var sig shard.BLSSignature
		copy(sig[:], signature.Serialize())

		ra, _ := numeric.NewDecFromStr("0.7")
		maxRate, _ := numeric.NewDecFromStr("1")
		maxChangeRate, _ := numeric.NewDecFromStr("0.5")
		return staking.DirectiveCreateValidator, staking.CreateValidator{
			Description: staking.Description{
				Name:            "SuperHero",
				Identity:        "YouWouldNotKnow",
				Website:         "Secret Website",
				SecurityContact: "LicenseToKill",
				Details:         "blah blah blah",
			},
			CommissionRates: staking.CommissionRates{
				Rate:          ra,
				MaxRate:       maxRate,
				MaxChangeRate: maxChangeRate,
			},
			MinSelfDelegation:  big.NewInt(1e18),
			MaxTotalDelegation: big.NewInt(3e18),
			ValidatorAddress:   crypto.PubkeyToAddress(key.PublicKey),
			SlotPubKeys:        []shard.BLSPublicKey{pub},
			SlotKeySigs:        []shard.BLSSignature{sig},
			Amount:             big.NewInt(1e18),
		}
	}
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	txs := []*types.Transaction{tx}
	stxs := []*staking.StakingTransaction{stx}
	header := blockfactory.NewTestHeader().With().Number(big.NewInt(314)).Header()
	block := types.NewBlock(header, txs, types.Receipts{&types.Receipt{}, &types.Receipt{}}, nil, nil, stxs)
	WriteBlock(db, block)
	WriteTxLookupEntries(db, block)

	if recTx, _, _, _ := ReadStakingTransaction(db, tx.Hash()); recTx != nil {
		t.Fatal("got staking transactions with plain tx hash")
	}
	if recTx, _, _, _ := ReadTransaction(db, stx.Hash()); recTx != nil {
		t.Fatal("got plain transactions with staking tx hash")
	}
}
