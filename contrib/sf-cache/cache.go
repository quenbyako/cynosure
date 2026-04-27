package cache

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

const (
	defaultMaxAttempts = 3
)

// ConstructorFunc defines a function that initializes a new value for a given key.
type ConstructorFunc[K comparable, V any] func(context.Context, K) (V, error)

// DestructorFunc defines a function that cleans up a value when it is evicted from the cache.
type DestructorFunc[K comparable, V any] func(K, V)

// Cache provides a thread-safe LRU cache with duplicate function suppression (single-flight)
// for cache misses. It is designed to handle "thundering herd" scenarios where multiple
// concurrent requests miss the cache for the same key.
type Cache[K comparable, V any] struct {
	lru         *expirable.LRU[K, *cacheEntry[K, V]]
	constructor ConstructorFunc[K, V]
	destructor  DestructorFunc[K, V]
	mu          sync.Mutex // Protects LRU access
	closed      atomic.Bool
	maxAttempts int
}

// New creates a new Cache with the specified parameters.
// maxSize must be greater than 0.
func New[K comparable, V any](
	constructor ConstructorFunc[K, V],
	destructor DestructorFunc[K, V],
	maxSize uint,
	ttl time.Duration,
) *Cache[K, V] {
	if maxSize == 0 {
		//nolint:forbidigo // We specifically panic if maxSize is 0.
		panic("maxSize must be greater than 0")
	}

	lruSize := math.MaxInt
	if maxSize < math.MaxInt {
		lruSize = int(maxSize)
	}

	return &Cache[K, V]{
		lru: expirable.NewLRU(lruSize, func(_ K, entry *cacheEntry[K, V]) {
			entry.destruct(destructor)
		}, ttl),
		constructor: constructor,
		destructor:  destructor,
		mu:          sync.Mutex{},
		closed:      atomic.Bool{},
		maxAttempts: defaultMaxAttempts,
	}
}

// Close stops the cache and clears all entries.
func (c *Cache[K, V]) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return ErrClosed
	}

	c.mu.Lock()
	// Manually destruct all remaining items because Resize/Purge might not
	// trigger callbacks synchronously or at all for some items.
	for _, entry := range c.lru.Values() {
		entry.destruct(c.destructor)
	}

	c.lru.Purge()
	c.mu.Unlock()

	return nil
}

// Get retrieves a value from the cache for the given key. If the key is not present,
// it uses the constructor to create a new value. Multiple concurrent calls for the
// same missing key will only result in one constructor call.
func (c *Cache[K, V]) Get(ctx context.Context, key K) (V, error) {
	if c.closed.Load() {
		var zero V

		return zero, ErrClosed
	}

	for range c.maxAttempts {
		entry := c.getOrCreateEntry(key)

		val, err := entry.retrieve(ctx)
		if errors.Is(err, ErrEvicted) {
			continue
		}

		return val, err
	}

	return *new(V), fmt.Errorf("key %v repeatedly evicted: %w", key, ErrEvicted)
}

func (c *Cache[K, V]) getOrCreateEntry(key K) *cacheEntry[K, V] {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.lru.Get(key)
	if !ok {
		entry = &cacheEntry[K, V]{ //nolint:exhaustruct
			constructor: c.constructor,
			key:         key,
			group: group[V]{ //nolint:exhaustruct
				mu: sync.Mutex{},
			},
			mu: sync.RWMutex{},
		}
		c.lru.Add(key, entry)
	}

	return entry
}

// cacheEntry wraps a single-flight Group to ensure that concurrent requests for the
// same key are suppressed. It also handles cleanup when the entry is evicted.
type cacheEntry[K comparable, V any] struct {
	constructor ConstructorFunc[K, V]
	key         K
	group       group[V]
	mu          sync.RWMutex
	destructed  atomic.Bool
}

func (e *cacheEntry[K, V]) destruct(destructFunc DestructorFunc[K, V]) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.destructed.CompareAndSwap(false, true) {
		// Prevent double-destruction.
		return
	}

	val, err, ok := e.group.Get()
	if ok && err == nil {
		destructFunc(e.key, val)
	}
}

func (e *cacheEntry[K, V]) retrieve(ctx context.Context) (V, error) {
	e.mu.RLock()

	if e.destructed.Load() {
		e.mu.RUnlock()

		var zero V

		return zero, ErrEvicted
	}

	val, err, _ := e.group.Do(ctx, func(ctx context.Context) (V, error) {
		return e.constructor(ctx, e.key)
	})
	e.mu.RUnlock()

	if err != nil {
		// If constructor failed, forget the result so the next attempt can retry.
		e.mu.Lock()
		e.group.Forget()
		e.mu.Unlock()
	}

	return val, err
}
