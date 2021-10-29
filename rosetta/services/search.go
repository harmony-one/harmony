package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	internal_common "github.com/harmony-one/harmony/internal/common"
	rosetta_common "github.com/harmony-one/harmony/rosetta/common"
)

// SearchAPI implements the server.SearchAPIServicer interface.
type SearchAPI struct {
	hmy *hmy.Harmony
}

func NewSearchAPI(hmy *hmy.Harmony) *SearchAPI {
	return &SearchAPI{hmy: hmy}
}

// SearchTransactions implements the /search/transactions endpoint
func (s *SearchAPI) SearchTransactions(ctx context.Context, request *types.SearchTransactionsRequest) (resp *types.SearchTransactionsResponse, err *types.Error) {
	cacheItem, cacheHelper, cacheErr := rosettaCacheHelper("SearchTransactions", request)
	if cacheErr == nil {
		if cacheItem != nil {
			return cacheItem.resp.(*types.SearchTransactionsResponse), nil
		} else {
			defer cacheHelper(resp, err)
		}
	}

	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
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

	var filteredHash, rangeHash []common.Hash

	if request.AccountIdentifier != nil {
		ddr, err := internal_common.ParseAddr(request.AccountIdentifier.Address)
		if err != nil {
			return nil, &rosetta_common.ErrCallParametersInvalid
		}

		address, err := internal_common.AddressToBech32(ddr)
		if err != nil {
			return nil, &rosetta_common.ErrCallParametersInvalid
		}

		histories, err := s.hmy.GetTransactionsHistory(address, "", "")
		if err != nil {
			return nil, rosetta_common.NewError(rosetta_common.CatchAllError, map[string]interface{}{
				"message": err.Error(),
			})
		}

		filteredHash = histories
	}

	if request.TransactionIdentifier != nil {
		hash := common.HexToHash(request.TransactionIdentifier.Hash)
		filteredHash = operatorFilter(request.Operator, filteredHash, []common.Hash{hash})
	}

	resp = &types.SearchTransactionsResponse{}
	if int64(len(filteredHash)) < offset {
		return resp, nil
	} else if int64(len(filteredHash)) < offset+limit {
		rangeHash = filteredHash[offset:]
	} else {
		rangeHash = filteredHash[offset : offset+limit]
	}

	for _, hash := range rangeHash {
		tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.hmy.ChainDb(), hash)
		if tx == nil {
			return nil, rosetta_common.NewError(rosetta_common.CatchAllError, map[string]interface{}{
				"message": "can not get tx info by hash",
			})
		}

		info, err := buildFromTXInfo(tx, blockHash, blockNumber, index)
		if err != nil {
			return nil, err
		}

		resp.Transactions = append(resp.Transactions, info)
	}

	resp.TotalCount = int64(len(resp.Transactions))
	if offset+limit < int64(len(filteredHash)) {
		resp.NextOffset = &resp.TotalCount
	}

	return resp, nil
}

func buildFromTXInfo(tx *hmyTypes.Transaction, blockHash common.Hash, blockNumber uint64, index uint64) (*types.BlockTransaction, *types.Error) {
	receiverAccountID, rosettaError := newAccountIdentifier(*tx.To())
	if rosettaError != nil {
		return nil, rosettaError
	}

	var typ string
	if tx.To() == nil {
		typ = rosetta_common.ContractCreationOperation
	} else if tx.ShardID() != tx.ToShardID() {
		typ = rosetta_common.NativeCrossShardTransferOperation
	} else {
		typ = rosetta_common.NativeTransferOperation
	}

	return &types.BlockTransaction{
		BlockIdentifier: &types.BlockIdentifier{
			Index: int64(blockNumber),
			Hash:  blockHash.Hex(),
		},
		Transaction: &types.Transaction{
			TransactionIdentifier: &types.TransactionIdentifier{
				Hash: tx.Hash().String(),
			},
			Operations: []*types.Operation{
				{
					OperationIdentifier: &types.OperationIdentifier{
						Index: int64(index),
					},
					Status:  &rosetta_common.SuccessOperationStatus.Status,
					Type:    typ,
					Account: receiverAccountID,
					Amount: &types.Amount{
						Value:    tx.Value().String(),
						Currency: &rosetta_common.NativeCurrency,
					},
				},
			},
			Metadata: map[string]interface{}{
				"index": index,
				"size":  tx.Size().String(),
			},
		},
	}, nil
}

func operatorFilter(operator *types.Operator, hashesArr ...[]common.Hash) (ret []common.Hash) {
	if len(hashesArr) == 0 {
		return ret
	}

	if operator == nil || *operator == types.AND { // operator is and
		filterMap := make(map[string]common.Hash)
		for _, hash := range hashesArr[0] {
			filterMap[hash.Hex()] = hash
		}

		for _, hashes := range hashesArr[1:] {
			filteredMap := make(map[string]common.Hash)

			for _, hash := range hashes {
				if _, ok := filterMap[hash.Hex()]; ok {
					filteredMap[hash.Hex()] = hash
				}
			}

			filterMap = filteredMap
		}

		for _, hash := range filterMap {
			ret = append(ret, hash)
		}
	} else { // operator is or
		for _, hashes := range hashesArr {
			ret = append(ret, hashes...)
		}
	}

	return ret
}
