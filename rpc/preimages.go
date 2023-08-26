package rpc

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/utils"
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
				// no 0x prefix, stored as hash
				fmt.Sprintf("%x", asHash.Bytes()),
				// with 0x prefix, stored as bytes
				address.Hex(),
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
	// fetch all the blocks, from start and end both inclusive
	// then execute them - the execution will write the pre-images
	// to disk and we are good to go
	return nil
}