package statedb_cache

import (
	"context"
	"log"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/ethereum/go-ethereum/ethdb"
	redis "github.com/go-redis/redis/v8"
	"github.com/harmony-one/harmony/internal/tikv/common"
)

const (
	maxMemoryEntrySize      = 64 * 1024
	maxPipelineEntriesCount = 10000
	cacheMinGapInSecond     = 600
)

type cacheWrapper struct {
	*fastcache.Cache
}

// Put inserts the given value into the key-value data store.
func (c *cacheWrapper) Put(key []byte, value []byte) error {
	c.Cache.Set(key, value)
	return nil
}

// Delete removes the key from the key-value data store.
func (c *cacheWrapper) Delete(key []byte) error {
	c.Cache.Del(key)
	return nil
}

type redisPipelineWrapper struct {
	redis.Pipeliner

	expiredTime time.Duration
}

// Put inserts the given value into the key-value data store.
func (c *redisPipelineWrapper) Put(key []byte, value []byte) error {
	c.SetEX(context.Background(), string(key), value, c.expiredTime)
	return nil
}

// Delete removes the key from the key-value data store.
func (c *redisPipelineWrapper) Delete(key []byte) error {
	c.Del(context.Background(), string(key))
	return nil
}

type StateDBCacheConfig struct {
	CacheSizeInMB        uint32
	CachePersistencePath string
	RedisServerAddr      []string
	RedisLRUTimeInDay    uint32
	DebugHitRate         bool
}

type StateDBCacheDatabase struct {
	config          StateDBCacheConfig
	enableReadCache bool
	isClose         uint64
	remoteDB        common.TiKVStore

	// l1cache (memory)
	l1Cache *cacheWrapper

	// l2cache (redis)
	l2Cache           *redis.ClusterClient
	l2ReadOnly        bool
	l2ExpiredTime     time.Duration
	l2NextExpiredTime time.Time
	l2ExpiredRefresh  chan string

	// stats
	l1HitCount           uint64
	l2HitCount           uint64
	missCount            uint64
	notFoundOrErrorCount uint64
}

func NewStateDBCacheDatabase(remoteDB common.TiKVStore, config StateDBCacheConfig, readOnly bool) (*StateDBCacheDatabase, error) {
	// create or load memory cache from file
	cache := fastcache.LoadFromFileOrNew(config.CachePersistencePath, int(config.CacheSizeInMB)*1024*1024)

	// create options
	option := &redis.ClusterOptions{
		Addrs:       config.RedisServerAddr,
		PoolSize:    32,
		IdleTimeout: 60 * time.Second,
	}

	if readOnly {
		option.PoolSize = 16
		option.ReadOnly = true
	}

	// check redis connection
	rdb := redis.NewClusterClient(option)
	err := rdb.Ping(context.Background()).Err()
	if err != nil {
		return nil, err
	}

	// reload redis cluster slots status
	rdb.ReloadState(context.Background())

	db := &StateDBCacheDatabase{
		config:          config,
		enableReadCache: true,
		remoteDB:        remoteDB,
		l1Cache:         &cacheWrapper{cache},
		l2Cache:         rdb,
		l2ReadOnly:      readOnly,
		l2ExpiredTime:   time.Duration(config.RedisLRUTimeInDay) * 24 * time.Hour,
	}

	if !readOnly {
		// Read a copy of the memory hit data into redis to improve the hit rate
		// refresh read time to prevent recycling by lru
		db.l2ExpiredRefresh = make(chan string, 100000)
		go db.startL2ExpiredRefresh()
	}

	// print debug info
	if config.DebugHitRate {
		go func() {
			for range time.Tick(5 * time.Second) {
				db.cacheStatusPrint()
			}
		}()
	}

	return db, nil
}

func (c *StateDBCacheDatabase) AncientDatadir() (string, error) {
	return "", nil
}

func (c *StateDBCacheDatabase) NewBatchWithSize(size int) ethdb.Batch {
	return nil
}

// Has retrieves if a key is present in the key-value data store.
func (c *StateDBCacheDatabase) Has(key []byte) (bool, error) {
	return c.remoteDB.Has(key)
}

// Get retrieves the given key if it's present in the key-value data store.
func (c *StateDBCacheDatabase) Get(key []byte) (ret []byte, err error) {
	if c.enableReadCache {
		var ok bool

		// first, get data from memory cache
		keyStr := string(key)
		if ret, ok = c.l1Cache.HasGet(nil, key); ok {
			if !c.l2ReadOnly {
				select {
				// refresh read time to prevent recycling by lru
				case c.l2ExpiredRefresh <- keyStr:
				default:
				}
			}

			atomic.AddUint64(&c.l1HitCount, 1)
			return ret, nil
		}

		defer func() {
			if err == nil {
				if len(ret) < maxMemoryEntrySize {
					// set data to memory db if loaded from redis cache or leveldb
					c.l1Cache.Set(key, ret)
				}
			}
		}()

		timeoutCtx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelFunc()

		// load data from redis cache
		data := c.l2Cache.Get(timeoutCtx, keyStr)
		if data.Err() == nil {
			atomic.AddUint64(&c.l2HitCount, 1)
			return data.Bytes()
		} else if data.Err() != redis.Nil {
			log.Printf("[Get WARN]Redis Error: %v", data.Err())
		}

		if !c.l2ReadOnly {
			defer func() {
				// set data to redis db if loaded from leveldb
				if err == nil {
					c.l2Cache.SetEX(context.Background(), keyStr, ret, c.l2ExpiredTime)
				}
			}()
		}
	}

	ret, err = c.remoteDB.Get(key)
	if err != nil {
		atomic.AddUint64(&c.notFoundOrErrorCount, 1)
	} else {
		atomic.AddUint64(&c.missCount, 1)
	}

	return
}

// Put inserts the given value into the key-value data store.
func (c *StateDBCacheDatabase) Put(key []byte, value []byte) (err error) {
	if c.enableReadCache {
		defer func() {
			if err == nil {
				if len(value) < maxMemoryEntrySize {
					// memory db only accept the small data
					c.l1Cache.Set(key, value)
				}
				if !c.l2ReadOnly {
					timeoutCtx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancelFunc()

					// send the data to redis
					res := c.l2Cache.SetEX(timeoutCtx, string(key), value, c.l2ExpiredTime)
					if res.Err() == nil {
						log.Printf("[Put WARN]Redis Error: %v", res.Err())
					}
				}
			}
		}()
	}

	return c.remoteDB.Put(key, value)
}

// Delete removes the key from the key-value data store.
func (c *StateDBCacheDatabase) Delete(key []byte) (err error) {
	if c.enableReadCache {
		defer func() {
			if err == nil {
				c.l1Cache.Del(key)
				if !c.l2ReadOnly {
					c.l2Cache.Del(context.Background(), string(key))
				}
			}
		}()
	}

	return c.remoteDB.Delete(key)
}

// NewBatch creates a write-only database that buffers changes to its host db
// until a final write is called.
func (c *StateDBCacheDatabase) NewBatch() ethdb.Batch {
	return newStateDBCacheBatch(c, c.remoteDB.NewBatch())
}

// Stat returns a particular internal stat of the database.
func (c *StateDBCacheDatabase) Stat(property string) (string, error) {
	switch property {
	case "tikv.save.cache":
		_ = c.l1Cache.SaveToFileConcurrent(c.config.CachePersistencePath, runtime.NumCPU())
	}
	return c.remoteDB.Stat(property)
}

func (c *StateDBCacheDatabase) Compact(start []byte, limit []byte) error {
	return c.remoteDB.Compact(start, limit)
}

func (c *StateDBCacheDatabase) NewIterator(start, end []byte) ethdb.Iterator {
	return c.remoteDB.NewIterator(start, end)
}

// Close disconnect the redis and save memory cache to file
func (c *StateDBCacheDatabase) Close() error {
	if atomic.CompareAndSwapUint64(&c.isClose, 0, 1) {
		defer c.remoteDB.Close()
		defer c.l2Cache.Close()

		err := c.l1Cache.SaveToFileConcurrent(c.config.CachePersistencePath, runtime.NumCPU())
		if err != nil {
			log.Printf("save file to '%s' error: %v", c.config.CachePersistencePath, err)
		}

		if !c.l2ReadOnly {
			time.Sleep(time.Second)
			close(c.l2ExpiredRefresh)

			for range c.l2ExpiredRefresh {
				// nop, clear chan
			}
		}
	}

	return nil
}

// cacheWrite write batch to cache
func (c *StateDBCacheDatabase) cacheWrite(b ethdb.Batch) error {
	if !c.l2ReadOnly {
		pipeline := c.l2Cache.Pipeline()

		err := b.Replay(&redisPipelineWrapper{Pipeliner: pipeline, expiredTime: c.l2ExpiredTime})
		if err != nil {
			return err
		}

		_, err = pipeline.Exec(context.Background())
		if err != nil {
			log.Printf("[BatchWrite WARN]Redis Error: %v", err)
		}
	}

	return b.Replay(c.l1Cache)
}

func (c *StateDBCacheDatabase) refreshL2ExpiredTime() {
	unix := time.Now().Add(c.l2ExpiredTime).Unix()
	unix = unix - (unix % cacheMinGapInSecond) + cacheMinGapInSecond
	c.l2NextExpiredTime = time.Unix(unix, 0)
}

// startL2ExpiredRefresh batch refresh redis cache
func (c *StateDBCacheDatabase) startL2ExpiredRefresh() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	pipeline := c.l2Cache.Pipeline()
	lastWrite := time.Now()

	for {
		select {
		case key, ok := <-c.l2ExpiredRefresh:
			if !ok {
				return
			}

			pipeline.ExpireAt(context.Background(), key, c.l2NextExpiredTime)
			if pipeline.Len() > maxPipelineEntriesCount {
				_, _ = pipeline.Exec(context.Background())
				lastWrite = time.Now()
			}
		case now := <-ticker.C:
			c.refreshL2ExpiredTime()

			if pipeline.Len() > 0 && now.Sub(lastWrite) > time.Second {
				_, _ = pipeline.Exec(context.Background())
				lastWrite = time.Now()
			}
		}
	}
}

// print stats to stdout
func (c *StateDBCacheDatabase) cacheStatusPrint() {
	var state = &fastcache.Stats{}
	state.Reset()
	c.l1Cache.UpdateStats(state)

	stats := c.l2Cache.PoolStats()
	log.Printf("redis: TotalConns: %d, IdleConns: %d, StaleConns: %d", stats.TotalConns, stats.IdleConns, stats.StaleConns)
	total := float64(c.l1HitCount + c.l2HitCount + c.missCount + c.notFoundOrErrorCount)

	log.Printf(
		"total: l1Hit: %d(%.2f%%), l2Hit: %d(%.2f%%), miss: %d(%.2f%%), notFoundOrErrorCount: %d, fastcache: GetCalls: %d, SetCalls: %d, Misses: %d, EntriesCount: %d, BytesSize: %d",
		c.l1HitCount, float64(c.l1HitCount)/total*100, c.l2HitCount, float64(c.l2HitCount)/total*100, c.missCount, float64(c.missCount)/total*100, c.notFoundOrErrorCount,
		state.GetCalls, state.SetCalls, state.Misses, state.EntriesCount, state.BytesSize,
	)
}

func (c *StateDBCacheDatabase) ReplayCache(batch ethdb.Batch) error {
	return c.cacheWrite(batch)
}
