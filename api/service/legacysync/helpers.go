package legacysync

import (
	"sync"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/harmony-one/harmony/internal/utils"
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
				utils.Logger().Warn().Err(err).Str("peerIP", peerConfig.ip).Str("peerPort", peerConfig.port).Msg("[Sync]GetBlockChainHeight failed")
				syncConfig.RemovePeer(peerConfig)
				return
			}
			utils.Logger().Info().Str("peerIP", peerConfig.ip).Uint64("blockHeight", response.BlockHeight).
				Msg("[SYNC] getMaxPeerHeight")

			lock.Lock()
			if response != nil {
				if maxHeight == uint64(math.MaxUint64) || maxHeight < response.BlockHeight {
					maxHeight = response.BlockHeight
				}
			}
			lock.Unlock()
		}()
		return
	})
	wg.Wait()
	return maxHeight
}
