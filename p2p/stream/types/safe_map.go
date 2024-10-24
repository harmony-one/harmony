package sttypes

import (
	"sort"
	"sync"
)

// SafeMap is a thread-safe map with its own lock for reading and writing.
type SafeMap[K comparable, V any] struct {
	data map[K]V
	mu   sync.RWMutex
}

// NewSafeMap initializes and returns a new SafeMap.
func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		data: make(map[K]V),
	}
}

// NewSafeMapWithInitialValues creates a new SafeMap with optional initial values.
func NewSafeMapWithInitialValues[K comparable, V any](initialValues map[K]V) *SafeMap[K, V] {
	m := &SafeMap[K, V]{
		data: make(map[K]V),
	}
	if initialValues != nil {
		m.mu.Lock()
		defer m.mu.Unlock()
		for k, v := range initialValues {
			m.data[k] = v
		}
	}
	return m
}

// Clone creates and returns a deep copy of the SafeMap
func (m *SafeMap[K, V]) Clone() *SafeMap[K, V] {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := NewSafeMap[K, V]()
	for k, v := range m.data {
		clone.data[k] = v
	}

	return clone
}

// Snapshot returns a copy of the underlying map
func (m *SafeMap[K, V]) Snapshot() map[K]V {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy of the internal map for safe iteration
	snapshot := make(map[K]V, len(m.data))
	for k, v := range m.data {
		snapshot[k] = v
	}
	return snapshot
}

// Set inserts or updates a key-value pair in the map.
func (m *SafeMap[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

// Get retrieves the value for a given key. It returns the value and a boolean indicating if the key exists.
func (m *SafeMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, exists := m.data[key]
	return val, exists
}

// Delete removes a key-value pair from the map.
func (m *SafeMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

// Exists checks if a key exists in the map.
func (m *SafeMap[K, V]) Exists(key K) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.data[key]
	return exists
}

// Keys returns a slice of all keys in the map.
func (m *SafeMap[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]K, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys
}

// Length returns the number of key-value pairs in the map.
func (m *SafeMap[K, V]) Length() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

func (m *SafeMap[K, V]) Iterate(f func(key K, value V)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for key, value := range m.data {
		f(key, value)
	}
}

// Clear removes all key-value pairs from the map.
func (m *SafeMap[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[K]V) // Reinitialize the map
}

// SortKeys returns a sorted clone of SafeMap.
func (m *SafeMap[K, V]) Sort(less func(i, j K) bool) *SafeMap[K, V] {
	keys := m.Keys()
	sort.Slice(keys, func(i, j int) bool {
		return less(keys[i], keys[j])
	})
	// Create a sorted copy of the map
	m.mu.RLock()
	defer m.mu.RUnlock()
	sorted := NewSafeMap[K, V]()
	for _, k := range keys {
		sorted.data[k] = m.data[k]
	}
	return sorted
}

// SortedSnapshot returns a sorted copy of the underlying map
func (m *SafeMap[K, V]) SortedSnapshot(less func(i, j K) bool) map[K]V {
	keys := m.Keys()
	sort.Slice(keys, func(i, j int) bool {
		return less(keys[i], keys[j])
	})
	// Create a sorted copy of the map
	m.mu.RLock()
	defer m.mu.RUnlock()
	sorted := make(map[K]V, len(m.data))
	for _, k := range keys {
		sorted[k] = m.data[k]
	}
	return sorted
}
