package node

import (
	"time"

	downloader_pb "github.com/harmony-one/harmony/api/service/syncing/downloader/proto"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/node/worker"
)

// Constants related to doing syncing.
const (
	lastMileThreshold = 4
	inSyncThreshold   = 1 // unit in number of block
	SyncFrequency     = 30
	MinConnectedPeers = 10 // minimum number of peers connected to in node syncing
)

// IsSameHeight tells whether node is at same bc height as a peer
// func (node *Node) IsSameHeight() (uint64, bool) {
// return node.stateSync.IsSameBlockchainHeight(node.Blockchain())
// }

// StartBeaconBlockStateSync update received beaconchain
// blocks and downloads missing beacon chain blocks
func (node *Node) StartBeaconBlockStateSync() error {
	return node.DoSyncing(
		node.Beaconchain(), node.BeaconWorker, false,
	)
}

// DoSyncing keep the node in sync with other peers,
// willJoinConsensus means the node will try to join consensus after catch up
func (node *Node) DoSyncing(
	bc *core.BlockChain,
	worker *worker.Worker,
	willJoinConsensus bool,
) error {

	ticker := time.NewTicker(time.Duration(SyncFrequency) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			node.doSync(bc, worker, willJoinConsensus)
		case <-node.Consensus.BlockNumLowChan:
			node.doSync(bc, worker, willJoinConsensus)
		}
	}

	return nil
}

// doSync keep the node in sync with other peers,
// willJoinConsensus means the node will try to join consensus after catch up
func (node *Node) doSync(
	bc *core.BlockChain, worker *worker.Worker, willJoinConsensus bool,
) {
	// if node.stateSync.GetActivePeerNumber() < MinConnectedPeers {
	// 	shardID := bc.ShardID()
	// 	peers, err := node.SyncingPeerProvider.SyncingPeers(shardID)
	// 	if err != nil {
	// 		utils.Logger().Warn().
	// 			Err(err).
	// 			Uint32("shard_id", shardID).
	// 			Msg("cannot retrieve syncing peers")
	// 		return
	// 	}
	// 	if err := node.stateSync.CreateSyncConfig(peers, false); err != nil {
	// 		utils.Logger().Warn().
	// 			Err(err).
	// 			Interface("peers", peers).
	// 			Msg("[SYNC] create peers error")
	// 		return
	// 	}
	// 	utils.Logger().Debug().
	// 		Int("len", node.stateSync.GetActivePeerNumber()).
	// 		Msg("[SYNC] Get Active Peers")
	// }

	// // TODO: treat fake maximum height
	// if node.stateSync.IsOutOfSync(bc) {
	// 	node.State.Store(NotInSync)
	// 	if willJoinConsensus {
	// 		node.Consensus.BlocksNotSynchronized()
	// 	}

	// 	node.stateSync.SyncLoop(bc, worker, false, node.Consensus)
	// 	if willJoinConsensus {
	// 		node.State.Store(ReadyForConsensus)
	// 		node.Consensus.BlocksSynchronized()
	// 	}
	// }
	// node.State.Store(ReadyForConsensus)
}

// StartBlockStateSync ..
func (node *Node) StartBlockStateSync() error {
	// if node.downloaderServer.GrpcServer == nil {
	// 	if _, err := node.downloaderServer.Start(
	// 		node.SelfPeer.IP, syncing.GetSyncingPort(node.SelfPeer.Port),
	// 	); err != nil {
	// 		return err
	// 	}
	// }

	// joinConsensus := node.NodeConfig.Role() == nodeconfig.Validator
	// return node.DoSyncing(node.Blockchain(), node.Worker, joinConsensus)
	return nil
}

// CalculateResponse implements DownloadInterface on Node object.
func (node *Node) CalculateResponse(
	request *downloader_pb.DownloaderRequest, incomingPeer string,
) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	return response, nil
}
