package caches

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2"
)

// GenericCache defines the behaviour for a key-value cache that supports storing
// and retrieving values, with optional TTL-based expiration.
//
// Type Parameters:
//   - T: the type of the cached values.
type GenericCache[T any] interface {
	// Get retrieves the value associated with the given key.
	//
	// Parameters:
	//   - key: the key to look up in the cache
	//
	// Returns:
	//   - T: the value stored under the key (zero value if not found or expired)
	//   - bool: true if the key was found and not expired, false otherwise
	Get(key string) (T, bool)

	// GetWithStatus retrieves the value for a key along with status flags indicating
	// whether the key exists and whether it is expired.
	//
	// Parameters:
	//	 - key: the key to look up in the cache.
	//
	// Returns:
	//	 - T: the stored value, or the zero value if the key is missing.
	//	 - bool: true if the key exists in the cache (even if expired), false otherwise
	//	 - bool: true if the key exists but the value is expired, false otherwise.
	GetWithStatus(key string) (T, bool, bool)

	// Set stores the given value under the provided key.
	//
	// Parameters:
	//   - key: the key under which to store the value
	//   - value: the value to store
	Set(key string, value T)
}

// cacheItem represents a cached value along with the time it was stored.
// The timestamp is used to enforce TTL (time-to-live) expiration policies.
type cacheItem[T any] struct {
	Value     T
	Timestamp time.Time
}

// BaseCache is a GenericCache implementation that wraps an LRU cache
// and enforces TTL-based expiration of entries.
//
// If ttl is zero, items never expire and eviction is determined solely
// by the LRU policy.
type BaseCache[T any] struct {
	cache *lru.Cache[string, cacheItem[T]]
	ttl   time.Duration
	mu    sync.RWMutex
}

// NewBaseCache creates a new BaseCache with the given maximum size and TTL.
//
// Parameters:
//   - size: the maximum number of entries before LRU eviction
//   - ttl: the time-to-live for items (zero disables expiration)
//
// Returns:
//   - *BaseCache[T]: the newly created cache
//   - error: non-nil if the cache could not be created
func NewBaseCache[T any](size int, ttl time.Duration) (*BaseCache[T], error) {
	cache, err := lru.New[string, cacheItem[T]](size)
	if err != nil {
		return nil, err
	}
	return &BaseCache[T]{
		cache: cache,
		ttl:   ttl,
	}, nil
}

// NewBaseCacheWithEviction creates a new BaseCache with the given size, TTL, and
// a custom eviction function.
//
// Parameters:
//   - size: the maximum number of entries before LRU eviction
//   - ttl: the time-to-live for items (zero disables expiration)
//   - evictFn: a function called whenever an item is evicted (either due to LRU policy or manually removed)
//
// Returns:
//   - *BaseCache[T]: the newly created cache
//   - error: non-nil if the cache could not be created
func NewBaseCacheWithEviction[T any](size int, ttl time.Duration, onEvict func(string, T)) (*BaseCache[T], error) {
	internalEvict := func(key string, item cacheItem[T]) {
		if onEvict != nil {
			onEvict(key, item.Value)
		}
	}
	cache, err := lru.NewWithEvict[string, cacheItem[T]](size, internalEvict)
	if err != nil {
		return nil, err
	}

	return &BaseCache[T]{
		cache: cache,
		ttl:   ttl,
	}, nil
}

// Get retrieves the value associated with the given key if it exists and has
// not expired. Expired items are removed.
//
// Parameters:
//   - key: the key to look up
//
// Returns:
//   - T: the value stored under the key (zero value if not found or expired)
//   - bool: true if the key was found and not expired, false otherwise
func (c *BaseCache[T]) Get(key string) (T, bool) {
	var zero T
	c.mu.RLock()

	if item, ok := c.cache.Get(key); ok {
		if c.ttl == 0 || time.Since(item.Timestamp) < c.ttl {
			c.mu.RUnlock()
			return item.Value, true
		}

		c.mu.RUnlock()
		c.mu.Lock()
		defer c.mu.Unlock()

		if item, ok = c.cache.Get(key); ok {
			if time.Since(item.Timestamp) > c.ttl {
				c.cache.Remove(key)
				return zero, false
			} else {
				// Item was refreshed
				return item.Value, true
			}
		}
		return zero, false
	}

	c.mu.RUnlock()
	return zero, false
}

// Set stores the given value under the provided key, recording the current time.
//
// Parameters:
//   - key: the key under which to store the value
//   - value: the value to store
func (c *BaseCache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Add(key, cacheItem[T]{
		Value:     value,
		Timestamp: time.Now(),
	})
}

// GetWithStatus retrieves a value along with its expiration status.
//
// Unlike Get, this method does not remove expired items from the cache;
// it simply reports their status.
func (c *BaseCache[T]) GetWithStatus(key string) (T, bool, bool) {
	var zero T

	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.cache.Get(key)
	if !ok {
		return zero, false, false
	}

	if c.ttl == 0 || time.Since(item.Timestamp) < c.ttl {
		return item.Value, true, false
	}
	return item.Value, true, true
}
