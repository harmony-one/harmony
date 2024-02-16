package lrucache

import lru "github.com/hashicorp/golang-lru"

type Cache[K comparable, V any] struct {
	cache *lru.Cache
}

func NewCache[K comparable, V any](size int) *Cache[K, V] {
	c, _ := lru.New(size)
	return &Cache[K, V]{
		cache: c,
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	v, ok := c.cache.Get(key)
	if !ok {
		var out V
		return out, false
	}
	return v.(V), true
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.cache.Add(key, value)
}

// Contains checks if a key is in the cache, without updating the
// recent-ness or deleting it for being stale.
func (c *Cache[K, V]) Contains(key K) bool {
	return c.cache.Contains(key)
}

func (c *Cache[K, V]) Len() int {
	return c.cache.Len()
}

func (c *Cache[K, V]) Keys() []K {
	out := make([]K, 0, c.cache.Len())
	for _, v := range c.cache.Keys() {
		out = append(out, v.(K))
	}
	return out
}
