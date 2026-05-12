package ratelimiter

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	cynosureUserID            attribute.Key = "cynosure.user_id"
	cynosureRatelimiterAmount attribute.Key = "cynosure.ratelimiter.amount"

	limitExceededKey     = "cynosure.ports.ratelimiter.limit_exceeded"
	retryAfterSecondsKey = "cynosure.ports.ratelimiter.retry_after_seconds"
)

type observable struct {
	t trace.Tracer

	limitExceeded     metric.Int64Counter
	retryAfterSeconds metric.Float64Histogram
}

func newObservable(stack ports.ObserveStack) (*observable, error) {
	if stack == nil {
		stack = ports.NoOpObserveStack()
	}

	meter := stack.Meter()

	limitExceeded, err := meter.Int64Counter(limitExceededKey,
		metric.WithDescription("Number of times rate limit was exceeded"),
	)
	if err != nil {
		return nil, fmt.Errorf("limit exceeded counter: %w", err)
	}

	retryAfterSeconds, err := meter.Float64Histogram(retryAfterSecondsKey,
		metric.WithDescription("Duration of retry-after in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("retry after histogram: %w", err)
	}

	return &observable{
		t:                 stack.Tracer(),
		limitExceeded:     limitExceeded,
		retryAfterSeconds: retryAfterSeconds,
	}, nil
}

// metric callbacks

func (o *observable) recordRateLimit(ctx context.Context, model string, retryAt time.Time) {
	retryAfter := time.Until(retryAt)

	o.limitExceeded.Add(ctx, 1,
		metric.WithAttributes(
			semconv.GenAIRequestModel(model),
		),
	)

	o.retryAfterSeconds.Record(ctx, retryAfter.Seconds(),
		metric.WithAttributes(
			semconv.GenAIRequestModel(model),
		),
	)
}

// trace callbacks

//nolint:spancheck // isolated in a wrapper
func (o *observable) consumeChatRequests(
	ctx context.Context, user ids.UserID, amount int,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.ratelimiter.consume_chat_requests",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			cynosureUserID.String(user.ID().String()),
			cynosureRatelimiterAmount.Int(amount),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck // isolated in a wrapper
func (o *observable) consumeEmbeddingRequests(
	ctx context.Context, user ids.UserID, amount int,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.ratelimiter.consume_embedding_requests",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			cynosureUserID.String(user.ID().String()),
			cynosureRatelimiterAmount.Int(amount),
		),
	)

	return ctx, &spanCallback{span: span}
}

// generic span

type span interface {
	end()
	recordError(err error)
}

type spanCallback struct {
	span trace.Span
}

func (c *spanCallback) end() {
	if c.span != nil {
		c.span.End()
	}
}

func (c *spanCallback) recordError(err error) {
	if err != nil && c.span != nil {
		c.span.RecordError(err)
	}
}
