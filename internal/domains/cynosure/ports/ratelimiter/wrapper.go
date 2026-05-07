package ratelimiter

import (
	"context"
	"errors"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// PortWrapped provides the interface for the wrapped rate limiter.
type PortWrapped interface {
	Port

	_PortWrapped()
}

type portWrapped struct {
	w Port
	t *observable
}

var _ PortWrapped = (*portWrapped)(nil)

func (t *portWrapped) _PortWrapped() {}

// Wrap wraps the given port with observability tools.
func Wrap(client Port, observable ports.ObserveStack) (PortWrapped, error) {
	if observable == nil {
		observable = ports.NoOpObserveStack()
	}

	obs, err := newObservable(observable)
	if err != nil {
		return nil, err
	}

	t := portWrapped{
		w: client,
		t: obs,
	}

	return &t, nil
}

// ConsumeChat consumes rate limit of messages for the given user.
func (t *portWrapped) ConsumeChatRequests(
	ctx context.Context, user ids.UserID, model string, inputTokens int,
) (callback ConsumedTokensFunc, err error) {
	ctx, span := t.t.consumeChatRequests(ctx, user, inputTokens)
	defer span.end()

	callback, err = t.w.ConsumeChatRequests(ctx, user, model, inputTokens)
	if e := new(RateLimitExceededError); errors.As(err, &e) {
		t.t.recordRateLimit(ctx, model, e.RetryAt())
	}

	span.recordError(err)

	//nolint:wrapcheck // should not wrap adapter errors
	return callback, err
}

func (t *portWrapped) ConsumeEmbeddingRequests(
	ctx context.Context, user ids.UserID, model string, inputTokens int,
) (err error) {
	ctx, span := t.t.consumeEmbeddingRequests(ctx, user, inputTokens)
	defer span.end()

	err = t.w.ConsumeEmbeddingRequests(ctx, user, model, inputTokens)
	if e := new(RateLimitExceededError); errors.As(err, &e) {
		t.t.recordRateLimit(ctx, model, e.RetryAt())
	}

	span.recordError(err)

	//nolint:wrapcheck // should not wrap adapter errors
	return err
}
