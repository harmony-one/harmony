package legacysync

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/harmony-one/harmony/api/service/legacysync/downloader"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

// getMaxPeerHeight gets the maximum blockchain heights from peers
func getMaxPeerHeight(syncConfig *SyncConfig) (uint64, error) {
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

	if maxHeight == uint64(math.MaxUint64) {
		return 0, fmt.Errorf("get max peer height failed")
	}

	return maxHeight, nil
}

func createSyncConfig(syncConfig *SyncConfig, peers []p2p.Peer, shardID uint32, selfPeerID libp2p_peer.ID, waitForEachPeerToConnect bool) (*SyncConfig, error) {
	// sanity check to ensure no duplicate peers
	if err := checkPeersDuplicity(peers); err != nil {
		return syncConfig, err
	}
	// limit the number of dns peers to connect
	randSeed := time.Now().UnixNano()
	targetSize, peers := limitNumPeers(peers, randSeed)

	utils.Logger().Debug().
		Str("self peer ID", string(selfPeerID)).
		Int("peers count", len(peers)).
		Int("target size", targetSize).
		Uint32("shardID", shardID).
		Msg("[SYNC] CreateSyncConfig: len of peers")

	if targetSize == 0 {
		return syncConfig, errors.New("[SYNC] no peers to connect to")
	}
	if syncConfig != nil {
		syncConfig.CloseConnections()
	}
	syncConfig = NewSyncConfig(shardID, selfPeerID, nil)

	if !waitForEachPeerToConnect {
		var wg sync.WaitGroup
		ps := peers[:targetSize]
		for _, peer := range ps {
			wg.Add(1)
			go func(peer p2p.Peer) {
				defer wg.Done()
				client := downloader.ClientSetup(peer.IP, peer.Port, false)
				if client == nil {
					return
				}
				peerConfig := &SyncPeerConfig{
					peer:   peer,
					client: client,
				}
				syncConfig.AddPeer(peerConfig)
			}(peer)
		}
		wg.Wait()
	} else {
		var connectedPeers int
		for _, peer := range peers {
			client := downloader.ClientSetup(peer.IP, peer.Port, true)
			if client == nil || !client.IsReady() {
				continue
			}
			peerConfig := &SyncPeerConfig{
				peer:   peer,
				client: client,
			}
			syncConfig.AddPeer(peerConfig)
			connectedPeers++
			if connectedPeers >= targetSize {
				break
			}
		}
	}
	utils.Logger().Info().
		Int("len", len(syncConfig.peers)).
		Uint32("shardID", shardID).
		Msg("[SYNC] Finished making connection to peers")

	return syncConfig, nil
}
