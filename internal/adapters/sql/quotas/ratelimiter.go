package quotas

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type conn interface {
	db.DBTX
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// Quotas is a PostgreSQL-backed implementation of ratelimiter.Port.
// It uses a leaky bucket algorithm with debt support, mirroring the Redis logic.
type Quotas struct {
	tx conn
	q  *db.Queries

	now func() time.Time
}

var _ ratelimiter.Port = (*Quotas)(nil)

// New creates a new PostgreSQL rate limiter.
func New(conn conn) Quotas {
	return Quotas{
		tx:  conn,
		q:   db.New(conn),
		now: time.Now,
	}
}

// WithClock sets a custom clock for the rate limiter.
func (q Quotas) WithClock(now func() time.Time) Quotas {
	q.now = now
	return q
}

// ConsumeChatRequests consumes message quota for the given user and model.
func (q Quotas) ConsumeChatRequests(
	ctx context.Context, user ids.UserID, model string, inputTokens int,
) (ratelimiter.ConsumedTokensFunc, error) {
	// 1. Get Quota
	quota, err := q.q.GetUserQuota(ctx, user.ID())
	if err != nil {
		return nil, fmt.Errorf("getting user quota: %w", err)
	}

	maxWait := time.Duration(quota.MaxAwaitPeriod.Microseconds) * time.Microsecond
	maxWait += time.Duration(quota.MaxAwaitPeriod.Days) * 24 * time.Hour

	// 1. Check Chat Output Limiter (Soft)
	if err := q.consumeWithMaxWait(ctx, user, "output", 0, maxWait, false); err != nil {
		return nil, err
	}

	// 2. Check Chat Input Limiter (Hard)
	if err := q.consumeWithMaxWait(ctx, user, "input", inputTokens, maxWait, false); err != nil {
		return nil, err
	}

	return func(ctx context.Context, outputTokens int) error {
		// Chat Output Limiter (Soft)
		return q.consumeWithMaxWait(ctx, user, "output", outputTokens, -1, false)
	}, nil
}

// ConsumeEmbeddingRequests consumes embedding quota for the given user.
func (q Quotas) ConsumeEmbeddingRequests(
	ctx context.Context, user ids.UserID, _ string, tokens int,
) error {
	return q.consumeWithMaxWait(ctx, user, "embedding", tokens, 0, false)
}

func (q Quotas) consumeWithMaxWait(
	ctx context.Context, user ids.UserID, typ string,
	amount int, maxWait time.Duration, force bool,
) error {
	now := q.now()

	// Using manual transaction to match the 'conn' interface pattern
	tx, err := q.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qq := q.q.WithTx(tx)

	quota, err := qq.GetUserQuota(ctx, user.ID())
	if err != nil {
		return fmt.Errorf("getting quota: %w", err)
	}

	var period pgtype.Interval
	var limit int
	switch typ {
	case "input":
		period = quota.ChatInputPeriod
		limit = int(quota.ChatInputLimit)
	case "output":
		period = quota.ChatOutputPeriod
		limit = int(quota.ChatOutputLimit)
	case "embedding":
		period = quota.EmbeddingPeriod
		limit = int(quota.EmbeddingLimit)
	}

	// 2. Get and lock the bucket
	bucket, err := qq.GetOrCreateBucket(ctx, db.GetOrCreateBucketParams{
		UserID:       user.ID(),
		ResourceType: typ,
		LastLeakAt:   now,
	})
	if err != nil {
		return fmt.Errorf("upserting bucket: %w", err)
	}

	// 3. Leaky Bucket Logic
	calc := calculateLeakyBucket(bucket.Level, bucket.LastLeakAt.Time, now, period, limit, amount, maxWait, force)

	var errResult error
	if calc.err != nil {
		errResult = calc.err
	}
	newLevel := calc.newLevel


	// 4. Update bucket
	err = qq.UpdateBucket(ctx, db.UpdateBucketParams{
		Level:        newLevel,
		LastLeakAt:   now,
		UserID:       user.ID(),
		ResourceType: typ,
	})
	if err != nil {
		return fmt.Errorf("updating bucket: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return errResult
}

type bucketCalculation struct {
	newLevel float64
	err      error
}

func calculateLeakyBucket(
	currentLevel float64, lastLeakAt, now time.Time,
	period pgtype.Interval, limit, amount int,
	maxWait time.Duration, force bool,
) bucketCalculation {
	elapsedUs := float64(now.Sub(lastLeakAt).Microseconds())

	periodUs := float64(period.Microseconds)
	periodUs += float64(period.Days) * 24 * 60 * 60 * 1000000
	periodUs += float64(period.Months) * 30 * 24 * 60 * 60 * 1000000

	if periodUs <= 0 {
		periodUs = 1000000 // avoid division by zero, default 1s
	}
	leakRate := float64(limit) / periodUs // tokens per microsecond

	leaked := elapsedUs * leakRate
	actualLevel := math.Max(0, currentLevel-leaked)
	newLevel := actualLevel

	retryAfterUs := 0.0
	if actualLevel+float64(amount) > float64(limit) {
		retryAfterUs = (actualLevel + float64(amount) - float64(limit)) / leakRate
	}

	var errResult error
	if !force && maxWait >= 0 {
		if (maxWait == 0 && retryAfterUs > 0) ||
			(maxWait > 0 && retryAfterUs >= float64(maxWait.Microseconds())) {

			effectiveRetryUs := retryAfterUs
			if maxWait > 0 && effectiveRetryUs > float64(maxWait.Microseconds()) {
				effectiveRetryUs = float64(maxWait.Microseconds())
			}
			errResult = ratelimiter.ErrRateLimitExceeded(now.Add(time.Duration(math.Ceil(effectiveRetryUs)) * time.Microsecond))
		}
	}

	if errResult == nil {
		newLevel += float64(amount)
	}

	// Cap the level to (limit + maxWait) during update
	if maxWait >= 0 {
		limitLevel := float64(limit)
		if maxWait > 0 {
			limitLevel += float64(maxWait.Microseconds()) * leakRate
		}
		if newLevel > limitLevel {
			newLevel = limitLevel
		}
	}

	return bucketCalculation{newLevel: newLevel, err: errResult}
}

