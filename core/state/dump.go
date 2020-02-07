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

package state

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	common2 "github.com/harmony-one/harmony/internal/common"
	staking "github.com/harmony-one/harmony/staking/types"
)

// DumpAccount ...
type DumpAccount struct {
	Balance  string            `json:"balance"`
	Nonce    uint64            `json:"nonce"`
	Root     string            `json:"root"`
	CodeHash string            `json:"codeHash"`
	Code     string            `json:"code"`
	Storage  map[string]string `json:"storage"`
}

// Dump ...
type Dump struct {
	Root     string                 `json:"root"`
	Accounts map[string]DumpAccount `json:"accounts"`
}

// RawDump ...
func (db *DB) RawDump() Dump {
	dump := Dump{
		Root:     fmt.Sprintf("%x", db.trie.Hash()),
		Accounts: make(map[string]DumpAccount),
	}

	it := trie.NewIterator(db.trie.NodeIterator(nil))
	for it.Next() {
		addr := db.trie.GetKey(it.Key)
		var data Account
		if err := rlp.DecodeBytes(it.Value, &data); err != nil {
			panic(err)
		}
		obj := newObject(nil, common.BytesToAddress(addr), data)
		var wrapper staking.ValidatorWrapper
		wrap := ""
		if err := rlp.DecodeBytes(obj.Code(db.db), &wrapper); err != nil {
			//
		} else {
			marsh, err := json.Marshal(wrapper)
			if err == nil {
				wrap = string(marsh)
			}
		}

		account := DumpAccount{
			Balance:  data.Balance.String(),
			Nonce:    data.Nonce,
			Root:     common.Bytes2Hex(data.Root[:]),
			CodeHash: common.Bytes2Hex(data.CodeHash),
			Code:     wrap,
			Storage:  make(map[string]string),
		}
		storageIt := trie.NewIterator(obj.getTrie(db.db).NodeIterator(nil))
		for storageIt.Next() {
			account.Storage[common.Bytes2Hex(db.trie.GetKey(storageIt.Key))] = common.Bytes2Hex(storageIt.Value)
		}
		dump.Accounts[common2.MustAddressToBech32(common.BytesToAddress(addr))] = account
	}
	return dump
}

// Dump ...
func (db *DB) Dump() string {
	json, err := json.MarshalIndent(db.RawDump(), "", "    ")
	if err != nil {
		fmt.Println("dump err", err)
	}

	return string(json)
}
