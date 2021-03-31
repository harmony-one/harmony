package services

import (
	"context"
	"fmt"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"

	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	internalCommon "github.com/harmony-one/harmony/internal/common"
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

// AccountBalance implements the /account/balance endpoint
func (s *AccountAPI) AccountBalance(
	ctx context.Context, request *types.AccountBalanceRequest,
) (*types.AccountBalanceResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	var block *hmyTypes.Block
	var rosettaError *types.Error
	if request.BlockIdentifier == nil {
		block = s.hmy.CurrentBlock()
	} else {
		block, rosettaError = getBlock(ctx, s.hmy, request.BlockIdentifier)
		if rosettaError != nil {
			return nil, rosettaError
		}
	}

	addr, err := getAddress(request.AccountIdentifier)
	if err != nil {
		return nil, common.NewError(common.SanityCheckError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	blockNum := rpc.BlockNumber(block.Header().Header.Number().Int64())
	balance := new(big.Int)

	// delegated balance
	if request.AccountIdentifier.SubAccount != nil {
		subAccount := request.AccountIdentifier.SubAccount
		ty, exist := subAccount.Metadata["type"]

		if exist && ty.(string) == "delegation" {
			validatorAddr := subAccount.Address
			validators, delegations := s.hmy.GetDelegationsByDelegatorByBlock(addr, block)
			for index, validator := range validators {
				if validatorAddr == internalCommon.MustAddressToBech32(validator) {
					balance = new(big.Int).Add(balance, delegations[index].Amount)
				}
			}
			// pending undelegated balance
		} else if exist && ty.(string) == "undelegation" {
			validatorAddr := subAccount.Address
			validators, delegations := s.hmy.GetDelegationsByDelegatorByBlock(addr, block)
			for index, validator := range validators {
				if validatorAddr == internalCommon.MustAddressToBech32(validator) {
					undelegations := delegations[index].Undelegations
					for _, undelegate := range undelegations {
						balance = new(big.Int).Add(balance, undelegate.Amount)
					}
				}
			}
		} else {
			return nil, common.NewError(common.SanityCheckError, map[string]interface{}{
				"message": "invalid sub account or type",
			})
		}
	} else {
		balance, err = s.hmy.GetBalance(ctx, addr, blockNum)
		if err != nil {
			return nil, common.NewError(common.SanityCheckError, map[string]interface{}{
				"message": "invalid address",
			})
		}
	}

	amount := types.Amount{
		Value:    balance.String(),
		Currency: &common.NativeCurrency,
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

// AccountMetadata used for account identifiers
type AccountMetadata struct {
	Address string `json:"hex_address"`
}

// newAccountIdentifier ..
func newAccountIdentifier(
	address ethCommon.Address,
) (*types.AccountIdentifier, *types.Error) {
	b32Address, err := internalCommon.AddressToBech32(address)
	if err != nil {
		return nil, common.NewError(common.SanityCheckError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	metadata, err := types.MarshalMap(AccountMetadata{Address: address.String()})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	return &types.AccountIdentifier{
		Address:  b32Address,
		Metadata: metadata,
	}, nil
}

// getAddress ..
func getAddress(
	identifier *types.AccountIdentifier,
) (ethCommon.Address, error) {
	if identifier == nil {
		return ethCommon.Address{}, fmt.Errorf("identifier cannot be nil")
	}
	return internalCommon.Bech32ToAddress(identifier.Address)
}
