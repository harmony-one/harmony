package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/rosetta/common"
	rpc2 "github.com/harmony-one/harmony/rpc"
	"github.com/pkg/errors"
)

var CallMethod = []string{
	"hmyv2_call",
	"hmyv2_getCode",
	"hmyv2_getStorageAt",
	"hmyv2_getDelegationsByDelegator",
	"hmyv2_getDelegationsByDelegatorByBlockNumber",
	"hmyv2_getDelegationsByValidator",
	"hmyv2_getAllValidatorAddresses",
	"hmyv2_getAllValidatorInformation",
	"hmyv2_getAllValidatorInformationByBlockNumber",
	"hmyv2_getElectedValidatorAddresses",
	"hmyv2_getValidatorInformation",
	"hmyv2_getCurrentUtilityMetrics",
	"hmyv2_getMedianRawStakeSnapshot",
	"hmyv2_getStakingNetworkInfo",
	"hmyv2_getSuperCommittees",
}

type CallAPIService struct {
	hmy                 *hmy.Harmony
	publicContractAPI   rpc.API
	publicStakingAPI    rpc.API
	publicBlockChainAPI rpc.API
}

// Call implements the /call endpoint.
func (c *CallAPIService) Call(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	switch request.Method {
	case "hmyv2_call":
		return c.call(ctx, request)
	case "hmyv2_getCode":
		return c.getCode(ctx, request)
	case "hmyv2_getStorageAt":
		return c.getStorageAt(ctx, request)
	case "hmyv2_getDelegationsByDelegator":
		return c.getDelegationsByDelegator(ctx, request)
	case "hmyv2_getDelegationsByDelegatorByBlockNumber":
		return c.getDelegationsByDelegatorByBlockNumber(ctx, request)
	case "hmyv2_getDelegationsByValidator":
		return c.getDelegationsByValidator(ctx, request)
	case "hmyv2_getAllValidatorAddresses":
		return c.getAllValidatorAddresses(ctx)
	case "hmyv2_getAllValidatorInformation":
		return c.getAllValidatorInformation(ctx, request)
	case "hmyv2_getAllValidatorInformationByBlockNumber":
		return c.getAllValidatorInformationByBlockNumber(ctx, request)
	case "hmyv2_getElectedValidatorAddresses":
		return c.getElectedValidatorAddresses(ctx)
	case "hmyv2_getValidatorInformation":
		return c.getValidatorInformation(ctx, request)
	case "hmyv2_getCurrentUtilityMetrics":
		return c.getCurrentUtilityMetrics()
	case "hmyv2_getMedianRawStakeSnapshot":
		return c.getMedianRawStakeSnapshot()
	case "hmyv2_getStakingNetworkInfo":
		return c.getStakingNetworkInfo(ctx)
	case "hmyv2_getSuperCommittees":
		return c.getSuperCommittees()
	}

	return nil, common.NewError(common.ErrCallMethodInvalid, map[string]interface{}{
		"message": "method not supported",
	})

}

func NewCallAPIService(
	hmy *hmy.Harmony,
	limiterEnable bool,
	rateLimit int,
	evmCallTimeout time.Duration,
) server.CallAPIServicer {
	return &CallAPIService{
		hmy:                 hmy,
		publicContractAPI:   rpc2.NewPublicContractAPI(hmy, rpc2.V2, limiterEnable, rateLimit, evmCallTimeout),
		publicStakingAPI:    rpc2.NewPublicStakingAPI(hmy, rpc2.V2),
		publicBlockChainAPI: rpc2.NewPublicBlockchainAPI(hmy, rpc2.V2, limiterEnable, rateLimit),
	}
}

func (c *CallAPIService) getSuperCommittees() (*types.CallResponse, *types.Error) {
	committees, err := c.hmy.GetSuperCommittees()
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get super committees error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": committees,
		},
	}, nil
}

func (c *CallAPIService) getStakingNetworkInfo(
	ctx context.Context,
) (*types.CallResponse, *types.Error) {
	publicBlockChainAPI := c.publicBlockChainAPI.Service.(*rpc2.PublicBlockchainService)
	resp, err := publicBlockChainAPI.GetStakingNetworkInfo(ctx)
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get staking network info error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": resp,
		},
	}, nil
}

func (c *CallAPIService) getMedianRawStakeSnapshot() (*types.CallResponse, *types.Error) {
	snapshot, err := c.hmy.GetMedianRawStakeSnapshot()
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get median raw stake snapshot error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": snapshot,
		},
	}, nil
}

func (c *CallAPIService) getCurrentUtilityMetrics() (*types.CallResponse, *types.Error) {
	metric, err := c.hmy.GetCurrentUtilityMetrics()
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get current utility metrics error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": metric,
		},
	}, nil
}

func (c *CallAPIService) getValidatorInformation(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	stakingAPI := c.publicStakingAPI.Service.(*rpc2.PublicStakingService)
	args := GetValidatorInformationRequest{}
	if err := args.UnmarshalFromInterface(request.Parameters); err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid parameters").Error(),
		})
	}
	resp, err := stakingAPI.GetValidatorInformation(ctx, args.ValidatorAddr)
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get validator information error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": resp,
		},
	}, nil
}

func (c *CallAPIService) getElectedValidatorAddresses(
	ctx context.Context,
) (*types.CallResponse, *types.Error) {
	stakingAPI := c.publicStakingAPI.Service.(*rpc2.PublicStakingService)
	addresses, err := stakingAPI.GetElectedValidatorAddresses(ctx)
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get elected validator addresses error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": addresses,
		},
	}, nil
}

func (c *CallAPIService) getAllValidatorInformationByBlockNumber(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	stakingAPI := c.publicStakingAPI.Service.(*rpc2.PublicStakingService)
	args := GetAllValidatorInformationByBlockNumberRequest{}
	if err := args.UnmarshalFromInterface(request.Parameters); err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid parameters").Error(),
		})
	}
	resp, err := stakingAPI.GetAllValidatorInformationByBlockNumber(ctx, args.PageNumber, rpc2.BlockNumber(args.BlockNumber))
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get all validator information by block number error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": resp,
		},
	}, nil
}

func (c *CallAPIService) getAllValidatorInformation(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	stakingAPI := c.publicStakingAPI.Service.(*rpc2.PublicStakingService)
	args := GetAllValidatorInformationRequest{}
	if err := args.UnmarshalFromInterface(request.Parameters); err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid parameters").Error(),
		})
	}
	resp, err := stakingAPI.GetAllValidatorInformation(ctx, args.PageNumber)
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get all validator information error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": resp,
		},
	}, nil

}

func (c *CallAPIService) getAllValidatorAddresses(
	ctx context.Context,
) (*types.CallResponse, *types.Error) {
	stakingAPI := c.publicStakingAPI.Service.(*rpc2.PublicStakingService)
	addresses, err := stakingAPI.GetAllValidatorAddresses(ctx)
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get all validator addresses error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": addresses,
		},
	}, nil
}

func (c *CallAPIService) getDelegationsByValidator(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	stakingAPI := c.publicStakingAPI.Service.(*rpc2.PublicStakingService)
	args := GetDelegationsByValidatorRequest{}
	if err := args.UnmarshalFromInterface(request.Parameters); err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid parameters").Error(),
		})
	}
	resp, err := stakingAPI.GetDelegationsByValidator(ctx, args.ValidatorAddr)
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get delegations by validator error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": resp,
		},
	}, nil
}

func (c *CallAPIService) getDelegationsByDelegatorByBlockNumber(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	stakingAPI := c.publicStakingAPI.Service.(*rpc2.PublicStakingService)
	args := GetDelegationByDelegatorAddrAndBlockNumRequest{}
	if err := args.UnmarshalFromInterface(request.Parameters); err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid parameters").Error(),
		})
	}

	addr, err := internal_common.Bech32ToAddress(args.DelegatorAddr)
	if err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "delegator address error").Error(),
		})
	}

	addOrList := rpc2.AddressOrList{Address: &addr}
	resp, err := stakingAPI.GetDelegationsByDelegatorByBlockNumber(ctx, addOrList, rpc2.BlockNumber(args.BlockNum))
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get delegations by delegator and number error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": resp,
		},
	}, nil
}

func (c *CallAPIService) getDelegationsByDelegator(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	stakingAPI := c.publicStakingAPI.Service.(*rpc2.PublicStakingService)
	args := GetDelegationsByDelegatorAddrRequest{}
	if err := args.UnmarshalFromInterface(request.Parameters); err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid parameters").Error(),
		})
	}
	resp, err := stakingAPI.GetDelegationsByDelegator(ctx, args.DelegatorAddr)
	if err != nil {
		return nil, common.NewError(common.ErrGetStakingInfo, map[string]interface{}{
			"message": errors.WithMessage(err, "get delegations by delegator error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": resp,
		},
	}, nil
}

func (c *CallAPIService) getStorageAt(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	contractAPI := c.publicContractAPI.Service.(*rpc2.PublicContractService)
	args := GetStorageAtRequest{}
	if err := args.UnmarshalFromInterface(request.Parameters); err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid parameters").Error(),
		})
	}

	res, err := contractAPI.GetStorageAt(ctx, args.Addr, args.Key, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(args.BlockNum)))
	if err != nil {
		return nil, common.NewError(common.ErrCallExecute, map[string]interface{}{
			"message": errors.WithMessage(err, "get storage at error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": res.String(),
		},
	}, nil
}

func (c *CallAPIService) getCode(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	contractAPI := c.publicContractAPI.Service.(*rpc2.PublicContractService)
	args := GetCodeRequest{}
	if err := args.UnmarshalFromInterface(request.Parameters); err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid parameters").Error(),
		})
	}
	code, err := contractAPI.GetCode(ctx, args.Addr, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(args.BlockNum)))
	if err != nil {
		return nil, common.NewError(common.ErrCallExecute, map[string]interface{}{
			"message": errors.WithMessage(err, "get code error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": code.String(),
		},
	}, nil
}

func (c *CallAPIService) call(
	ctx context.Context, request *types.CallRequest,
) (*types.CallResponse, *types.Error) {
	contractAPI := c.publicContractAPI.Service.(*rpc2.PublicContractService)
	args := CallRequest{}
	if err := args.UnmarshalFromInterface(request.Parameters); err != nil {
		return nil, common.NewError(common.ErrCallParametersInvalid, map[string]interface{}{
			"message": errors.WithMessage(err, "invalid parameters").Error(),
		})
	}
	data, err := contractAPI.Call(ctx, args.CallArgs, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(args.BlockNum)))
	if err != nil {
		return nil, common.NewError(common.ErrCallExecute, map[string]interface{}{
			"message": errors.WithMessage(err, "call smart contract error").Error(),
		})
	}
	return &types.CallResponse{
		Result: map[string]interface{}{
			"result": data.String(),
		},
	}, nil
}

type CallRequest struct {
	rpc2.CallArgs
	BlockNum int64 `json:"block_num"`
}

func (cr *CallRequest) UnmarshalFromInterface(args interface{}) error {
	var callRequest CallRequest
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &callRequest); err != nil {
		return err
	}
	*cr = callRequest
	return nil
}

type GetCodeRequest struct {
	Addr     string `json:"addr"`
	BlockNum int64  `json:"block_num"`
}

func (cr *GetCodeRequest) UnmarshalFromInterface(args interface{}) error {
	var getCodeRequest GetCodeRequest
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &getCodeRequest); err != nil {
		return err
	}
	*cr = getCodeRequest
	return nil
}

type GetStorageAtRequest struct {
	Addr     string `json:"addr"`
	Key      string `json:"key"`
	BlockNum int64  `json:"block_num"`
}

func (sr *GetStorageAtRequest) UnmarshalFromInterface(args interface{}) error {
	var getStorageAt GetStorageAtRequest
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &getStorageAt); err != nil {
		return err
	}
	*sr = getStorageAt
	return nil
}

type GetDelegationsByDelegatorAddrRequest struct {
	DelegatorAddr string `json:"delegator_addr"`
}

func (r *GetDelegationsByDelegatorAddrRequest) UnmarshalFromInterface(args interface{}) error {
	var getDelegationsByDelegatorAddrRequest GetDelegationsByDelegatorAddrRequest
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &getDelegationsByDelegatorAddrRequest); err != nil {
		return err
	}
	*r = getDelegationsByDelegatorAddrRequest
	return nil
}

type GetDelegationByDelegatorAddrAndBlockNumRequest struct {
	DelegatorAddr string `json:"delegator_addr"`
	BlockNum      int64  `json:"block_num"`
}

func (r *GetDelegationByDelegatorAddrAndBlockNumRequest) UnmarshalFromInterface(args interface{}) error {
	var getDelegationByDelegatorAddrAndBlockNumRequest GetDelegationByDelegatorAddrAndBlockNumRequest
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &getDelegationByDelegatorAddrAndBlockNumRequest); err != nil {
		return err
	}
	*r = getDelegationByDelegatorAddrAndBlockNumRequest
	return nil
}

type GetDelegationsByValidatorRequest struct {
	ValidatorAddr string `json:"validator_addr"`
}

func (r *GetDelegationsByValidatorRequest) UnmarshalFromInterface(args interface{}) error {
	var getDelegationsByValidatorRequest GetDelegationsByValidatorRequest
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &getDelegationsByValidatorRequest); err != nil {
		return err
	}
	*r = getDelegationsByValidatorRequest
	return nil
}

type GetAllValidatorInformationRequest struct {
	PageNumber int `json:"page_number"`
}

func (r *GetAllValidatorInformationRequest) UnmarshalFromInterface(args interface{}) error {
	var getAllValidatorInformationRequest GetAllValidatorInformationRequest
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &getAllValidatorInformationRequest); err != nil {
		return err
	}
	*r = getAllValidatorInformationRequest
	return nil
}

type GetAllValidatorInformationByBlockNumberRequest struct {
	PageNumber  int   `json:"page_number"`
	BlockNumber int64 `json:"block_number"`
}

func (r *GetAllValidatorInformationByBlockNumberRequest) UnmarshalFromInterface(args interface{}) error {
	var getAllValidatorInformationByBlockNumber GetAllValidatorInformationByBlockNumberRequest
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &getAllValidatorInformationByBlockNumber); err != nil {
		return err
	}
	*r = getAllValidatorInformationByBlockNumber
	return nil
}

type GetValidatorInformationRequest struct {
	ValidatorAddr string `json:"validator_addr"`
}

func (r *GetValidatorInformationRequest) UnmarshalFromInterface(args interface{}) error {
	var getValidatorInformationRequest GetValidatorInformationRequest
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &getValidatorInformationRequest); err != nil {
		return err
	}
	*r = getValidatorInformationRequest
	return nil
}
