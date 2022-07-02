package keylocker

import "testing"

func TestKeyLocker_Sequential(t *testing.T) {
	n := New()
	key := 1
	v := make(chan struct{}, 1)
	n.Lock(key, func() (interface{}, error) {
		v <- struct{}{}
		return nil, nil
	})
	<-v // we know that goroutine really executed
	n.Lock(key, func() (interface{}, error) {
		return nil, nil
	})
}
