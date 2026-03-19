// Package redis provides redis-backed adapter implementations.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/quenbyako/core"
	"github.com/redis/go-redis/v9"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	pkgNamge = "github.com/quenbyako/cynosure/internal/adapters/redis"
)

// RateLimiter is a Redis-backed implementation of ratelimiter.Port.
type RateLimiter struct {
	tracer  ports.ObserveStack
	limiter *redis_rate.Limiter
	now     func() time.Time
	limit   redis_rate.Limit
}

var _ ratelimiter.PortFactory = (*RateLimiter)(nil)

// NewRateLimiter creates a new Redis rate limiter.
func NewRateLimiter(
	rdb redis.UniversalClient,
	limit, burst int,
	now func() time.Time,
	tracer core.Metrics,
) *RateLimiter {
	if now == nil {
		now = time.Now
	}

	observability := ports.NoOpObserveStack()
	if tracer != nil {
		observability = ports.StackFromCore(tracer, pkgNamge)
	}

	return &RateLimiter{
		limiter: redis_rate.NewLimiter(rdb),
		limit: redis_rate.Limit{
			Rate:   limit,
			Burst:  burst,
			Period: time.Second,
		},
		now:    now,
		tracer: observability,
	}
}

// RateLimiter returns ratelimiter.PortWrapped interface.
//
//nolint:ireturn // it's a port interface
func (r *RateLimiter) RateLimiter() ratelimiter.PortWrapped { return ratelimiter.Wrap(r, r.tracer) }

// Consume consumes message quota for the given user.
func (r *RateLimiter) Consume(ctx context.Context, user ids.UserID, count int) error {
	// For testing purposes: if the mocked clock advances into the future relative to
	// real time, we actually sleep to let the Redis backend naturally catch up.
	if r.now != nil {
		delay := time.Until(r.now())
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	key := "rate:user:" + user.ID().String()

	res, err := r.limiter.AllowN(ctx, key, r.limit, count)
	if err != nil {
		return fmt.Errorf("redis allow n: %w", err)
	}

	if res.Allowed == 0 {
		return fmt.Errorf("%w for user %q", ratelimiter.ErrRateLimitExceeded, user.ID().String())
	}

	return nil
}
