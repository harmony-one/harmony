package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/types"
	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
)

// EventAPI implements the server.EventsAPIServicer interface.
type EventAPI struct {
	hmy *hmy.Harmony
}

func NewEventAPI(hmy *hmy.Harmony) *EventAPI {
	return &EventAPI{hmy: hmy}
}

// EventsBlocks implements the /events/blocks endpoint
func (e *EventAPI) EventsBlocks(ctx context.Context, request *types.EventsBlocksRequest) (resp *types.EventsBlocksResponse, err *types.Error) {
	cacheItem, cacheHelper, cacheErr := rosettaCacheHelper("EventsBlocks", request)
	if cacheErr == nil {
		if cacheItem != nil {
			return cacheItem.resp.(*types.EventsBlocksResponse), nil
		} else {
			defer cacheHelper(resp, err)
		}
	}

	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, e.hmy.ShardID); err != nil {
		return nil, err
	}

	var offset, limit int64

	if request.Limit == nil {
		limit = 10
	} else {
		limit = *request.Limit
		if limit > 1000 {
			limit = 1000
		}
	}

	if request.Offset == nil {
		offset = 0
	} else {
		offset = *request.Offset
	}

	resp = &types.EventsBlocksResponse{
		MaxSequence: e.hmy.BlockChain.CurrentHeader().Number().Int64(),
	}

	for i := offset; i < offset+limit; i++ {
		block := e.hmy.BlockChain.GetBlockByNumber(uint64(i))
		if block == nil {
			break
		}

		resp.Events = append(resp.Events, buildFromBlock(block))
	}

	return resp, nil
}

func buildFromBlock(block *hmyTypes.Block) *types.BlockEvent {
	return &types.BlockEvent{
		Sequence: block.Number().Int64(),
		BlockIdentifier: &types.BlockIdentifier{
			Index: block.Number().Int64(),
			Hash:  block.Hash().Hex(),
		},
		Type: types.ADDED,
	}
}
