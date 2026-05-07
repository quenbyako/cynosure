package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/quenbyako/core"
	"github.com/redis/go-redis/v9"

	_ "embed"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

//go:embed lua/adaptive_limit.lua
var adaptiveLimitScript string

// RateLimiter is a Redis-backed implementation of ratelimiter.Port.
type RateLimiter struct {
	observability ports.ObserveStack
	rdb           redis.UniversalClient

	inputLimit     Limit
	outputLimit    Limit
	embeddingLimit Limit

	maxWaitTime time.Duration
}

type Limit struct {
	Rate   int
	Burst  int
	Period time.Duration
}

var (
	_ ratelimiter.PortFactory = (*RateLimiter)(nil)
	_ ratelimiter.Port        = (*RateLimiter)(nil)
)

// New creates a new Redis rate limiter.
//
// This adapter DOES NOT and will not ever support providing custom clock
// function, since Redis server has its own clock, which is used in Lua scripts.
// This means, that all time-based actions (expiration, rate limit waiting,
// etc.) based on real time.
func New(
	rdb redis.UniversalClient,
	inputPeriod time.Duration, inputBurst int,
	outputPeriod time.Duration, outputBurst int,
	embeddingPeriod time.Duration, embeddingBurst int,
	maxWaitTime time.Duration,
	tracer core.Metrics,
) *RateLimiter {
	observability := ports.NoOpObserveStack()
	if tracer != nil {
		observability = ports.StackFromCore(tracer, pkgName)
	}

	return &RateLimiter{
		rdb:            rdb,
		inputLimit:     Limit{Rate: inputBurst, Burst: inputBurst, Period: inputPeriod},
		outputLimit:    Limit{Rate: outputBurst, Burst: outputBurst, Period: outputPeriod},
		embeddingLimit: Limit{Rate: embeddingBurst, Burst: embeddingBurst, Period: embeddingPeriod},
		maxWaitTime:    maxWaitTime,
		observability:  observability,
	}
}

// RateLimiter returns ratelimiter.PortWrapped interface.
func (r *RateLimiter) RateLimiter() (ratelimiter.PortWrapped, error) {
	return ratelimiter.Wrap(r, r.observability)
}

// ConsumeChatRequests consumes message quota for the given user and model.
func (r *RateLimiter) ConsumeChatRequests(
	ctx context.Context, user ids.UserID, model string, inputTokens int,
) (ratelimiter.ConsumedTokensFunc, error) {
	// 1. Check Chat Output Limiter (Soft)
	// We check output first so that if the user is in debt, we don't leak
	// input tokens. We use maxWait=0 here because we want to block STARTING
	// the request if they are already in debt.
	err := r.consumeWithMaxWait(ctx, user, model, 0, "output", 0, false)
	if err != nil {
		return nil, err
	}

	// 2. Check Chat Input Limiter (Hard)
	// Input does not support debt, so we strictly enforce maxWait=0
	if err := r.consumeWithMaxWait(ctx, user, model, inputTokens, "input", 0, false); err != nil {
		return nil, err
	}

	return func(ctx context.Context, outputTokens int) error {
		// Reservation: allow going into debt. We pass force=true (last arg) to
		// ensure
		// it never blocks during reservation and caps debt to maxWaitTime.
		return r.consumeWithMaxWait(ctx, user, model, outputTokens, "output", r.maxWaitTime, true)
	}, nil
}

// ConsumeEmbeddingRequests consumes embedding requests quota for the given user.
func (r *RateLimiter) ConsumeEmbeddingRequests(
	ctx context.Context, user ids.UserID, model string, requests int,
) error {
	return r.consumeWithMaxWait(ctx, user, model, requests, "embedding", 0, false)
}

//nolint:funlen // logic requires these steps
func (r *RateLimiter) consumeWithMaxWait(
	ctx context.Context, user ids.UserID, _ string,
	amount int, typ string, maxWait time.Duration, force bool,
) error {
	limit, err := r.getLimitByType(typ)
	if err != nil {
		return err
	}

	if limit.Period == 0 || (amount <= 0 && maxWait >= 0) {
		if limit.Period == 0 {
			return nil
		}
	}

	now := time.Now()
	key := fmt.Sprintf("rate:%s:{user:%s}", typ, user.ID().String())
	timeKey := key + ":time"

	res, err := r.evalRateLimit(ctx, key, timeKey, amount, limit, maxWait, now, force)
	if err != nil {
		return err
	}

	allowed, ok1 := res[0].(int64)
	retryAfterMs, ok2 := res[1].(int64)

	if !ok1 || !ok2 {
		return fmt.Errorf("%w: unexpected redis response types: %T, %T",
			redis.ErrInvalidCommand, res[0], res[1])
	}

	if allowed == 0 {
		return r.reportRateLimitExceeded(now, retryAfterMs, maxWait)
	}

	return nil
}

func (r *RateLimiter) reportRateLimitExceeded(
	now time.Time, retryAfterMs int64, maxWait time.Duration,
) error {
	retryAfter := time.Duration(retryAfterMs) * time.Millisecond

	if maxWait > 0 && retryAfter > maxWait {
		retryAfter = maxWait
	}

	return ratelimiter.ErrRateLimitExceeded(now.Add(retryAfter))
}

func (r *RateLimiter) evalRateLimit(
	ctx context.Context, key, timeKey string, amount int,
	limit Limit, maxWait time.Duration, now time.Time, force bool,
) ([]any, error) {
	forceInt := 0
	if force {
		forceInt = 1
	}

	res, err := r.rdb.Eval(ctx, adaptiveLimitScript, []string{key, timeKey},
		amount, limit.Rate, limit.Period.Milliseconds(), limit.Burst,
		r.getLuaMaxWait(maxWait), now.UnixMilli(), forceInt,
	).Slice()
	if err != nil {
		return nil, fmt.Errorf("redis eval: %w", err)
	}

	return res, nil
}

func (r *RateLimiter) getLimitByType(typ string) (Limit, error) {
	switch typ {
	case "input":
		return r.inputLimit, nil
	case "output":
		return r.outputLimit, nil
	case "embedding":
		return r.embeddingLimit, nil
	default:
		//nolint:err113 // internal adapter error
		return Limit{}, fmt.Errorf("unknown limit type: %s", typ)
	}
}

func (r *RateLimiter) getLuaMaxWait(maxWait time.Duration) int64 {
	if maxWait < 0 {
		return -1
	}

	return maxWait.Milliseconds()
}
