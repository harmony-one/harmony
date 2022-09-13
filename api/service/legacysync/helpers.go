package legacysync

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/pkg/errors"
)

// getMaxPeerHeight gets the maximum blockchain heights from peers
func getMaxPeerHeight(syncConfig *SyncConfig) uint64 {
	maxHeight := uint64(math.MaxUint64)
	var (
		wg   sync.WaitGroup
		lock sync.Mutex
	)

	syncConfig.ForEachPeer(func(peerConfig *SyncPeerConfig) (brk bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//debug
			// utils.Logger().Debug().Bool("isBeacon", isBeacon).Str("peerIP", peerConfig.ip).Str("peerPort", peerConfig.port).Msg("[Sync]getMaxPeerHeight")
			response, err := peerConfig.client.GetBlockChainHeight()
			if err != nil {
				utils.Logger().Warn().Err(err).Str("peerIP", peerConfig.peer.IP).Str("peerPort", peerConfig.peer.Port).Msg("[Sync]GetBlockChainHeight failed")
				syncConfig.RemovePeer(peerConfig, fmt.Sprintf("failed getMaxPeerHeight for shard %d with message: %s", syncConfig.ShardID(), err.Error()))
				return
			}
			utils.Logger().Info().Str("peerIP", peerConfig.peer.IP).Uint64("blockHeight", response.BlockHeight).
				Msg("[SYNC] getMaxPeerHeight")

			lock.Lock()
			if response != nil {
				if response.BlockHeight < math.MaxUint32 { // That's enough for decades.
					if maxHeight == uint64(math.MaxUint64) || maxHeight < response.BlockHeight {
						maxHeight = response.BlockHeight
					}
				}
			}
			lock.Unlock()
		}()
		return
	})
	wg.Wait()
	return maxHeight
}

func createSyncConfig(syncConfig *SyncConfig, provider SyncingPeerProvider, shardID uint32) (*SyncConfig, error) {
	peers, err := provider.SyncingPeers(shardID)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get syncing peers for shard %d", shardID)
	}

	storage := syncConfig.Storage()
	storage.AddPeers(peers)

	// limit the number of dns peers to connect
	peers = storage.GetPeersN(limitPeersNum())

	utils.Logger().Debug().
		Int("len", len(peers)).
		Uint32("shardID", shardID).
		Msg("[SYNC] CreateSyncConfig: len of peers")

	if len(peers) == 0 {
		return syncConfig, errors.New("[SYNC] no peers to connect to")
	}
	if syncConfig != nil {
		syncConfig.CloseConnections()
	}
	syncConfig = NewSyncConfig(shardID, storage)

	ch := make(chan *SyncPeerConfig, len(peers))
	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(peer p2p.Peer) {
			defer wg.Done()
			client := downloader.ClientSetup(peer.IP, peer.Port)
			if client == nil {
				return
			}
			peerConfig := &SyncPeerConfig{
				peer:   peer,
				client: client,
			}
			ch <- peerConfig
		}(peer)
	}
	wg.Wait()
	close(ch)
	for v := range ch {
		syncConfig.AddPeer(v)
	}
	utils.Logger().Info().
		Int("len", len(syncConfig.peers)).
		Uint32("shardID", shardID).
		Msg("[SYNC] Finished making connection to peers")

	return syncConfig, nil
}
