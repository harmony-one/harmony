package stagedsync

import (
	"math"
	"sync"
	"time"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

const (
	// syncStatusExpiration is the expiration time out of a sync status.
	// If last sync result in memory is before the expiration, the sync status
	// will be updated.
	syncStatusExpiration = 6 * time.Second

	// syncStatusExpirationNonValidator is the expiration of sync cache for non-validators.
	// Compared with non-validator, the sync check is not as strict as validator nodes.
	// TODO: add this field to harmony config
	syncStatusExpirationNonValidator = 12 * time.Second
)

type (
	syncStatus struct {
		lastResult     SyncCheckResult
		MaxPeersHeight uint64
		currentCycle   SyncCycle
		lastUpdateTime time.Time
		lock           sync.RWMutex
		expiration     time.Duration
	}

	SyncCheckResult struct {
		IsSynchronized bool
		OtherHeight    uint64
		HeightDiff     uint64
	}

	SyncCycle struct {
		Number       uint64
		StartHash    []byte
		TargetHeight uint64
		ExtraHashes  map[uint64][]byte
		lock         sync.RWMutex
	}
)

func NewSyncStatus(role nodeconfig.Role) syncStatus {
	expiration := getSyncStatusExpiration(role)
	return syncStatus{
		expiration: expiration,
	}
}

func getSyncStatusExpiration(role nodeconfig.Role) time.Duration {
	switch role {
	case nodeconfig.Validator:
		return syncStatusExpiration
	case nodeconfig.ExplorerNode:
		return syncStatusExpirationNonValidator
	default:
		return syncStatusExpirationNonValidator
	}
}

func (status *syncStatus) Get(fallback func() SyncCheckResult) SyncCheckResult {
	status.lock.RLock()
	if !status.expired() {
		result := status.lastResult
		status.lock.RUnlock()
		return result
	}
	status.lock.RUnlock()

	status.lock.Lock()
	defer status.lock.Unlock()
	if status.expired() {
		result := fallback()
		if result.OtherHeight > 0 && result.OtherHeight < uint64(math.MaxUint64) {
			status.update(result)
		}
	}
	return status.lastResult
}

func (status *syncStatus) expired() bool {
	return time.Since(status.lastUpdateTime) > status.expiration
}

func (status *syncStatus) update(result SyncCheckResult) {
	status.lastUpdateTime = time.Now()
	status.lastResult = result
}
