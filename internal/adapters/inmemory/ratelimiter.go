package inmemory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/rate"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// userEntry stores the rate limiters for a single user.
type userEntry struct {
	// inputChatTokens is an INPUT token bucket for chat model.
	inputChatTokens *rate.Limiter
	// outputChatTokens is an OUTPUT token bucket for chat model.
	outputChatTokens *rate.Limiter
	// embeddingTokens is a separate token bucket for embeddings. Unlike chat
	// tokens, embeddings can be easily calculated before request, since,
	// tokenizer is publicly available and can be evaluated on server side.
	embeddingTokens *rate.Limiter
	// lastSeen is the last time the user was seen.
	lastSeen atomic.Int64
}

func newUserEntry(
	inputTokensEvery time.Duration, inputTokensBurst int,
	outputTokensEvery time.Duration, outputTokensBurst int,
	embeddingTokensEvery time.Duration, embeddingTokensBurst int,
	maxWait time.Duration,
) *userEntry {
	return &userEntry{
		inputChatTokens:  rate.NewLimiterWithMaxWait(inputTokensEvery, inputTokensBurst, maxWait),
		outputChatTokens: rate.NewLimiterWithMaxWait(outputTokensEvery, outputTokensBurst, maxWait),
		embeddingTokens:  rate.NewLimiterWithMaxWait(embeddingTokensEvery, embeddingTokensBurst, maxWait),
		lastSeen:         atomic.Int64{},
	}
}

// RateLimiter is an in-memory implementation of the ratelimiter.Port.
// It uses a simple token bucket algorithm that supports negative rate quotas.
type RateLimiter struct {
	tracer ports.ObserveStack
	now    func() time.Time

	entries map[ids.UserID]*userEntry

	inputEvery     time.Duration
	inputBurst     int
	outputEvery    time.Duration
	outputBurst    int
	embeddingEvery time.Duration
	embeddingBurst int

	maxWaitTime time.Duration

	entriesMux sync.RWMutex
}

var (
	_ ratelimiter.PortFactory = (*RateLimiter)(nil)
	_ ratelimiter.Port        = (*RateLimiter)(nil)
)

type clock = func() time.Time

// NewRateLimiter creates a new in-memory rate limiter.
func NewRateLimiter(
	inputPeriod time.Duration, inputLimit int,
	outputPeriod time.Duration, outputLimit int,
	embeddingPeriod time.Duration, embeddingLimit int,
	maxWaitTime time.Duration,
	now clock,
	metrics core.Metrics,
) *RateLimiter {
	if now == nil {
		now = time.Now
	}

	observability := ports.StackFromCore(metrics, pkgName)

	return &RateLimiter{
		tracer:         observability,
		now:            now,
		entries:        make(map[ids.UserID]*userEntry),
		inputEvery:     inputPeriod,
		inputBurst:     inputLimit,
		outputEvery:    outputPeriod,
		outputBurst:    outputLimit,
		embeddingEvery: embeddingPeriod,
		embeddingBurst: embeddingLimit,
		maxWaitTime:    maxWaitTime,
		entriesMux:     sync.RWMutex{},
	}
}

// RateLimiter returns ratelimiter.PortWrapped interface.
func (r *RateLimiter) RateLimiter() ratelimiter.PortWrapped { return ratelimiter.Wrap(r, r.tracer) }

// ConsumeChatRequests consumes message quota and returns a settlement callback.
//
// TODO: Currently implementation doesn't use model name, however, we must
// recalculate tokens based on model.
func (r *RateLimiter) ConsumeChatRequests(
	_ context.Context, user ids.UserID, _ string, inputTokens int,
) (ratelimiter.ConsumedTokensFunc, error) {
	now := r.now()
	entry := r.getEntry(user, now)

	// 1. Check Chat Output Limiter (Soft)
	// We only allow starting if the balance is currently positive.
	outputRes := entry.outputChatTokens.SoftReserveN(now, 0)
	if !outputRes.OK() || outputRes.RetryAt().After(now) {
		outputRes.CancelAt(now)

		return nil, ratelimiter.ErrRateLimitExceeded(outputRes.RetryAt())
	}

	// 2. Check Chat Input Limiter (Hard)
	inputRes := entry.inputChatTokens.ReserveN(now, inputTokens)
	if !inputRes.OK() || inputRes.RetryAt().After(now) {
		inputRes.CancelAt(now)

		return nil, ratelimiter.ErrRateLimitExceeded(inputRes.RetryAt())
	}

	return func(_ context.Context, outputTokens int) error {
		// Use ForceReserveN for settlement to ensure usage is recorded regardless of balance.
		// The penalty ceiling is handled automatically by the Limiter's minTokens.
		entry.outputChatTokens.ForceReserveN(r.now(), outputTokens)
		return nil
	}, nil
}

// ConsumeEmbeddingRequests consumes embedding requests quota for the given
// user.
//
// TODO: Currently implementation doesn't use model name, however, we must
// recalculate tokens based on model.
func (r *RateLimiter) ConsumeEmbeddingRequests(
	_ context.Context, user ids.UserID, _ string, inputTokens int,
) error {
	now := r.now()
	entry := r.getEntry(user, now)

	// Embedding tokens are always calculated upfront, so we use it as a hard limit.
	res := entry.embeddingTokens.ReserveN(now, inputTokens)
	if !res.OK() || res.RetryAt().After(now) {
		res.CancelAt(now)

		return ratelimiter.ErrRateLimitExceeded(res.RetryAt())
	}

	return nil
}

func (r *RateLimiter) getEntry(user ids.UserID, now time.Time) *userEntry {
	r.entriesMux.RLock()
	entry, ok := r.entries[user]
	r.entriesMux.RUnlock()

	if ok {
		entry.lastSeen.Store(now.UnixNano())
		return entry
	}

	r.entriesMux.Lock()
	defer r.entriesMux.Unlock()

	// Double-check after acquiring write lock
	entry, ok = r.entries[user]
	if !ok {
		entry = newUserEntry(
			r.inputEvery, r.inputBurst,
			r.outputEvery, r.outputBurst,
			r.embeddingEvery, r.embeddingBurst,
			r.maxWaitTime,
		)
		entry.lastSeen.Store(now.UnixNano())
		r.entries[user] = entry
	}

	return entry
}

const (
	tickerPeriodFactor = 2
)

// Cleanup periodically removes stale rate limiters from memory.
//
// Cleanup uses maxWaitTime to determine when a rate limiter is stale, since all
// limits are constrained to maxWaitTime.
func (r *RateLimiter) Cleanup(ctx context.Context) error {
	if r.maxWaitTime <= 0 {
		return nil
	}

	ticker := time.NewTicker(r.maxWaitTime / tickerPeriodFactor)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.Canceled) {
				// returning nil, cause cleanup is optional and it doesn't affect
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
	r.entriesMux.Lock()
	defer r.entriesMux.Unlock()

	for user, entry := range r.entries {
		lastSeen := entry.lastSeen.Load()
		if lastSeen != 0 && now-lastSeen > int64(r.maxWaitTime) {
			delete(r.entries, user)
		}
	}
}
