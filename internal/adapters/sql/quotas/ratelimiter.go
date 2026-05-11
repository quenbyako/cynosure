// Package quotas provides a PostgreSQL implementation of the ratelimiter.Port.
package quotas

import (
	"context"
	"errors"
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
	ctx context.Context, user ids.UserID, _ string, inputTokens int,
) (ratelimiter.ConsumedTokensFunc, error) {
	quota, err := q.q.GetUserQuota(ctx, user.ID())
	if err != nil {
		return nil, fmt.Errorf("getting user quota: %w", err)
	}

	maxWait := intervalToDuration(quota.MaxAwaitPeriod)
	if err := q.atomicConsume(ctx, user, quota, inputTokens, maxWait); err != nil {
		return nil, err
	}

	return func(ctx context.Context, outputTokens int) error {
		return q.consume(ctx, user, "output", quota, outputTokens, -1, false)
	}, nil
}

func (q Quotas) atomicConsume(
	ctx context.Context, user ids.UserID, quota db.GetUserQuotaRow,
	input int, maxWait time.Duration,
) error {
	transaction, err := q.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() { _ = transaction.Rollback(ctx) }() //nolint:errcheck

	queries := q.q.WithTx(transaction)
	types := []string{"output", "input"}
	amounts := []int{0, input}

	for i, typ := range types {
		err = q.lockedConsume(
			ctx, queries, user, typ, quota, amounts[i], maxWait, false,
		)
		if err != nil {
			if errors.As(err, new(*ratelimiter.RateLimitExceededError)) {
				_ = transaction.Commit(ctx) //nolint:errcheck
			}

			return err
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// ConsumeEmbeddingRequests consumes embedding quota for the given user.
func (q Quotas) ConsumeEmbeddingRequests(
	ctx context.Context, user ids.UserID, _ string, tokens int,
) error {
	quota, err := q.q.GetUserQuota(ctx, user.ID())
	if err != nil {
		return fmt.Errorf("getting user quota: %w", err)
	}

	return q.consume(ctx, user, "embedding", quota, tokens, 0, false)
}

func (q Quotas) consume(
	ctx context.Context, user ids.UserID, typ string,
	quota db.GetUserQuotaRow, amount int, maxWait time.Duration, force bool,
) error {
	transaction, err := q.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() { _ = transaction.Rollback(ctx) }() //nolint:errcheck

	err = q.lockedConsume(
		ctx, q.q.WithTx(transaction), user, typ, quota, amount, maxWait, force,
	)
	if err != nil {
		if errors.As(err, new(*ratelimiter.RateLimitExceededError)) {
			_ = transaction.Commit(ctx) //nolint:errcheck
		}

		return err
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (q Quotas) lockedConsume(
	ctx context.Context, queries *db.Queries, user ids.UserID,
	typ string, quota db.GetUserQuotaRow, amount int, maxWait time.Duration, force bool,
) error {
	now := q.now()
	period, limit := selectQuota(typ, quota)

	bucket, err := q.getAndLockBucket(ctx, queries, user, typ, now)
	if err != nil {
		return err
	}

	calc := calculateLeakyBucket(bucket, now, period, limit, amount, maxWait, force)

	if err := queries.UpdateBucket(ctx, db.UpdateBucketParams{
		Level: calc.newLevel, LastLeakAt: now, UserID: user.ID(), ResourceType: typ,
	}); err != nil {
		return fmt.Errorf("updating bucket: %w", err)
	}

	return calc.err
}

func (q Quotas) getAndLockBucket(
	ctx context.Context, queries *db.Queries, user ids.UserID, typ string, now time.Time,
) (db.GetBucketForUpdateRow, error) {
	bucket, err := queries.GetBucketForUpdate(ctx, db.GetBucketForUpdateParams{
		UserID: user.ID(), ResourceType: typ,
	})

	if errors.Is(err, pgx.ErrNoRows) {
		return q.createAndLockBucket(ctx, queries, user, typ, now)
	}

	if err != nil {
		return db.GetBucketForUpdateRow{}, fmt.Errorf("getting bucket: %w", err)
	}

	return bucket, nil
}

func (q Quotas) createAndLockBucket(
	ctx context.Context, queries *db.Queries, user ids.UserID, typ string, now time.Time,
) (db.GetBucketForUpdateRow, error) {
	if err := queries.CreateBucketIfNotExists(ctx, db.CreateBucketIfNotExistsParams{
		UserID: user.ID(), ResourceType: typ, LastLeakAt: now,
	}); err != nil {
		return db.GetBucketForUpdateRow{}, fmt.Errorf("creating bucket: %w", err)
	}

	res, err := queries.GetBucketForUpdate(ctx, db.GetBucketForUpdateParams{
		UserID: user.ID(), ResourceType: typ,
	})
	if err != nil {
		return db.GetBucketForUpdateRow{}, fmt.Errorf("getting bucket after creation: %w", err)
	}

	return res, nil
}

func selectQuota(typ string, quota db.GetUserQuotaRow) (period pgtype.Interval, limit int) {
	switch typ {
	case "input":
		return quota.ChatInputPeriod, int(quota.ChatInputLimit)
	case "output":
		return quota.ChatOutputPeriod, int(quota.ChatOutputLimit)
	case "embedding":
		return quota.EmbeddingPeriod, int(quota.EmbeddingLimit)
	default:
		return pgtype.Interval{}, 0
	}
}

type bucketCalculation struct {
	err      error
	newLevel float64
}

func calculateLeakyBucket(
	bucket db.GetBucketForUpdateRow, now time.Time, period pgtype.Interval,
	limit, amount int, maxWait time.Duration, force bool,
) bucketCalculation {
	periodUs := float64(intervalToDuration(period).Microseconds())

	if periodUs <= 0 {
		periodUs = 1000000
	}

	leakRate := float64(limit) / periodUs
	leakDurationUs := math.Max(0, float64(now.Sub(bucket.LastLeakAt.Time).Microseconds()))
	leaked := leakDurationUs * leakRate
	level := math.Max(0, bucket.Level-leaked)

	newLevel, err := checkRateLimit(level, amount, limit, leakRate, maxWait, now, force)

	if maxWait >= 0 {
		limitLevel := float64(limit) + float64(maxWait.Microseconds())*leakRate
		newLevel = math.Min(newLevel, limitLevel)
	}

	return bucketCalculation{newLevel: newLevel, err: err}
}

func checkRateLimit(
	level float64, amount, limit int, leakRate float64,
	maxWait time.Duration, now time.Time, force bool,
) (float64, error) {
	retryUs := (level + float64(amount) - float64(limit)) / leakRate

	if !force && maxWait >= 0 && retryUs > 0 &&
		(maxWait == 0 || retryUs >= float64(maxWait.Microseconds())) {
		effectiveRetryUs := retryUs
		if maxWait > 0 {
			effectiveRetryUs = math.Min(retryUs, float64(maxWait.Microseconds()))
		}

		return level, ratelimiter.ErrRateLimitExceeded(
			now.Add(time.Duration(math.Ceil(effectiveRetryUs)) * time.Microsecond),
		)
	}

	return level + float64(amount), nil
}

func intervalToDuration(interval pgtype.Interval) time.Duration {
	const (
		hoursPerDay   = 24
		daysPerMonth  = 30
		hoursPerMonth = daysPerMonth * hoursPerDay
	)

	d := time.Duration(interval.Microseconds) * time.Microsecond
	d += time.Duration(interval.Days) * hoursPerDay * time.Hour
	d += time.Duration(interval.Months) * hoursPerMonth * time.Hour

	return d
}
