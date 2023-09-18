package rpc

import (
	"context"
	"fmt"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
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

func (s *PreimagesService) Export(_ context.Context, path string) error {
	// these are by default not blocking
	return core.ExportPreimages(s.hmy.BlockChain, path)
}

func (s *PreimagesService) Import(_ context.Context, path string) error {
	// these are by default not blocking
	return core.ImportPreimages(s.hmy.BlockChain, path)
}
func (s *PreimagesService) Generate(_ context.Context, start, end rpc.BlockNumber) error {
	// earliestBlock: the number of blocks in the past where you can generate the preimage from the last block
	earliestBlock := uint64(2)
	currentBlockNum := s.hmy.CurrentBlock().NumberU64()

	var startBlock uint64
	switch start {
	case rpc.EarliestBlockNumber:
		startBlock = earliestBlock
	case rpc.LatestBlockNumber:
		startBlock = earliestBlock
	case rpc.PendingBlockNumber:
		startBlock = earliestBlock
	default:
		startBlock = uint64(start)
	}

	var endBlock = uint64(end)
	switch end {
	case rpc.EarliestBlockNumber:
		endBlock = currentBlockNum
	case rpc.LatestBlockNumber:
		endBlock = currentBlockNum
	case rpc.PendingBlockNumber:
		endBlock = currentBlockNum
	default:
		endBlock = uint64(end)
	}

	fmt.Printf("Generating preimage from block %d to %d\n", startBlock, endBlock)

	if number := currentBlockNum; number > endBlock {
		fmt.Printf(
			"Cropping generate endpoint from %d to %d\n",
			endBlock, number,
		)
		endBlock = number
	}

	if startBlock >= endBlock {
		fmt.Printf(
			"Cropping generate startpoint from %d to %d\n",
			startBlock, endBlock-earliestBlock,
		)
		startBlock = endBlock - earliestBlock
	}

	// these are by default not blocking
	return core.GeneratePreimages(s.hmy.BlockChain, startBlock, endBlock)
}
