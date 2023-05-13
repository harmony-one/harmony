package knownpeers

import (
	"sync"

	"github.com/harmony-one/harmony/p2p"
)

type threadSafe struct {
	mu       sync.Mutex
	internal KnownPeers
}

func WrapThreadSafe(hosts KnownPeers) KnownPeers {
	return &threadSafe{internal: hosts}
}

func (a *threadSafe) GetChecked(limit int) []p2p.Peer {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.internal.GetChecked(limit)
}

func (a *threadSafe) GetUnchecked(limit int) []p2p.Peer {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.internal.GetUnchecked(limit)
}

func (a *threadSafe) AddChecked(hosts ...p2p.Peer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.internal.AddChecked(hosts...)
}

func (a *threadSafe) AddUnchecked(hosts ...p2p.Peer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.internal.AddUnchecked(hosts...)
}

func (a *threadSafe) GetUncheckedCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.internal.GetUncheckedCount()
}
