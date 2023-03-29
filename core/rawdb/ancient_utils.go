// Copyright 2022 The go-ethereum Authors
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
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

type tableSize struct {
	name string
	size common.StorageSize
}

// freezerInfo contains the basic information of the freezer.
type freezerInfo struct {
	name  string      // The identifier of freezer
	head  uint64      // The number of last stored item in the freezer
	tail  uint64      // The number of first stored item in the freezer
	sizes []tableSize // The storage size per table
}

// count returns the number of stored items in the freezer.
func (info *freezerInfo) count() uint64 {
	return info.head - info.tail + 1
}

// size returns the storage size of the entire freezer.
func (info *freezerInfo) size() common.StorageSize {
	var total common.StorageSize
	for _, table := range info.sizes {
		total += table.size
	}
	return total
}

// inspectFreezers inspects all freezers registered in the system.
// This function is NOT used, just ported over from the Ethereum
func inspectFreezers(db ethdb.Database) ([]freezerInfo, error) {
	var infos []freezerInfo
	for _, freezer := range freezers {
		switch freezer {
		case chainFreezerName:
			// Chain ancient store is a bit special. It's always opened along
			// with the key-value store, inspect the chain store directly.
			info := freezerInfo{name: freezer}
			// Retrieve storage size of every contained table.
			for table := range chainFreezerNoSnappy {
				size, err := db.AncientSize(table)
				if err != nil {
					return nil, err
				}
				info.sizes = append(info.sizes, tableSize{name: table, size: common.StorageSize(size)})
			}
			// Retrieve the number of last stored item
			ancients, err := db.Ancients()
			if err != nil {
				return nil, err
			}
			info.head = ancients - 1

			// Retrieve the number of first stored item
			tail, err := db.Tail()
			if err != nil {
				return nil, err
			}
			info.tail = tail
			infos = append(infos, info)

		default:
			return nil, fmt.Errorf("unknown freezer, supported ones: %v", freezers)
		}
	}
	return infos, nil
}
