package blockedpeers

import (
	"github.com/harmony-one/harmony/internal/utils/lrucache"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"time"
)

type Manager struct {
	internal *lrucache.Cache[libp2p_peer.ID, time.Time]
}

func NewManager(size int) *Manager {
	return &Manager{
		internal: lrucache.NewCache[libp2p_peer.ID, time.Time](size),
	}
}

func (m *Manager) IsBanned(key libp2p_peer.ID, now time.Time) bool {
	future, ok := m.internal.Get(key)

	if ok {
		return future.After(now) // future > now
	}
	return ok
}

func (m *Manager) Ban(key libp2p_peer.ID, future time.Time) {
	m.internal.Set(key, future)
}

func (m *Manager) Contains(key libp2p_peer.ID) bool {
	return m.internal.Contains(key)
}

func (m *Manager) Len() int {
	return m.internal.Len()
}

func (m *Manager) Keys() []libp2p_peer.ID {
	return m.internal.Keys()
}
