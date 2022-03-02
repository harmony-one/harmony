package local_cache

import (
	"bytes"
	"time"

	"github.com/allegro/bigcache"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/utils"
)

type cacheWrapper struct {
	*bigcache.BigCache
}

type CacheConfig struct {
	CacheTime time.Duration
	CacheSize int
}

func (c *cacheWrapper) Put(key []byte, value []byte) error {
	return c.BigCache.Set(String(key), value)
}

func (c *cacheWrapper) Delete(key []byte) error {
	return c.BigCache.Delete(String(key))
}

type LocalCacheDatabase struct {
	ethdb.KeyValueStore

	enableReadCache bool

	deleteMap map[string]bool
	readCache *cacheWrapper
}

func NewLocalCacheDatabase(remoteDB ethdb.KeyValueStore, cacheConfig CacheConfig) *LocalCacheDatabase {
	config := bigcache.DefaultConfig(cacheConfig.CacheTime)
	config.HardMaxCacheSize = cacheConfig.CacheSize
	config.MaxEntriesInWindow = cacheConfig.CacheSize * 4 * int(cacheConfig.CacheTime.Seconds())
	cache, _ := bigcache.NewBigCache(config)

	db := &LocalCacheDatabase{
		KeyValueStore: remoteDB,

		enableReadCache: true,
		deleteMap:       make(map[string]bool),
		readCache:       &cacheWrapper{cache},
	}

	go func() {
		for range time.Tick(time.Minute) {
			utils.Logger().Info().
				Interface("stats", cache.Stats()).
				Int("count", cache.Len()).
				Int("size", cache.Capacity()).
				Msg("local-cache stats")
		}
	}()

	return db
}

func (c *LocalCacheDatabase) Has(key []byte) (bool, error) {
	return c.KeyValueStore.Has(key)
}

func (c *LocalCacheDatabase) Get(key []byte) (ret []byte, err error) {
	if c.enableReadCache {
		if bytes.Compare(key, []byte("LastBlock")) != 0 {
			strKey := String(key)
			ret, err = c.readCache.Get(strKey)
			if err == nil {
				return ret, nil
			}

			defer func() {
				if err == nil {
					_ = c.readCache.Set(strKey, ret)
				}
			}()
		}
	}

	return c.KeyValueStore.Get(key)
}

func (c *LocalCacheDatabase) Put(key []byte, value []byte) error {
	if c.enableReadCache {
		_ = c.readCache.Put(key, value)
	}

	return c.KeyValueStore.Put(key, value)
}

func (c *LocalCacheDatabase) Delete(key []byte) error {
	if c.enableReadCache {
		_ = c.readCache.Delete(key)
	}

	return c.KeyValueStore.Delete(key)
}

func (c *LocalCacheDatabase) NewBatch() ethdb.Batch {
	return newLocalCacheBatch(c)
}

func (c *LocalCacheDatabase) batchWrite(b *LocalCacheBatch) error {
	if c.enableReadCache {
		_ = b.Replay(c.readCache)
	}

	batch := c.KeyValueStore.NewBatch()
	err := b.Replay(batch)
	if err != nil {
		return err
	}

	return batch.Write()
}
