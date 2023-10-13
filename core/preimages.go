package core

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
)

// ImportPreimages is public so `main.go` can call it directly`
func ImportPreimages(chain BlockChain, path string) error {
	reader, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open file for reading: %s", err)
	}
	csvReader := csv.NewReader(reader)
	dbReader := chain.ChainDb()
	imported := uint64(0)
	for {
		record, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("MyBlockNumber field missing, cannot proceed")
		}
		if err != nil {
			return fmt.Errorf("could not read from reader: %s", err)
		}
		// this means the address is a number
		if blockNumber, err := strconv.ParseUint(record[1], 10, 64); err == nil {
			if record[0] == "MyBlockNumber" {
				// set this value in database, and prometheus, if needed
				prev, err := rawdb.ReadPreimageImportBlock(dbReader)
				if err != nil {
					if !errors.Is(err, leveldb.ErrNotFound) {
						return fmt.Errorf("no prior value found, overwriting: %s", err)
					}
				}
				if blockNumber > prev {
					if rawdb.WritePreimageImportBlock(dbReader, blockNumber) != nil {
						return fmt.Errorf("error saving last import block: %s", err)
					}
				}
				// this is the last record
				imported = blockNumber
				break
			}
		}
		key := ethCommon.HexToHash(record[0])
		value := ethCommon.Hex2Bytes(record[1])
		// validate
		if crypto.Keccak256Hash(value) != key {
			fmt.Println("Data mismatch: skipping", record)
			continue
		}
		// add to database
		_ = rawdb.WritePreimages(
			dbReader, map[ethCommon.Hash][]byte{
				key: value,
			},
		)
	}
	// now, at this point, we will have to generate missing pre-images
	if imported != 0 {
		genStart, _ := rawdb.ReadPreImageStartBlock(dbReader)
		genEnd, _ := rawdb.ReadPreImageEndBlock(dbReader)
		current := chain.CurrentBlock().NumberU64()
		toGenStart, toGenEnd := FindMissingRange(imported, genStart, genEnd, current)
		if toGenStart != 0 && toGenEnd != 0 {
			if err := GeneratePreimages(
				chain, toGenStart, toGenEnd,
			); err != nil {
				return fmt.Errorf("error generating: %s", err)
			}
		}
	}

	return nil
}

// ExportPreimages is public so `main.go` can call it directly`
func ExportPreimages(chain BlockChain, path string) error {
	if err := chain.CommitPreimages(); err != nil {
		return fmt.Errorf("unable to commit preimages: %w", err)
	}
	// set up csv
	writer, err := os.Create(path)
	if err != nil {
		utils.Logger().Error().
			Msgf("unable to create file at %s due to %s", path, err)
		return fmt.Errorf(
			"unable to create file at %s due to %s",
			path, err,
		)
	}
	csvWriter := csv.NewWriter(writer)
	// open trie
	block := chain.CurrentBlock()
	statedb, err := chain.StateAt(block.Root())
	if err != nil {
		utils.Logger().Error().
			Msgf(
				"unable to open statedb at %s due to %s",
				block.Root(), err,
			)
		return fmt.Errorf(
			"unable to open statedb at %x due to %s",
			block.Root(), err,
		)
	}
	trie, err := statedb.Database().OpenTrie(
		block.Root(),
	)
	if err != nil {
		utils.Logger().Error().
			Msgf(
				"unable to open trie at %x due to %s",
				block.Root(), err,
			)
		return fmt.Errorf(
			"unable to open trie at %x due to %s",
			block.Root(), err,
		)
	}
	accountIterator := trie.NodeIterator(nil)
	dbReader := chain.ChainDb()
	for accountIterator.Next(true) {
		// the leaf nodes of the MPT represent accounts
		if !accountIterator.Leaf() {
			continue
		}
		// the leaf key is the hashed address
		hashed := accountIterator.LeafKey()
		asHash := ethCommon.BytesToHash(hashed)
		// obtain the corresponding address
		preimage := rawdb.ReadPreimage(
			dbReader, asHash,
		)
		if len(preimage) == 0 {
			utils.Logger().Warn().
				Msgf("Address not found for %x", asHash)
			continue
		}
		address := ethCommon.BytesToAddress(preimage)
		// key value format, so hash of value is first
		csvWriter.Write([]string{
			fmt.Sprintf("%x", asHash.Bytes()),
			fmt.Sprintf("%x", address.Bytes()),
		})
	}
	// lastly, write the block number
	csvWriter.Write(
		[]string{
			"MyBlockNumber",
			block.Number().String(),
		},
	)
	// to disk
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		utils.Logger().Error().
			Msgf("unable to write csv due to %s", err)
		return fmt.Errorf("unable to write csv due to %s", err)
	}
	writer.Close()
	return nil
}

func GeneratePreimages(chain BlockChain, start, end uint64) error {
	if start < 2 {
		return fmt.Errorf("too low starting point %d", start)
	}
	fmt.Println("generating from", start, "to", end)

	// fetch all the blocks, from start and end both inclusive
	// then execute them - the execution will write the pre-images
	// to disk and we are good to go

	// attempt to find a block number for which we have block and state
	// with number < start
	var startingState *state.DB
	var startingBlock *types.Block
	for i := start - 1; i > 0; i-- {
		fmt.Println("finding block number", i)
		startingBlock = chain.GetBlockByNumber(i)
		if startingBlock == nil {
			fmt.Println("not found block number", i)
			// rewound too much in snapdb, so exit loop
			// although this is only designed for s2/s3 nodes in mind
			// which do not have such a snapdb
			break
		}
		fmt.Println("found block number", startingBlock.NumberU64(), startingBlock.Root().Hex())
		stateAt, err := chain.StateAt(startingBlock.Root())
		if err != nil {
			continue
		}
		startingState = stateAt
		break
	}
	if startingBlock == nil || startingState == nil {
		return fmt.Errorf("no eligible starting block with state found")
	}

	var endingState *state.DB
	var errProcess error
	// now execute block T+1 based on starting state
	for i := startingBlock.NumberU64() + 1; i <= end; i++ {
		if i%10000 == 0 {
			fmt.Println("processing block", i)
		}
		block := chain.GetBlockByNumber(i)
		if block == nil {
			// because we have startingBlock we must have all following
			return fmt.Errorf("block %d not found", i)
		}
		stateAt, _ := chain.StateAt(block.Root())
		_, _, _, _, _, _, endingState, errProcess = chain.Processor().Process(block, startingState, *chain.GetVMConfig(), false)
		if errProcess != nil {
			return fmt.Errorf("error executing block #%d: %s", i, errProcess)
		}

		if stateAt != nil {
			if root, err := endingState.Commit(false); err != nil {
				return fmt.Errorf("unable to commit state for block '%d': %w", i, err)
			} else if root.Hex() != block.Root().Hex() {
				return fmt.Errorf("block root hashes different after commit commitRoot='%s' blockRoot='%s'", root.Hex(), block.Root().Hex())
			}

			if err := chain.CommitPreimages(); err != nil {
				return fmt.Errorf("error committing preimages for block '%d': %w", i, err)
			}
			startingState = stateAt
		} else {
			startingState = endingState
		}
	}
	// force any pre-images in memory so far to go to disk, if they haven't already
	fmt.Println("committing images")
	if endingState != nil {
		if _, err := endingState.Commit(false); err != nil {
			return fmt.Errorf("unable to commit state for block: %w", err)
		}
	}
	if err := chain.CommitPreimages(); err != nil {
		return fmt.Errorf("error committing preimages %s", err)
	}

	if _, _, err := rawdb.WritePreImageStartEndBlock(chain.ChainDb(), startingBlock.NumberU64(), end); err != nil {
		return fmt.Errorf("error writing pre-image gen blocks %s", err)
	}
	return nil
}
func FindMissingRange(
	imported, start, end, current uint64,
) (uint64, uint64) {
	// both are unset
	if start == 0 && end == 0 {
		if imported < current {
			return imported + 1, current
		} else {
			return 0, 0
		}
	}
	// constraints: start <= end <= current
	// in regular usage, we should have end == current
	// however, with the GenerateFlag usage, we can have end < current
	check1 := start <= end
	if !check1 {
		panic("Start > End")
	}
	check2 := end <= current
	if !check2 {
		panic("End > Current")
	}
	// imported can sit in any of the 4 ranges
	if imported < start {
		// both inclusive
		return imported + 1, start - 1
	}
	if imported < end {
		return end + 1, current
	}
	if imported < current {
		return imported + 1, current
	}
	// future data imported
	if current < imported {
		return 0, 0
	}
	return 0, 0
}

func VerifyPreimages(header *block.Header, chain BlockChain) (uint64, error) {
	var existingPreimages uint64
	parentRoot := chain.GetBlockByHash(
		header.ParentHash(),
	).Root() // for examining MPT at this root, should exist

	db, err := chain.StateAt(parentRoot)
	if err != nil {
		return 0, err
	}

	trie, err := db.Database().OpenTrie(parentRoot)
	if err != nil {
		return 0, err
	}

	if err := chain.CommitPreimages(); err != nil {
		return 0, fmt.Errorf("unable to commit preimages: %w", err)
	}

	diskDB := db.Database().DiskDB()
	// start the iteration
	accountIterator := trie.NodeIterator(nil)
	for accountIterator.Next(true) {
		// leaf means leaf node of the MPT, which is an account
		// the leaf key is the address
		if accountIterator.Leaf() {
			key := accountIterator.LeafKey()
			preimage := rawdb.ReadPreimage(diskDB, common.BytesToHash(key))
			if len(preimage) == 0 {
				err := errors.New(
					fmt.Sprintf(
						"cannot find preimage for %x after '%d' accounts", key, existingPreimages,
					),
				)
				utils.Logger().Warn().Msg(err.Error())
				return existingPreimages, err
			}
			address := common.BytesToAddress(preimage)
			// skip blank address
			if address == (common.Address{}) {
				continue
			}

			existingPreimages++
		}
	}

	return existingPreimages, nil
}

func WritePreimagesMetricsIntoPrometheus(chain BlockChain, sendMetrics func(preimageStart, preimageEnd, lastPreimageImport, verifiedAddresses uint64, shard uint32)) {
	shardID := chain.ShardID()
	if shardID < 2 {
		return
	}

	ticker := time.NewTicker(time.Minute * 5)
	dbReader := chain.ChainDb()

	for {
		select {
		case <-ticker.C:
			lastImport, _ := rawdb.ReadPreimageImportBlock(dbReader)
			startBlock, _ := rawdb.ReadPreImageStartBlock(dbReader)
			endBlock, _ := rawdb.ReadPreImageEndBlock(dbReader)

			chain.CurrentBlock().NumberU64()
			verify, _ := VerifyPreimages(chain.CurrentBlock().Header(), chain)

			sendMetrics(startBlock, endBlock, lastImport, verify, shardID)
		}
	}
}
