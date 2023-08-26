package rpc

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/api/service/prometheus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/utils"
	prom "github.com/prometheus/client_golang/prometheus"
)

type PreimagesService struct {
	hmy *hmy.Harmony
}

// NewPreimagesAPI creates a new API for the RPC interface
func NewPreimagesAPI(hmy *hmy.Harmony, version string) rpc.API {
	var service interface{} = &PreimagesService{hmy}
	return rpc.API{
		Namespace: version,
		Version:   APIVersion,
		Service:   service,
		Public:    true,
	}
}

func (s *PreimagesService) Export(ctx context.Context, path string) error {
	// these are by default not blocking
	return ExportPreimages(s.hmy.BlockChain, path)
}

// ExportPreimages is public so `main.go` can call it directly`
func ExportPreimages(chain core.BlockChain, path string) error {
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
		if accountIterator.Leaf() {
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

func GeneratePreimages(chain core.BlockChain, start, end uint64) error {
	if start < 2 {
		return fmt.Errorf("too low starting point %d", start)
	}
	// fetch all the blocks, from start and end both inclusive
	// then execute them - the execution will write the pre-images
	// to disk and we are good to go

	// attempt to find a block number for which we have block and state
	// with number < start
	var startingState *state.DB
	var startingBlock *types.Block
	for i := start - 1; i > 0; i-- {
		startingBlock = chain.GetBlockByNumber(i)
		if startingBlock == nil {
			// rewound too much in snapdb, so exit loop
			// although this is only designed for s2/s3 nodes in mind
			// which do not have such a snapdb
			break
		}
		state, err := chain.StateAt(startingBlock.Root())
		if err == nil {
			continue
		}
		startingState = state
		break
	}
	if startingBlock == nil || startingState == nil {
		return fmt.Errorf("no eligible starting block with state found")
	}
	
	// now execute block T+1 based on starting state
	for i := startingBlock.NumberU64() + 1; i <= end; i++ {
		block := chain.GetBlockByNumber(i)
		if block == nil {
			// because we have startingBlock we must have all following
			return fmt.Errorf("block %d not found", i)
		}
		_, _, _, _, _, _, endingState, err := chain.Processor().Process(block, startingState, *chain.GetVMConfig(), false)
		if err == nil {
			return fmt.Errorf("error executing block #%d: %s", i, err)
		}
		startingState = endingState
	}
	// force any pre-images in memory so far to go to disk, if they haven't already
	if err := chain.CommitPreimages(); err != nil {
		return fmt.Errorf("error committing preimages %s", err)
	}
	// save information about generated pre-images start and end nbs
	toWrite := []uint64{0, 0}
	existingStart, err := rawdb.ReadPreImageStartBlock(chain.ChainDb())
	if err != nil || existingStart > startingBlock.NumberU64() + 1 {
		toWrite[0] = startingBlock.NumberU64() + 1
	} else {
		toWrite[0] = existingStart
	}
	existingEnd, err := rawdb.ReadPreImageEndBlock(chain.ChainDb())
	if err != nil || existingEnd < end {
		toWrite[1] = end
	} else {
		toWrite[1] = existingEnd
	}
	if err := rawdb.WritePreImageStartEndBlock(chain.ChainDb(), toWrite[0], toWrite[1]); err != nil {
		return fmt.Errorf("error writing pre-image gen blocks %s", err)
	}
	// add prometheus metrics as well
	startGauge := prom.NewGauge(
		prom.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "blockchain",
			Name:      "preimage_start",
			Help:      "the first block for which pre-image generation ran locally",
		},
	)
	endGauge := prom.NewGauge(
		prom.GaugeOpts{
			Namespace: "hmy",
			Subsystem: "blockchain",
			Name:      "preimage_end",
			Help:      "the last block for which pre-image generation ran locally",
		},
	)
	prometheus.PromRegistry().MustRegister(
		startGauge, endGauge,
	)
	startGauge.Set(float64(toWrite[0]))
	endGauge.Set(float64(toWrite[1]))
	return nil
}
