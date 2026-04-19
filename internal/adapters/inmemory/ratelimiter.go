// Package inmemory provides simple in-memory implementations.
package inmemory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quenbyako/core"
	"golang.org/x/time/rate"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/adapters/inmemory"
)

// userEntry stores the rate limiter and the last seen timestamp for a user.
type userEntry struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64
}

// RateLimiter is an in-memory implementation of the ratelimiter.Port.
type RateLimiter struct {
	tracer     ports.ObserveStack
	now        func() time.Time
	entries    map[ids.UserID]*userEntry
	limit      rate.Limit
	ttl        time.Duration
	burst      int
	entiresMux sync.RWMutex
}

var (
	_ ratelimiter.PortFactory = (*RateLimiter)(nil)
	_ ratelimiter.Port        = (*RateLimiter)(nil)
)

type clock = func() time.Time

// NewRateLimiter creates a new in-memory rate limiter.
func NewRateLimiter(
	limit rate.Limit,
	burst int,
	ttl time.Duration,
	now clock,
	tracer core.Metrics,
) *RateLimiter {
	if now == nil {
		now = time.Now
	}

	observability := ports.NoOpObserveStack()
	if tracer != nil {
		observability = ports.StackFromCore(tracer, pkgName)
	}

	return &RateLimiter{
		limit:      limit,
		burst:      burst,
		ttl:        ttl,
		now:        now,
		entiresMux: sync.RWMutex{},
		entries:    make(map[ids.UserID]*userEntry),
		tracer:     observability,
	}
}

// RateLimiter returns ratelimiter.PortWrapped interface.
//
//nolint:ireturn // it's a port interface
func (r *RateLimiter) RateLimiter() ratelimiter.PortWrapped { return ratelimiter.Wrap(r, r.tracer) }

// Consume consumes message quota for the given user.
func (r *RateLimiter) Consume(ctx context.Context, user ids.UserID, count int) error {
	now := r.now()

	r.entiresMux.RLock()
	entry, ok := r.entries[user]
	r.entiresMux.RUnlock()

	if !ok {
		r.entiresMux.Lock()
		// Double-check after acquiring write lock
		entry, ok = r.entries[user]
		if !ok {
			entry = &userEntry{
				limiter:  rate.NewLimiter(r.limit, r.burst),
				lastSeen: atomic.Int64{},
			}
			// must set while locking to prevent leacing entry in zero value.
			entry.lastSeen.Store(now.UnixNano())
			r.entries[user] = entry
		}
		r.entiresMux.Unlock()
	}

	entry.lastSeen.Store(now.UnixNano())

	if !entry.limiter.AllowN(now, count) {
		return fmt.Errorf("%w for user %q", ratelimiter.ErrRateLimitExceeded, user.ID().String())
	}

	return nil
}

const (
	tickerPeriodFactor = 2
)

// Cleanup periodically removes stale rate limiters from memory.
func (r *RateLimiter) Cleanup(ctx context.Context) error {
	if r.ttl <= 0 {
		return nil
	}

	ticker := time.NewTicker(r.ttl / tickerPeriodFactor)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.Canceled) {
				// returnin nil, cause cleanup is optional and it doesn't affect
				// on logic so much.
				return nil
			}

			return fmt.Errorf("cleanup job: %w", err)

		case <-ticker.C:
			r.evictStaleEntries()
		}
	}
}

func (r *RateLimiter) evictStaleEntries() {
	now := r.now().UnixNano()
	r.entiresMux.Lock()
	defer r.entiresMux.Unlock()

	for user, entry := range r.entries {
		lastSeen := entry.lastSeen.Load()
		if lastSeen != 0 && now-lastSeen > int64(r.ttl) {
			delete(r.entries, user)
		}
	}
}
