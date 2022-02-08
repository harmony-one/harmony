package leveldb_shard

import (
	"hash/crc32"
	"sync"
	"sync/atomic"
)

func mapDBIndex(key []byte, dbCount uint32) uint32 {
	return crc32.ChecksumIEEE(key) % dbCount
}

func parallelRunAndReturnErr(parallelNum int, cb func(index int) error) error {
	wg := sync.WaitGroup{}
	errAtomic := atomic.Value{}

	for i := 0; i < parallelNum; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			err := cb(i)
			if err != nil {
				errAtomic.Store(err)
			}
		}(i)
	}

	wg.Wait()

	if err := errAtomic.Load(); err != nil {
		return errAtomic.Load().(error)
	} else {
		return nil
	}
}
