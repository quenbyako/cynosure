// Package quotas provides a PostgreSQL implementation of the ratelimiter.Port.
package quotas

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	sqlerrors "github.com/quenbyako/cynosure/internal/adapters/sql/errors"
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
	cfg Config
}

// Config defines the default quota configuration for users without an assigned plan.
type Config struct {
	DefaultPlanID uuid.UUID
}

var _ ratelimiter.Port = (*Quotas)(nil)

// New creates a new PostgreSQL rate limiter.
func New(conn conn, cfg Config) Quotas {
	return Quotas{
		tx:  conn,
		q:   db.New(conn),
		now: time.Now,
		cfg: cfg,
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
	quota, err := q.getQuota(ctx, user)
	if err != nil {
		return nil, err
	}

	maxWait := intervalToDuration(quota.MaxAwaitPeriod)
	// For chat, we use strict=true for pre-flight checks (input and output-preflight).
	// This ensures we don't start a chat if there's ANY debt or insufficient tokens.
	if err := q.atomicConsume(ctx, user, quota, inputTokens, maxWait, true); err != nil {
		return nil, err
	}

	return func(ctx context.Context, outputTokens int) error {
		// Settlement is forced and not strict (it uses the plan's maxWait for capping).
		return q.consume(ctx, user, "output", quota, outputTokens, maxWait, true, false)
	}, nil
}

func (q Quotas) atomicConsume(
	ctx context.Context, user ids.UserID, quota db.GetUserQuotaRow,
	input int, maxWait time.Duration, strict bool,
) error {
	transaction, err := q.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() { _ = transaction.Rollback(ctx) }() //nolint:errcheck

	queries := q.q.WithTx(transaction)
	// Sort types to avoid deadlocks. Alphabetical order is used.
	types := []string{"input", "output"}
	amounts := []int{input, 0}

	for i, typ := range types {
		// We use the provided strictness for the atomic operation.
		// For Chat, it's usually true to enforce hard pre-flight limits.
		if err := q.lockedConsume(
			ctx, queries, user, typ, quota, amounts[i], maxWait, false, strict,
		); err != nil {
			return handleConsumeError(ctx, transaction, err)
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func handleConsumeError(ctx context.Context, transaction pgx.Tx, err error) error {
	if errors.As(err, new(*ratelimiter.RateLimitExceededError)) {
		_ = transaction.Commit(ctx) //nolint:errcheck
	}

	return err
}

// ConsumeEmbeddingRequests consumes embedding quota for the given user.
func (q Quotas) ConsumeEmbeddingRequests(
	ctx context.Context, user ids.UserID, _ string, tokens int,
) error {
	quota, err := q.getQuota(ctx, user)
	if err != nil {
		return err
	}

	// Embedding is always upfront and strict.
	return q.consume(ctx, user, "embedding", quota, tokens, 0, false, true)
}

func (q Quotas) consume(
	ctx context.Context, user ids.UserID, typ string, quota db.GetUserQuotaRow,
	amount int, maxWait time.Duration, force, strict bool,
) error {
	transaction, err := q.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() { _ = transaction.Rollback(ctx) }() //nolint:errcheck

	err = q.lockedConsume(
		ctx, q.q.WithTx(transaction), user, typ, quota, amount, maxWait, force, strict,
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
	ctx context.Context, queries *db.Queries, user ids.UserID, typ string,
	quota db.GetUserQuotaRow, amount int, maxWait time.Duration, force, strict bool,
) error {
	now := q.now()
	period, limit := selectQuota(typ, quota)

	bucket, err := q.getAndLockBucket(ctx, queries, user, typ, now)
	if err != nil {
		return err
	}

	calc := calculateLeakyBucket(bucket, now, period, limit, amount, maxWait, force, strict)

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

func (q Quotas) getQuota(ctx context.Context, uid ids.UserID) (db.GetUserQuotaRow, error) {
	quota, err := q.q.GetUserQuota(ctx, uid.ID())
	if err == nil {
		return quota, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return db.GetUserQuotaRow{}, fmt.Errorf("getting user quota: %w", err)
	}

	return q.getDefaultQuota(ctx)
}

func (q Quotas) getDefaultQuota(ctx context.Context) (db.GetUserQuotaRow, error) {
	if q.cfg.DefaultPlanID == uuid.Nil {
		return db.GetUserQuotaRow{}, sqlerrors.ErrNoDefaultPlan
	}

	planQuota, err := q.q.GetPlanQuotaByID(ctx, q.cfg.DefaultPlanID)
	if err != nil {
		return db.GetUserQuotaRow{}, fmt.Errorf(
			"getting default plan %s: %w", q.cfg.DefaultPlanID, err,
		)
	}

	return db.GetUserQuotaRow(planQuota), nil
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
	limit, amount int, maxWait time.Duration, force, strict bool,
) bucketCalculation {
	leakRate, level := calculateLevel(bucket, now, period, limit)

	checkWait := maxWait
	if strict {
		checkWait = 0
	}

	newLevel, err := checkRateLimit(level, amount, limit, leakRate, checkWait, now, force)
	if err != nil && !force {
		return bucketCalculation{newLevel: level, err: err}
	}

	// 5. Apply penalty ceiling (capping the debt).
	// In parity with inmemory/rate.go, we only cap the debt if maxWait > 0.
	// If maxWait is 0 or negative, we allow infinite debt (but we will block
	// any non-force consumption if we are in debt).
	if maxWait > 0 {
		limitLevel := float64(limit) + float64(maxWait.Microseconds())*leakRate
		newLevel = math.Min(newLevel, limitLevel)
	}

	return bucketCalculation{newLevel: newLevel, err: err}
}

func calculateLevel(
	bucket db.GetBucketForUpdateRow, now time.Time, period pgtype.Interval, limit int,
) (leakRate, level float64) {
	periodUs := float64(intervalToDuration(period).Microseconds())
	if periodUs <= 0 {
		periodUs = 1000000
	}

	if limit > 0 {
		leakRate = float64(limit) / periodUs
	}

	leakDurationUs := math.Max(0, float64(now.Sub(bucket.LastLeakAt.Time).Microseconds()))
	leaked := leakDurationUs * leakRate
	level = math.Max(0, bucket.Level-leaked)

	return leakRate, level
}

func checkRateLimit(
	level float64, amount, limit int, leakRate float64,
	maxWait time.Duration, now time.Time, force bool,
) (float64, error) {
	if leakRate <= 0 {
		if amount > limit && !force {
			return level, ratelimiter.ErrRateLimitExceeded(now.Add(maxWait))
		}

		return level + float64(amount), nil
	}

	retryUs := (level + float64(amount) - float64(limit)) / leakRate
	if !force && maxWait >= 0 && retryUs > 0 &&
		(maxWait == 0 || retryUs >= float64(maxWait.Microseconds())) {
		wait := retryUs
		if maxWait > 0 {
			wait = math.Min(retryUs, float64(maxWait.Microseconds()))
		}

		return level, ratelimiter.ErrRateLimitExceeded(
			now.Add(time.Duration(math.Ceil(wait)) * time.Microsecond),
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
