package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/core/vm"
	ethRpc "github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/rpc"
)

// ConstructMetadataOptions is constructed by ConstructionPreprocess for ConstructionMetadata options
type ConstructMetadataOptions struct {
	TransactionMetadata *TransactionMetadata `json:"transaction_metadata"`
	OperationType       string               `json:"operation_type,omitempty"`
	GasPriceMultiplier  *float64             `json:"gas_price_multiplier,omitempty"`
}

// UnmarshalFromInterface ..
func (m *ConstructMetadataOptions) UnmarshalFromInterface(metadata interface{}) error {
	var T ConstructMetadataOptions
	dat, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dat, &T); err != nil {
		return err
	}
	if T.TransactionMetadata == nil {
		return fmt.Errorf("transaction metadata is required")
	}
	if T.OperationType == "" {
		return fmt.Errorf("operation type is required")
	}
	*m = T
	return nil
}

// ConstructionPreprocess implements the /construction/preprocess endpoint.
// Note that `request.MaxFee` is never considered for this construction implementation.
func (s *ConstructAPI) ConstructionPreprocess(
	ctx context.Context, request *types.ConstructionPreprocessRequest,
) (*types.ConstructionPreprocessResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	txMetadata := &TransactionMetadata{}
	if request.Metadata != nil {
		if err := txMetadata.UnmarshalFromInterface(request.Metadata); err != nil {
			return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "invalid transaction metadata").Error(),
			})
		}
	}
	if txMetadata.FromShardID != nil && *txMetadata.FromShardID != s.hmy.ShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": fmt.Sprintf("expect from shard ID to be %v", s.hmy.ShardID),
		})
	}

	components, rosettaError := GetOperationComponents(request.Operations)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if components.From == nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "sender address is not found for given operations",
		})
	}
	if request.Operations[0].Type == common.CreateValidatorOperation && len(txMetadata.SlotPubKeys) == 0 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "invalid slot public keys",
		})
	}
	if request.Operations[0].Type == common.CreateValidatorOperation && len(txMetadata.SlotKeySigs) == 0 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "invalid slot key signatures",
		})
	}
	if request.Operations[0].Type == common.EditValidatorOperation && (txMetadata.SlotPubKeyToAdd == "" ||
		txMetadata.SlotPubKeyToRemove == "" || txMetadata.SlotKeyToAddSig == "") {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "slot pub key to add/remove or sig to add error",
		})
	}
	if txMetadata.ToShardID != nil && txMetadata.FromShardID != nil &&
		components.Type != common.NativeCrossShardTransferOperation && *txMetadata.ToShardID != *txMetadata.FromShardID {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "given from & to shard are different for a native same shard transfer",
		})
	}
	if request.SuggestedFeeMultiplier != nil && *request.SuggestedFeeMultiplier < 1 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "given gas price multiplier must be at least 1",
		})
	}

	options, err := types.MarshalMap(ConstructMetadataOptions{
		TransactionMetadata: txMetadata,
		OperationType:       components.Type,
		GasPriceMultiplier:  request.SuggestedFeeMultiplier,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if _, err := getAddress(components.From); err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return &types.ConstructionPreprocessResponse{
		Options: options,
		RequiredPublicKeys: []*types.AccountIdentifier{
			components.From,
		},
	}, nil
}

// ConstructMetadata with a set of operations will construct a valid transaction
type ConstructMetadata struct {
	Nonce           uint64               `json:"nonce"`
	GasLimit        uint64               `json:"gas_limit"`
	GasPrice        *big.Int             `json:"gas_price"`
	ContractCode    hexutil.Bytes        `json:"contract_code"`
	EvmReturn       hexutil.Bytes        `json:"evm_return"`
	EvmErrorMessage string               `json:"evm_error_message"`
	Transaction     *TransactionMetadata `json:"transaction_metadata"`
}

// UnmarshalFromInterface ..
func (m *ConstructMetadata) UnmarshalFromInterface(blockArgs interface{}) error {
	var T ConstructMetadata
	dat, err := json.Marshal(blockArgs)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dat, &T); err != nil {
		return err
	}
	if T.GasPrice == nil {
		return fmt.Errorf("gas price is required")
	}
	if T.Transaction == nil {
		return fmt.Errorf("transaction metadata is required")
	}
	*m = T
	return nil
}

// ConstructionMetadata implements the /construction/metadata endpoint.
func (s *ConstructAPI) ConstructionMetadata(
	ctx context.Context, request *types.ConstructionMetadataRequest,
) (*types.ConstructionMetadataResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	options := &ConstructMetadataOptions{}
	if err := options.UnmarshalFromInterface(request.Options); err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid metadata option(s)").Error(),
		})
	}

	if len(request.PublicKeys) != 1 {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "require sender public key only",
		})
	}
	senderAddr, rosettaError := getAddressFromPublicKey(request.PublicKeys[0])
	if rosettaError != nil {
		return nil, rosettaError
	}
	nonce, err := s.hmy.GetPoolNonce(ctx, *senderAddr)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	currBlock, err := s.hmy.BlockByNumber(ctx, ethRpc.LatestBlockNumber)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	if options.OperationType == common.NativeCrossShardTransferOperation &&
		!s.hmy.BlockChain.Config().AcceptsCrossTx(currBlock.Epoch()) {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "cross-shard transaction is not accepted yet",
		})
	}

	data := hexutil.Bytes{}
	if options.TransactionMetadata.Data != nil {
		var err error
		if data, err = hexutil.Decode(*options.TransactionMetadata.Data); err != nil {
			return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "invalid tx data format").Error(),
			})
		}
	}

	var contractAddress ethCommon.Address
	if options.TransactionMetadata.ContractAccountIdentifier != nil {
		contractAddress, err = getAddress(options.TransactionMetadata.ContractAccountIdentifier)
		if err != nil {
			return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
				"message": errors.WithMessage(err, "unable to get provided contract address").Error(),
			})
		}
	}
	state, _, err := s.hmy.StateAndHeaderByNumber(ctx, ethRpc.LatestBlockNumber)
	if state == nil || err != nil {
		return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
			"message": "block state not found for latest block",
		})
	}

	latest := ethRpc.BlockNumberOrHashWithNumber(ethRpc.LatestBlockNumber)
	var estGasUsed uint64
	if !isStakingOperation(options.OperationType) {
		if options.OperationType == common.ContractCreationOperation {
			estGasUsed, err = rpc.EstimateGas(ctx, s.hmy, rpc.CallArgs{From: senderAddr, Data: &data}, latest, nil)
			estGasUsed *= 2 // HACK to account for imperfect contract creation estimation
		} else {
			estGasUsed, err = rpc.EstimateGas(
				ctx, s.hmy, rpc.CallArgs{From: senderAddr, To: &contractAddress, Data: &data}, latest, nil,
			)
		}
	} else {
		estGasUsed, err = vm.IntrinsicGas(data, false, false,
			false, options.OperationType == common.CreateValidatorOperation)
		estGasUsed *= 2

	}
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid transaction data").Error(),
		})
	}
	gasMul := float64(1)
	if options.GasPriceMultiplier != nil && *options.GasPriceMultiplier > 1 {
		gasMul = *options.GasPriceMultiplier
	}
	sugNativeFee, sugNativePrice := getSuggestedNativeFeeAndPrice(gasMul, new(big.Int).SetUint64(estGasUsed))

	evmErrorMsg := ""
	evmReturn := hexutil.Bytes{}
	if len(data) > 0 && (options.OperationType == common.ContractCreationOperation ||
		options.OperationType == common.NativeTransferOperation) {
		gas := hexutil.Uint64(estGasUsed)
		callArgs := rpc.CallArgs{
			From: senderAddr,
			Data: &data,
			Gas:  &gas,
		}
		if options.OperationType == common.NativeTransferOperation {
			callArgs.To = &contractAddress
		}
		evmExe, err := rpc.DoEVMCall(
			ctx, s.hmy, callArgs, latest, s.evmCallTimeout,
		)
		if err != nil {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": errors.WithMessage(err, "unable to execute EVM").Error(),
			})
		}
		if evmExe.VMErr != nil {
			evmErrorMsg = evmExe.VMErr.Error()
		}
		evmReturn = evmExe.ReturnData
		sugNativeFee, sugNativePrice = getSuggestedNativeFeeAndPrice(gasMul, new(big.Int).SetUint64(evmExe.UsedGas))
	}

	metadata, err := types.MarshalMap(ConstructMetadata{
		Nonce:           nonce,
		GasPrice:        sugNativePrice,
		GasLimit:        estGasUsed,
		Transaction:     options.TransactionMetadata,
		ContractCode:    state.GetCode(contractAddress),
		EvmErrorMessage: evmErrorMsg,
		EvmReturn:       evmReturn,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return &types.ConstructionMetadataResponse{
		Metadata:     metadata,
		SuggestedFee: sugNativeFee,
	}, nil
}

// getSuggestedNativeFeeAndPrice ..
func getSuggestedNativeFeeAndPrice(
	gasMul float64, estGasUsed *big.Int,
) ([]*types.Amount, *big.Int) {
	if estGasUsed == nil {
		estGasUsed = big.NewInt(0).SetUint64(params.TxGas)
	}
	if gasMul < 1 {
		gasMul = 1
	}
	gasPriceFloat := big.NewFloat(0).Mul(big.NewFloat(DefaultGasPrice), big.NewFloat(gasMul))
	gasPriceTruncated, _ := gasPriceFloat.Uint64()
	gasPrice := new(big.Int).SetUint64(gasPriceTruncated)
	return []*types.Amount{
		{
			Value:    fmt.Sprintf("%v", new(big.Int).Mul(gasPrice, estGasUsed)),
			Currency: &common.NativeCurrency,
		},
	}, gasPrice
}

// isStakingOperation ..
func isStakingOperation(op string) bool {
	for _, stakingOp := range common.StakingOperationTypes {
		if stakingOp == op {
			return true
		}
	}
	return false
}
