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

func (s *PreimagesService) Export(ctx context.Context, path string) error {
	// these are by default not blocking
	return core.ExportPreimages(s.hmy.BlockChain, path)
}
func (s *PreimagesService) Generate(ctx context.Context, start, end uint64) error {
	if number := s.hmy.CurrentBlock().NumberU64(); number > end {
		fmt.Printf(
			"Cropping generate endpoint from %d to %d\n",
			end, number,
		)
		end = number
	}
	// these are by default not blocking
	return core.GeneratePreimages(s.hmy.BlockChain, start, end)
}
