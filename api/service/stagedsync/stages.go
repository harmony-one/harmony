// Copyright 2020 The Erigon Authors
// This file is part of the Erigon library.
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

// Get Max Heights
// Build Sync Matrix
// ProcessStateSync
//
// Consensus Last Mile
//

package stagedsync

import (
	"encoding/binary"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/kv"
)

// SyncStageID represents the stages of syncronisation in the Mode.StagedSync mode
// It is used to persist the information about the stage state into the database.
// It should not be empty and should be unique.
type SyncStageID string

var (
	Headers     SyncStageID = "Headers"     // Headers are downloaded
	BlockHashes SyncStageID = "BlockHashes" // Headers hashes are downloaded from peers
	TasksQueue  SyncStageID = "TasksQueue"  // Generate Tasks Queue
	Bodies      SyncStageID = "Bodies"      // Block bodies are downloaded, TxHash and UncleHash are getting verified
	States      SyncStageID = "States"      // will construct most recent state from downloaded blocks
	LastMile    SyncStageID = "LastMile"    // update blocks after sync and update last mile blocks as well
	Finish      SyncStageID = "Finish"      // Nominal stage after all other stages
)

var AllStages = []SyncStageID{
	Headers,
	BlockHashes,
	TasksQueue,
	Bodies,
	States,
	LastMile,
	Finish,
}

func GetStageID(stage SyncStageID, isBeacon bool, prune bool) []byte {
	name := stage
	if isBeacon {
		name = "beacon_" + name
	}
	if prune {
		name = "prune_" + name
	}
	return []byte(name)
}

// GetStageProgress retrieves saved progress of given sync stage from the database
func GetStageProgress(db kv.Getter, stage SyncStageID, isBeacon bool) (uint64, error) {

	v, err := db.GetOne(kv.SyncStageProgress, GetStageID(stage, isBeacon, false))
	if err != nil {
		return 0, err
	}
	return unmarshalData(v)
}

func SaveStageProgress(db kv.Putter, stage SyncStageID, isBeacon bool, progress uint64) error {
	return db.Put(kv.SyncStageProgress, GetStageID(stage, isBeacon, false), marshalData(progress))
}

// GetStagePruneProgress retrieves saved progress of given sync stage from the database
func GetStagePruneProgress(db kv.Getter, stage SyncStageID, isBeacon bool) (uint64, error) {
	v, err := db.GetOne(kv.SyncStageProgress, GetStageID(stage, isBeacon, true))
	if err != nil {
		return 0, err
	}
	return unmarshalData(v)
}

func SaveStagePruneProgress(db kv.Putter, stage SyncStageID, isBeacon bool, progress uint64) error {
	return db.Put(kv.SyncStageProgress, GetStageID(stage, isBeacon, true), marshalData(progress))
}

func marshalData(blockNumber uint64) []byte {
	return encodeBigEndian(blockNumber)
}

func unmarshalData(data []byte) (uint64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if len(data) < 8 {
		return 0, fmt.Errorf("value must be at least 8 bytes, got %d", len(data))
	}
	return binary.BigEndian.Uint64(data[:8]), nil
}

func encodeBigEndian(n uint64) []byte {
	var v [8]byte
	binary.BigEndian.PutUint64(v[:], n)
	return v[:]
}
