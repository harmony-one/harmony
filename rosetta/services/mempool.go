package services

import (
	"context"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	ethCommon "github.com/ethereum/go-ethereum/common"
	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/staking"
)

// MempoolAPI implements the server.MempoolAPIServicer interface
type MempoolAPI struct {
	hmy *hmy.Harmony
}

// NewMempoolAPI creates a new instance of MempoolAPI
func NewMempoolAPI(hmy *hmy.Harmony) server.MempoolAPIServicer {
	return &MempoolAPI{
		hmy: hmy,
	}
}

// Mempool ...
func (s *MempoolAPI) Mempool(
	ctx context.Context, req *types.NetworkRequest,
) (*types.MempoolResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(req.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	pool, err := s.hmy.GetPoolTransactions()
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unable to fetch pool transactions",
		})
	}
	txIDs := make([]*types.TransactionIdentifier, pool.Len())
	for i, tx := range pool {
		txIDs[i] = &types.TransactionIdentifier{
			Hash: tx.Hash().String(),
		}
	}
	return &types.MempoolResponse{
		TransactionIdentifiers: txIDs,
	}, nil
}

// MempoolTransaction ...
func (s *MempoolAPI) MempoolTransaction(
	ctx context.Context, req *types.MempoolTransactionRequest,
) (*types.MempoolTransactionResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(req.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	hash := ethCommon.HexToHash(req.TransactionIdentifier.Hash)
	poolTx := s.hmy.GetPoolTransaction(hash)
	if poolTx == nil {
		return nil, &common.TransactionNotFoundError
	}

	var nilAddress ethCommon.Address
	senderAddr, _ := poolTx.SenderAddress()
	estLog := &hmyTypes.Log{
		Address:     senderAddr,
		Topics:      []ethCommon.Hash{staking.CollectRewardsTopic},
		Data:        big.NewInt(0).Bytes(),
		BlockNumber: s.hmy.CurrentBlock().NumberU64(),
	}

	estReceipt := &hmyTypes.Receipt{
		PostState:         []byte{},
		Status:            hmyTypes.ReceiptStatusSuccessful, // Assume transaction will succeed
		CumulativeGasUsed: poolTx.Gas(),
		Bloom:             [256]byte{},
		Logs:              []*hmyTypes.Log{estLog},
		TxHash:            poolTx.Hash(),
		ContractAddress:   nilAddress, // ContractAddress is only for smart contract creation & can not be determined until transaction is finalized
		GasUsed:           poolTx.Gas(),
	}

	respTx, err := formatTransaction(poolTx, estReceipt)
	if err != nil {
		return nil, err
	}

	return &types.MempoolTransactionResponse{
		Transaction: respTx,
	}, nil
}
