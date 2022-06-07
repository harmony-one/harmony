package p2p

import "sync"

type ConnectCallbacks struct {
	cbs []ConnectCallback
	mu  sync.RWMutex
}

func (a *ConnectCallbacks) Add(cb ConnectCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cbs = append(a.cbs, cb)
}

func (a *ConnectCallbacks) GetAll() []ConnectCallback {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]ConnectCallback, len(a.cbs))
	copy(out, a.cbs)
	return out
}

type DisconnectCallbacks struct {
	cbs []DisconnectCallback
	mu  sync.RWMutex
}

func (a *DisconnectCallbacks) Add(cb DisconnectCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cbs = append(a.cbs, cb)
}

func (a *DisconnectCallbacks) GetAll() []DisconnectCallback {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]DisconnectCallback, len(a.cbs))
	copy(out, a.cbs)
	return out
}
