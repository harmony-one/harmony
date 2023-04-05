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
	"encoding/binary"
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
)

// ReadDatabaseVersion retrieves the version number of the database.
func ReadDatabaseVersion(db ethdb.KeyValueReader) *uint64 {
	var version uint64

	enc, _ := db.Get(databaseVersionKey)
	if len(enc) == 0 {
		return nil
	}
	if err := rlp.DecodeBytes(enc, &version); err != nil {
		return nil
	}

	return &version
}

// WriteDatabaseVersion stores the version number of the database
func WriteDatabaseVersion(db ethdb.KeyValueWriter, version uint64) {
	enc, err := rlp.EncodeToBytes(version)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to encode database version")
	}
	if err = db.Put(databaseVersionKey, enc); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to store the database version")
	}
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func ReadChainConfig(db ethdb.KeyValueReader, hash common.Hash) *params.ChainConfig {
	data, _ := db.Get(configKey(hash))
	if len(data) == 0 {
		return nil
	}
	var config params.ChainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		utils.Logger().Error().Err(err).Interface("hash", hash).Msg("Invalid chain config JSON")
		return nil
	}
	return &config
}

// WriteChainConfig writes the chain config settings to the database.
func WriteChainConfig(db ethdb.KeyValueWriter, hash common.Hash, cfg *params.ChainConfig) {
	if cfg == nil {
		return
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to JSON encode chain config")
	}
	if err := db.Put(configKey(hash), data); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to store chain config")
	}
}

// ReadGenesisStateSpec retrieves the genesis state specification based on the
// given genesis (block-)hash.
func ReadGenesisStateSpec(db ethdb.KeyValueReader, blockhash common.Hash) []byte {
	data, _ := db.Get(genesisStateSpecKey(blockhash))
	return data
}

// WriteGenesisStateSpec writes the genesis state specification into the disk.
func WriteGenesisStateSpec(db ethdb.KeyValueWriter, blockhash common.Hash, data []byte) {
	if err := db.Put(genesisStateSpecKey(blockhash), data); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to store genesis state")
	}
}

// crashList is a list of unclean-shutdown-markers, for rlp-encoding to the
// database
type crashList struct {
	Discarded uint64   // how many ucs have we deleted
	Recent    []uint64 // unix timestamps of 10 latest unclean shutdowns
}

const crashesToKeep = 10

// PushUncleanShutdownMarker appends a new unclean shutdown marker and returns
// the previous data
// - a list of timestamps
// - a count of how many old unclean-shutdowns have been discarded
// This function is NOT used, just ported over from the Ethereum
func PushUncleanShutdownMarker(db ethdb.KeyValueStore) ([]uint64, uint64, error) {
	var uncleanShutdowns crashList
	// Read old data
	if data, err := db.Get(uncleanShutdownKey); err != nil {
		utils.Logger().Warn().Err(err).Msg("Error reading unclean shutdown markers")
	} else if err := rlp.DecodeBytes(data, &uncleanShutdowns); err != nil {
		return nil, 0, err
	}
	var discarded = uncleanShutdowns.Discarded
	var previous = make([]uint64, len(uncleanShutdowns.Recent))
	copy(previous, uncleanShutdowns.Recent)
	// Add a new (but cap it)
	uncleanShutdowns.Recent = append(uncleanShutdowns.Recent, uint64(time.Now().Unix()))
	if count := len(uncleanShutdowns.Recent); count > crashesToKeep+1 {
		numDel := count - (crashesToKeep + 1)
		uncleanShutdowns.Recent = uncleanShutdowns.Recent[numDel:]
		uncleanShutdowns.Discarded += uint64(numDel)
	}
	// And save it again
	data, _ := rlp.EncodeToBytes(uncleanShutdowns)
	if err := db.Put(uncleanShutdownKey, data); err != nil {
		utils.Logger().Warn().Err(err).Msg("Failed to write unclean-shutdown marker")
		return nil, 0, err
	}
	return previous, discarded, nil
}

// PopUncleanShutdownMarker removes the last unclean shutdown marker
// This function is NOT used, just ported over from the Ethereum
func PopUncleanShutdownMarker(db ethdb.KeyValueStore) {
	var uncleanShutdowns crashList
	// Read old data
	if data, err := db.Get(uncleanShutdownKey); err != nil {
		utils.Logger().Warn().Err(err).Msg("Error reading unclean shutdown markers")
	} else if err := rlp.DecodeBytes(data, &uncleanShutdowns); err != nil {
		utils.Logger().Error().Err(err).Msg("Error decoding unclean shutdown markers") // Should mos def _not_ happen
	}
	if l := len(uncleanShutdowns.Recent); l > 0 {
		uncleanShutdowns.Recent = uncleanShutdowns.Recent[:l-1]
	}
	data, _ := rlp.EncodeToBytes(uncleanShutdowns)
	if err := db.Put(uncleanShutdownKey, data); err != nil {
		utils.Logger().Warn().Err(err).Msg("Failed to clear unclean-shutdown marker")
	}
}

// UpdateUncleanShutdownMarker updates the last marker's timestamp to now.
// This function is NOT used, just ported over from the Ethereum
func UpdateUncleanShutdownMarker(db ethdb.KeyValueStore) {
	var uncleanShutdowns crashList
	// Read old data
	if data, err := db.Get(uncleanShutdownKey); err != nil {
		utils.Logger().Warn().Err(err).Msg("Error reading unclean shutdown markers")
	} else if err := rlp.DecodeBytes(data, &uncleanShutdowns); err != nil {
		utils.Logger().Warn().Err(err).Msg("Error decoding unclean shutdown markers")
	}
	// This shouldn't happen because we push a marker on Backend instantiation
	count := len(uncleanShutdowns.Recent)
	if count == 0 {
		utils.Logger().Warn().Msg("No unclean shutdown marker to update")
		return
	}
	uncleanShutdowns.Recent[count-1] = uint64(time.Now().Unix())
	data, _ := rlp.EncodeToBytes(uncleanShutdowns)
	if err := db.Put(uncleanShutdownKey, data); err != nil {
		utils.Logger().Warn().Err(err).Msg("Failed to write unclean-shutdown marker")
	}
}

// ReadTransitionStatus retrieves the eth2 transition status from the database
// This function is NOT used, just ported over from the Ethereum
func ReadTransitionStatus(db ethdb.KeyValueReader) []byte {
	data, _ := db.Get(transitionStatusKey)
	return data
}

// WriteTransitionStatus stores the eth2 transition status to the database
// This function is NOT used, just ported over from the Ethereum
func WriteTransitionStatus(db ethdb.KeyValueWriter, data []byte) {
	if err := db.Put(transitionStatusKey, data); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to store the eth2 transition status")
	}
}

// WriteLeaderRotationMeta writes the leader continuous blocks count to the database.
func WriteLeaderRotationMeta(db DatabaseWriter, leader []byte, epoch uint64, count, shifts uint64) error {
	if len(leader) != bls.PublicKeySizeInBytes {
		return errors.New("invalid leader public key size")
	}
	value := make([]byte, bls.PublicKeySizeInBytes+8*3)
	copy(value, leader)
	binary.LittleEndian.PutUint64(value[len(leader)+8*0:], epoch)
	binary.LittleEndian.PutUint64(value[len(leader)+8*1:], count)
	binary.LittleEndian.PutUint64(value[len(leader)+8*2:], shifts)
	if err := db.Put(leaderContinuousBlocksCountKey(), value); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to store leader continuous blocks count")
		return err
	}
	return nil
}

// ReadLeaderRotationMeta retrieves the leader continuous blocks count from the database.
func ReadLeaderRotationMeta(db DatabaseReader) (pubKeyBytes []byte, epoch, count, shifts uint64, err error) {
	data, _ := db.Get(leaderContinuousBlocksCountKey())
	if len(data) != bls.PublicKeySizeInBytes+24 {
		return nil, 0, 0, 0, errors.New("invalid leader continuous blocks count")
	}

	pubKeyBytes = data[:bls.PublicKeySizeInBytes]
	epoch = binary.LittleEndian.Uint64(data[bls.PublicKeySizeInBytes:])
	count = binary.LittleEndian.Uint64(data[bls.PublicKeySizeInBytes+8:])
	shifts = binary.LittleEndian.Uint64(data[bls.PublicKeySizeInBytes+16:])
	return pubKeyBytes, epoch, count, shifts, nil
}
