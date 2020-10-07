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
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
)

// ReadDatabaseVersion retrieves the version number of the database.
func ReadDatabaseVersion(db DatabaseReader) int {
	var version int

	enc, _ := db.Get(databaseVerisionKey)
	rlp.DecodeBytes(enc, &version)

	return version
}

// WriteDatabaseVersion stores the version number of the database
func WriteDatabaseVersion(db DatabaseWriter, version int) error {
	enc, _ := rlp.EncodeToBytes(version)
	if err := db.Put(databaseVerisionKey, enc); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to store the database version")
		return err
	}
	return nil
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func ReadChainConfig(db DatabaseReader, hash common.Hash) *params.ChainConfig {
	data, _ := db.Get(configKey(hash))
	if len(data) == 0 {
		return nil
	}
	var config params.ChainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		utils.Logger().Error().Err(err).Str("hash", hash.Hex()).Msg("Invalid chain config JSON")
		return nil
	}
	return &config
}

// WriteChainConfig writes the chain config settings to the database.
func WriteChainConfig(db DatabaseWriter, hash common.Hash, cfg *params.ChainConfig) error {
	if cfg == nil {
		return errors.New("nil config")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to JSON encode chain config")
		return err
	}
	if err := db.Put(configKey(hash), data); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to store chain config")
		return err
	}
	return nil
}

// ReadPreimage retrieves a single preimage of the provided hash.
func ReadPreimage(db DatabaseReader, hash common.Hash) []byte {
	data, _ := db.Get(preimageKey(hash))
	return data
}

// WritePreimages writes the provided set of preimages to the database. `number` is the
// current block number, and is used for debug messages only.
func WritePreimages(db DatabaseWriter, number uint64, preimages map[common.Hash][]byte) error {
	for hash, preimage := range preimages {
		if err := db.Put(preimageKey(hash), preimage); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to store trie preimage")
			return err
		}
	}
	preimageCounter.Inc(int64(len(preimages)))
	preimageHitCounter.Inc(int64(len(preimages)))
	return nil
}
