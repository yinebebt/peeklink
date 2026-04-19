package main

import (
	"sync"
	"time"
)

// Cache is a generic in-memory TTL cache, safe for concurrent use.
type Cache[T any] struct {
	mu    sync.RWMutex
	items map[string]cacheEntry[T]
}

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

// NewCache constructs a TTL cache and starts a background eviction loop.
//
// cleanupInterval controls how often expired entries are removed. If the value
// is zero or negative, a conservative default is used instead of panicking.
func NewCache[T any](cleanupInterval time.Duration) *Cache[T] {
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute
	}
	c := &Cache[T]{items: make(map[string]cacheEntry[T])}
	go c.evictLoop(cleanupInterval)
	return c
}

// Get returns the cached value for a key when it is still fresh.
func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(e.expiresAt) {
		var zero T
		return zero, false
	}
	return e.value, true
}

// Set stores value under a key for the given TTL.
func (c *Cache[T]) Set(key string, value T, ttl time.Duration) {
	c.mu.Lock()
	c.items[key] = cacheEntry[T]{value: value, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

func (c *Cache[T]) evictLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.items {
			if now.After(e.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
