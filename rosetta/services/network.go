package services

import (
	"context"
	"fmt"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/harmony-one/harmony/hmy"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/rosetta/common"
	commonRPC "github.com/harmony-one/harmony/rpc/common"
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

// NetworkList implements the /network/list endpoint
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

// NetworkStatus implements the /network/status endpoint
func (s *NetworkAPIService) NetworkStatus(
	ctx context.Context,
	request *types.NetworkRequest,
) (*types.NetworkStatusResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	currentHeader, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		rosettaError := common.CatchAllError
		rosettaError.Details = map[string]interface{}{
			"message": fmt.Sprintf("unable to get current header: %v", err.Error()),
		}
		return nil, &rosettaError
	}

	genesisHeader, err := s.hmy.HeaderByNumber(ctx, rpc.BlockNumber(0))
	if err != nil {
		rosettaError := common.CatchAllError
		rosettaError.Details = map[string]interface{}{
			"message": fmt.Sprintf("unable to get genesis header: %v", err.Error()),
		}
		return nil, &rosettaError
	}

	peers, rosettaError := getPeersFromNodePeerInfo(s.hmy.GetPeerInfo())
	if rosettaError != nil {
		return nil, rosettaError
	}

	return &types.NetworkStatusResponse{
		CurrentBlockIdentifier: &types.BlockIdentifier{
			Index: currentHeader.Number().Int64(),
			Hash:  currentHeader.Hash().String(),
		},
		CurrentBlockTimestamp: currentHeader.Time().Int64() * 1e3, // Timestamp must be in ms.
		GenesisBlockIdentifier: &types.BlockIdentifier{
			Index: genesisHeader.Number().Int64(),
			Hash:  genesisHeader.Hash().String(),
		},
		Peers: peers,
		// TODO (dm): implement proper sync status report
		SyncStatus: &types.SyncStatus{
			CurrentIndex: currentHeader.Number().Int64(),
		},
	}, nil
}

// NetworkOptions implements the /network/options endpoint
func (s *NetworkAPIService) NetworkOptions(
	ctx context.Context,
	request *types.NetworkRequest,
) (*types.NetworkOptionsResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

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
		&common.InvalidNetworkError,
		&common.TransactionSubmissionError,
		&common.BlockNotFoundError,
		&common.TransactionNotFoundError,
	}
}

// getPeersFromNodePeerInfo formats all the unique peers from the NodePeerInfo and
// notes each topic for each peer in the metadata.
func getPeersFromNodePeerInfo(allPeerInfo commonRPC.NodePeerInfo) ([]*types.Peer, *types.Error) {
	seenPeerIndex := map[peer.ID]int{}
	peers := []*types.Peer{}
	for _, peerInfo := range allPeerInfo.P {
		for _, pID := range peerInfo.Peers {
			i, ok := seenPeerIndex[pID]
			if !ok {
				newPeer := &types.Peer{
					PeerID: pID.String(),
					Metadata: map[string]interface{}{
						"topics": []string{peerInfo.Topic},
					},
				}
				peers = append(peers, newPeer)
				seenPeerIndex[pID] = len(peers) - 1
			} else {
				topics, ok := peers[i].Metadata["topics"].([]string)
				if !ok {
					err := common.SanityCheckError
					err.Message = "could not cast peer metadata to slice of string"
					return nil, &err
				}
				for _, topic := range topics {
					if peerInfo.Topic == topic {
						continue
					}
				}
				peers[i].Metadata["topics"] = append(topics, peerInfo.Topic)
			}
		}
	}
	return peers, nil
}

func assertValidNetworkIdentifier(netID *types.NetworkIdentifier, shardID uint32) *types.Error {
	currNetID, err := common.GetNetwork(shardID)
	if err != nil {
		rosettaError := common.SanityCheckError
		rosettaError.Details = map[string]interface{}{
			"message": fmt.Sprintf("Error while asserting valid network ID: %v", err.Error()),
		}
		return &rosettaError
	}

	// Check for valid network ID and set a message if an error occurs
	message := ""
	if netID.Blockchain != currNetID.Blockchain {
		message = fmt.Sprintf("Invalid blockchain, expected %v", currNetID.Blockchain)
	} else if netID.Network != currNetID.Network {
		message = fmt.Sprintf("Invalid network, expected %v", currNetID.Network)
	} else if netID.SubNetworkIdentifier.Network != currNetID.SubNetworkIdentifier.Network {
		message = fmt.Sprintf(
			"Invalid subnetwork, expected %v", currNetID.SubNetworkIdentifier.Network,
		)
	} else {
		var metadata, currMetadata common.SubNetworkMetadata

		if err := currMetadata.UnmarshalFromInterface(currNetID.SubNetworkIdentifier.Metadata); err != nil {
			rosettaError := common.SanityCheckError
			rosettaError.Details = map[string]interface{}{
				"message": fmt.Sprintf("Error while asserting valid network ID: %v", err.Error()),
			}
			return &rosettaError
		}
		if err := metadata.UnmarshalFromInterface(netID.SubNetworkIdentifier.Metadata); err != nil {
			message = fmt.Sprintf("Subnetwork metadata is of unknown format: %v", err.Error())
		}

		if metadata.IsBeacon != currMetadata.IsBeacon {
			if currMetadata.IsBeacon {
				message = "Invalid subnetwork, expected beacon chain subnetwork"
			} else {
				message = "Invalid subnetwork, expected non-beacon chain subnetwork"
			}
		}
	}

	if message != "" {
		rosettaError := common.InvalidNetworkError
		rosettaError.Details = map[string]interface{}{
			"message": message,
		}
		return &rosettaError
	}
	return nil
}
