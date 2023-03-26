package services

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/math"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/eth/rpc"
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
	currBlock := s.hmy.CurrentBlock()
	var currentHeader *block.Header
	var err error
	if currBlock.Number().Cmp(big.NewInt(0)) == 1 && !s.hmy.IsStakingEpoch(currBlock.Epoch()) {
		// all blocks in the era before staking epoch requires the next block to get the block reward transactions
		blkNum := new(big.Int).Sub(currBlock.Number(), big.NewInt(1))
		currentHeader, err = s.hmy.HeaderByNumber(ctx, rpc.BlockNumber(blkNum.Uint64()))
	} else {
		currentHeader, err = s.hmy.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	}
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
	isSyncing, targetHeight, _ := s.hmy.NodeAPI.SyncStatus(s.hmy.BlockChain.ShardID())
	syncStatus := common.SyncingFinish
	if targetHeight == 0 {
		syncStatus = common.SyncingUnknown
	} else if isSyncing {
		syncStatus = common.SyncingNewBlock
	}
	stage := syncStatus.String()

	currentBlockIdentifier := &types.BlockIdentifier{
		Index: currentHeader.Number().Int64(),
		Hash:  currentHeader.Hash().String(),
	}

	// Only applicable to non-archival nodes
	var oldestBlockIdentifier *types.BlockIdentifier
	if !nodeconfig.GetShardConfig(s.hmy.ShardID).GetArchival() {
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
	targetInt := int64(targetHeight)
	if targetHeight == math.MaxUint64 {
		targetInt = 0
	}
	currentIndex := currentHeader.Number().Int64()
	ss := &types.SyncStatus{
		CurrentIndex: &currentIndex,
		TargetIndex:  &targetInt,
		Stage:        &stage,
	}

	return &types.NetworkStatusResponse{
		CurrentBlockIdentifier: currentBlockIdentifier,
		OldestBlockIdentifier:  oldestBlockIdentifier,
		CurrentBlockTimestamp:  currentHeader.Time().Int64() * 1e3, // Timestamp must be in ms.
		GenesisBlockIdentifier: &types.BlockIdentifier{
			Index: genesisHeader.Number().Int64(),
			Hash:  genesisHeader.Hash().String(),
		},
		Peers:      peers,
		SyncStatus: ss,
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
	isArchival := nodeconfig.GetShardConfig(s.hmy.ShardID).GetArchival()
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
		&common.UnsupportedCurveTypeError,
		&common.InvalidTransactionConstructionError,
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

	if netID == nil || types.Hash(currNetID) != types.Hash(netID) {
		return &common.InvalidNetworkError
	}
	return nil
}
