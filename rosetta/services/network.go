package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/harmony-one/harmony/hmy"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/shard"
)

// NetworkAPIService implements the server.NetworkAPIServicer interface.
type NetworkAPIService struct {
	hmy *hmy.Harmony
}

// NewNetworkAPIService creates a new instance of a NetworkAPIService.
func NewNetworkAPIService(hmy *hmy.Harmony) server.NetworkAPIServicer {
	return &NetworkAPIService{
		hmy: hmy,
	}
}

// NetworkList implements the /network/list endpoint (placeholder)
// TODO (dm): Update Node API to support beacon shard functionality for all nodes.
func (s *NetworkAPIService) NetworkList(
	ctx context.Context,
	request *types.MetadataRequest,
) (*types.NetworkListResponse, *types.Error) {
	network, err := common.GetNetwork(s.hmy.ShardID)
	if err != nil {
		rosettaError := common.CatchAllError
		rosettaError.Details = map[string]interface{}{
			"message": err.Error(),
		}
		return nil, &rosettaError
	}
	return &types.NetworkListResponse{
		NetworkIdentifiers: []*types.NetworkIdentifier{
			network,
		},
	}, nil
}

// NetworkStatus implements the /network/status endpoint (placeholder)
// FIXME: remove placeholder & implement block endpoint
func (s *NetworkAPIService) NetworkStatus(
	ctx context.Context,
	request *types.NetworkRequest,
) (*types.NetworkStatusResponse, *types.Error) {
	return &types.NetworkStatusResponse{
		CurrentBlockIdentifier: &types.BlockIdentifier{
			Index: 1000,
			Hash:  "block 1000",
		},
		CurrentBlockTimestamp: int64(1586483189000),
		GenesisBlockIdentifier: &types.BlockIdentifier{
			Index: 0,
			Hash:  "block 0",
		},
		Peers: []*types.Peer{
			{
				PeerID: "peer 1",
			},
		},
	}, nil
}

// NetworkOptions implements the /network/options endpoint (placeholder)
func (s *NetworkAPIService) NetworkOptions(
	ctx context.Context,
	request *types.NetworkRequest,
) (*types.NetworkOptionsResponse, *types.Error) {
	version := &types.Version{
		RosettaVersion: common.RosettaVersion,
		NodeVersion:    nodeconfig.GetVersion(),
	}

	var allow *types.Allow
	isArchival := nodeconfig.GetDefaultConfig().GetArchival()
	if s.hmy.ShardID == shard.BeaconChainShardID {
		allow = getBeaconAllow(isArchival)
	} else {
		allow = getAllow(isArchival)
	}

	return &types.NetworkOptionsResponse{
		Version: version,
		Allow:   allow,
	}, nil
}

func getBeaconAllow(isArchival bool) *types.Allow {
	return &types.Allow{
		OperationStatuses:       append(getOperationStatuses(), getBeaconOperationStatuses()...),
		OperationTypes:          append(common.PlainOperationTypes, common.StakingOperationTypes...),
		Errors:                  append(getErrors(), getBeaconErrors()...),
		HistoricalBalanceLookup: isArchival,
	}
}

func getAllow(isArchival bool) *types.Allow {
	return &types.Allow{
		OperationStatuses:       getOperationStatuses(),
		OperationTypes:          common.PlainOperationTypes,
		Errors:                  getErrors(),
		HistoricalBalanceLookup: isArchival,
	}
}

func getBeaconOperationStatuses() []*types.OperationStatus {
	return []*types.OperationStatus{}
}

func getOperationStatuses() []*types.OperationStatus {
	return []*types.OperationStatus{
		common.SuccessOperationStatus,
		common.FailureOperationStatus,
		common.ContractFailureOperationStatus,
	}
}

func getBeaconErrors() []*types.Error {
	return []*types.Error{
		&common.StakingTransactionSubmissionError,
	}
}

func getErrors() []*types.Error {
	return []*types.Error{
		&common.CatchAllError,
		&common.SanityCheckError,
		&common.TransactionSubmissionError,
	}
}
