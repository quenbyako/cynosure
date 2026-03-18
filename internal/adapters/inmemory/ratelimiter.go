// Package inmemory provides simple in-memory implementations.
package inmemory

import (
	"context"
	"fmt"
	"sync"
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

// RateLimiter is an in-memory implementation of the ratelimiter.Port.
type RateLimiter struct {
	tracer   ports.ObserveStack
	limiters map[ids.UserID]*rate.Limiter
	now      func() time.Time
	limit    rate.Limit
	burst    int
	mu       sync.Mutex
}

var _ ratelimiter.PortFactory = (*RateLimiter)(nil)

type clock = func() time.Time

// NewRateLimiter creates a new in-memory rate limiter.
func NewRateLimiter(limit rate.Limit, burst int, now clock, tracer core.Metrics) *RateLimiter {
	if now == nil {
		now = time.Now
	}

	observability := ports.NoOpObserveStack()
	if tracer != nil {
		observability = ports.StackFromCore(tracer, pkgName)
	}

	return &RateLimiter{
		limit:    limit,
		burst:    burst,
		now:      now,
		mu:       sync.Mutex{},
		limiters: make(map[ids.UserID]*rate.Limiter),
		tracer:   observability,
	}
}

// RateLimiter returns ratelimiter.PortWrapped interface.
//
//nolint:ireturn // it's a port interface
func (r *RateLimiter) RateLimiter() ratelimiter.PortWrapped { return ratelimiter.Wrap(r, r.tracer) }

// Consume consumes message quota for the given user.
func (r *RateLimiter) Consume(ctx context.Context, user ids.UserID, count int) error {
	r.mu.Lock()

	lim, ok := r.limiters[user]
	if !ok {
		lim = rate.NewLimiter(r.limit, r.burst)
		r.limiters[user] = lim
	}

	r.mu.Unlock()

	if !lim.AllowN(r.now(), count) {
		return fmt.Errorf("%w for user %q", ratelimiter.ErrRateLimitExceeded, user.ID().String())
	}

	return nil
}
