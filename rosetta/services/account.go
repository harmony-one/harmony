package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"

	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/rosetta/common"
)

// AccountAPI implements the server.AccountAPIServicer interface.
type AccountAPI struct {
	hmy *hmy.Harmony
}

// NewAccountAPI creates a new instance of a BlockAPI.
func NewAccountAPI(hmy *hmy.Harmony) server.AccountAPIServicer {
	return &AccountAPI{
		hmy: hmy,
	}
}

// AccountBalance ...
func (s *AccountAPI) AccountBalance(
	ctx context.Context, request *types.AccountBalanceRequest,
) (*types.AccountBalanceResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	var block *hmyTypes.Block
	if request.BlockIdentifier == nil {
		block = s.hmy.CurrentBlock()
	} else {
		var err error
		if request.BlockIdentifier.Hash != nil {
			blockHash := ethCommon.HexToHash(*request.BlockIdentifier.Hash)
			block, err = s.hmy.GetBlock(ctx, blockHash)
			if err != nil {
				return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
					"message": "block hash not found",
				})
			}
		} else {
			blockNum := rpc.BlockNumber(*request.BlockIdentifier.Index)
			block, err = s.hmy.BlockByNumber(ctx, blockNum)
			if err != nil {
				return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
					"message": "block index not found",
				})
			}
		}
	}

	addr, err := getAddress(request.AccountIdentifier)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	blockNum := rpc.BlockNumber(block.Header().Header.Number().Int64())
	balance, err := s.hmy.GetBalance(ctx, addr, blockNum)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "invalid address",
		})
	}

	amount := types.Amount{
		Value:    balance.String(),
		Currency: &common.Currency,
	}

	respBlock := types.BlockIdentifier{
		Index: blockNum.Int64(),
		Hash:  block.Header().Hash().String(),
	}

	return &types.AccountBalanceResponse{
		BlockIdentifier: &respBlock,
		Balances:        []*types.Amount{&amount},
	}, nil
}
