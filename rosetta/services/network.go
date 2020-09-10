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

// NetworkAPI implements the server.NetworkAPIServicer interface.
type NetworkAPI struct {
	hmy *hmy.Harmony
}

// NewNetworkAPI creates a new instance of a NetworkAPI.
func NewNetworkAPI(hmy *hmy.Harmony) server.NetworkAPIServicer {
	return &NetworkAPI{
		hmy: hmy,
	}
}

// NetworkList implements the /network/list endpoint
// TODO (dm): Update Node API to support multiple shards...
func (s *NetworkAPI) NetworkList(
	ctx context.Context, request *types.MetadataRequest,
) (*types.NetworkListResponse, *types.Error) {
	network, err := common.GetNetwork(s.hmy.ShardID)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return &types.NetworkListResponse{
		NetworkIdentifiers: []*types.NetworkIdentifier{
			network,
		},
	}, nil
}

// NetworkStatus implements the /network/status endpoint
func (s *NetworkAPI) NetworkStatus(
	ctx context.Context, request *types.NetworkRequest,
) (*types.NetworkStatusResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	// Fetch relevant headers, syncing status, & peers
	currentHeader, err := s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": fmt.Sprintf("unable to get current header: %v", err.Error()),
		})
	}
	genesisHeader, err := s.hmy.HeaderByNumber(ctx, rpc.BlockNumber(0))
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": fmt.Sprintf("unable to get genesis header: %v", err.Error()),
		})
	}
	peers, rosettaError := getPeersFromNodePeerInfo(s.hmy.GetPeerInfo())
	if rosettaError != nil {
		return nil, rosettaError
	}
	targetHeight := int64(s.hmy.NodeAPI.GetMaxPeerHeight())
	syncStatus := common.SyncingFinish
	if s.hmy.NodeAPI.IsOutOfSync(s.hmy.BlockChain) {
		syncStatus = common.SyncingNewBlock
	} else if targetHeight == 0 {
		syncStatus = common.SyncingStartup
	}
	stage := syncStatus.String()

	currentBlockIdentifier := &types.BlockIdentifier{
		Index: currentHeader.Number().Int64(),
		Hash:  currentHeader.Hash().String(),
	}

	// Only applicable to non-archival nodes
	var oldestBlockIdentifier *types.BlockIdentifier
	if !nodeconfig.GetDefaultConfig().GetArchival() {
		maxGarbCollectedBlockNum := s.hmy.BlockChain.GetMaxGarbageCollectedBlockNumber()
		if maxGarbCollectedBlockNum == -1 || maxGarbCollectedBlockNum >= currentHeader.Number().Int64() {
			oldestBlockIdentifier = currentBlockIdentifier
		} else {
			oldestBlockHeader, err := s.hmy.HeaderByNumber(ctx, rpc.BlockNumber(maxGarbCollectedBlockNum+1))
			if err != nil {
				return nil, common.NewError(common.CatchAllError, map[string]interface{}{
					"message": fmt.Sprintf("unable to get oldest block header: %v", err.Error()),
				})
			}
			oldestBlockIdentifier = &types.BlockIdentifier{
				Index: oldestBlockHeader.Number().Int64(),
				Hash:  oldestBlockHeader.Hash().String(),
			}
		}
	}

	return &types.NetworkStatusResponse{
		CurrentBlockIdentifier: currentBlockIdentifier,
		OldestBlockIdentifier:  oldestBlockIdentifier,
		CurrentBlockTimestamp:  currentHeader.Time().Int64() * 1e3, // Timestamp must be in ms.
		GenesisBlockIdentifier: &types.BlockIdentifier{
			Index: genesisHeader.Number().Int64(),
			Hash:  genesisHeader.Hash().String(),
		},
		Peers: peers,
		SyncStatus: &types.SyncStatus{
			CurrentIndex: currentHeader.Number().Int64(),
			TargetIndex:  &targetHeight,
			Stage:        &stage,
		},
	}, nil
}

// NetworkOptions implements the /network/options endpoint
func (s *NetworkAPI) NetworkOptions(
	ctx context.Context, request *types.NetworkRequest,
) (*types.NetworkOptionsResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	// Fetch allows based on current network option
	var allow *types.Allow
	isArchival := nodeconfig.GetDefaultConfig().GetArchival()
	if s.hmy.ShardID == shard.BeaconChainShardID {
		allow = getBeaconAllow(isArchival)
	} else {
		allow = getAllow(isArchival)
	}

	return &types.NetworkOptionsResponse{
		Version: &types.Version{
			RosettaVersion: common.RosettaVersion,
			NodeVersion:    nodeconfig.GetVersion(),
		},
		Allow: allow,
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
		&common.ReceiptNotFoundError,
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
					return nil, common.NewError(common.SanityCheckError, map[string]interface{}{
						"message": "could not cast peer metadata to slice of string",
					})
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
		return common.NewError(common.SanityCheckError, map[string]interface{}{
			"message": fmt.Sprintf("Error while asserting valid network ID: %v", err.Error()),
		})
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
			return common.NewError(common.SanityCheckError, map[string]interface{}{
				"message": fmt.Sprintf("Error while asserting valid network ID: %v", err.Error()),
			})
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
		return common.NewError(common.InvalidNetworkError, map[string]interface{}{
			"message": message,
		})
	}
	return nil
}
