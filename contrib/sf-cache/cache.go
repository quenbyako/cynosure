package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type Cache[K comparable, V any] struct {
	closed atomic.Bool

	constructor func(context.Context, K) (V, error)
	lru         *expirable.LRU[K, *singleflight[K, V]]
}

func New[K comparable, V any](
	constructor func(context.Context, K) (V, error),
	destructor func(K, V),
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
	}
}

var (
	ErrClosed = errors.New("closed")
)

func (h *Cache[K, V]) Close() error {
	if !h.closed.CompareAndSwap(false, true) {
		return ErrClosed
	}

	h.lru.Resize(0)

	return nil
}

func (h *Cache[K, V]) Get(ctx context.Context, k K) (V, error) {
	if h.closed.Load() {
		var zero V
		return zero, ErrClosed
	}

	// TODO: VERY VERY IMPORTANT: method is not thread-safe. We have to rewrite
	// it differently, for test — meh, but ok
	conn, ok := h.lru.Get(k)
	if !ok {
		h.lru.Add(k, &singleflight[K, V]{k: k, constructor: h.constructor})
	}

	conn, ok = h.lru.Get(k)
	if !ok {
		panic("unreachable")
	}

	return conn.retrieve(ctx)
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
	defer d.mu.RUnlock()

	if d.destructed.Load() {
		panic("destructed")
	}

	res, err, _ := d.group.Do(ctx, func(ctx context.Context) (V, error) { return d.constructor(ctx, d.k) })
	if err != nil {
		d.group.Forget() // Forget the group if it failed
		return v, err
	}

	return res, nil
}
