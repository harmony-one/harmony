package keylocker

import "sync"

type KeyLocker struct {
	m sync.Map
}

func New() *KeyLocker {
	return &KeyLocker{}
}

func (a *KeyLocker) Lock(key interface{}, f func() (interface{}, error)) (interface{}, error) {
	mu := &sync.Mutex{}
	for {
		actual, _ := a.m.LoadOrStore(key, mu)
		mu2 := actual.(*sync.Mutex)
		mu.Lock()
		if mu2 != mu {
			// acquired someone else lock.
			mu.Unlock()
			continue
		}
		rs, err := f()
		mu.Unlock()
		a.m.Delete(key)
		return rs, err
	}

}
