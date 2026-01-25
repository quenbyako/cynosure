package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type ConstructorFunc[K comparable, V any] func(context.Context, K) (V, error)
type DestructorFunc[K comparable, V any] func(K, V)

type Cache[K comparable, V any] struct {
	closed atomic.Bool

	constructor ConstructorFunc[K, V]
	lru         *expirable.LRU[K, *singleflight[K, V]]
	mu          sync.Mutex // Protects LRU operations
	maxAttempts int
}

func New[K comparable, V any](
	constructor ConstructorFunc[K, V],
	destructor DestructorFunc[K, V],
	maxSize uint,
	ttl time.Duration,
) *Cache[K, V] {
	if maxSize == 0 {
		panic("maxSize must be greater than 0")
	}

	return &Cache[K, V]{
		constructor: constructor,
		lru: expirable.NewLRU(int(maxSize), func(_ K, conn *singleflight[K, V]) {
			conn.destruct(destructor)
		}, ttl),
		mu:          sync.Mutex{},
		maxAttempts: 3,
	}
}

var (
	ErrClosed     = errors.New("closed")
	ErrDestructed = errors.New("entry was evicted from cache")
)

func (h *Cache[K, V]) Close() error {
	if !h.closed.CompareAndSwap(false, true) {
		return ErrClosed
	}

	h.mu.Lock()
	h.lru.Resize(0)
	h.mu.Unlock()

	return nil
}

func (h *Cache[K, V]) Get(ctx context.Context, k K) (V, error) {
	if h.closed.Load() {
		var zero V
		return zero, ErrClosed
	}

	// Retry loop to handle race between Get and LRU eviction
	for range h.maxAttempts {
		// Thread-safe LRU access: lock during Get+Add to prevent race conditions
		h.mu.Lock()
		conn, ok := h.lru.Get(k)
		if !ok {
			// Create new singleflight entry for this key
			conn = &singleflight[K, V]{k: k, constructor: h.constructor}
			h.lru.Add(k, conn)
		}
		h.mu.Unlock()

		// retrieve() handles its own synchronization
		v, err := conn.retrieve(ctx)

		// If entry was evicted between Get and retrieve, retry
		if errors.Is(err, ErrDestructed) {
			continue
		}

		return v, err
	}

	// If we still get ErrDestructed after retries, something is wrong
	var zero V
	return zero, errors.New("cache entry repeatedly evicted, possible cache thrashing")
}

type singleflight[K comparable, V any] struct {
	k           K
	constructor func(context.Context, K) (V, error)

	mu         sync.RWMutex
	destructed atomic.Bool
	group      group[V]
}

func (d *singleflight[K, V]) destruct(f func(K, V)) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.destructed.CompareAndSwap(false, true) {
		panic(errors.New("already destructed"))
	}

	defer d.group.Forget()
	if v, _, ok := d.group.Get(); ok {
		f(d.k, v)
	}
}

func (d *singleflight[K, V]) retrieve(ctx context.Context) (v V, err error) {
	d.mu.RLock()

	if d.destructed.Load() {
		d.mu.RUnlock()
		// Entry was evicted from cache, return error instead of panicking
		return v, ErrDestructed
	}

	res, err, _ := d.group.Do(ctx, func(ctx context.Context) (V, error) { return d.constructor(ctx, d.k) })
	d.mu.RUnlock()

	// If constructor failed, forget the result so next attempt creates a fresh client.
	// This is critical for HTTP clients: a failed connection should be retried, not cached.
	// We call Forget() AFTER RUnlock because Do() has already completed and all waiting
	// goroutines have received their results.
	if err != nil {
		d.mu.Lock()
		d.group.Forget()
		d.mu.Unlock()
	}

	return res, err
}
