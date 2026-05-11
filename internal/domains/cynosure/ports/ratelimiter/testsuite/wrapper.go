package testsuite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// TB is a subset of testing.TB interface.
type TB interface {
	Logf(format string, args ...any)
	Helper()
}

type waitWrapper struct {
	recordedMock time.Time
	tb           TB
	inner        ratelimiter.Port
	// mock clock, that returns from suite
	mockNow func() time.Time
	params  SetupParams
	// diff between start mock and real time, recorded before any call
	diffSync time.Duration
}

var _ ratelimiter.Port = (*waitWrapper)(nil)

// NewWaitWrapper creates a new ratelimiter.Port that synchronizes mock time
// with real time. It uses time.Sleep to ensure that the underlying adapter sees
// the expected elapsed time.
func NewWaitWrapper(tb TB, inner ratelimiter.Port, params SetupParams) ratelimiter.Port {
	return &waitWrapper{
		tb:           tb,
		inner:        inner,
		params:       params,
		mockNow:      params.Now,
		recordedMock: params.Now(),
		diffSync:     time.Since(params.StartedAt),
	}
}

func (w *waitWrapper) ConsumeChatRequests(
	ctx context.Context, user ids.UserID, model string, inputTokens int,
) (ratelimiter.ConsumedTokensFunc, error) {
	w.tb.Helper()

	if err := w.sync(ctx); err != nil {
		return nil, fmt.Errorf("sync before consume: %w", err)
	}

	report, err := w.inner.ConsumeChatRequests(ctx, user, model, inputTokens)
	if err != nil {
		return nil, w.scaleError(err)
	}

	return func(ctx context.Context, outputTokens int) error {
		if err := w.sync(ctx); err != nil {
			return fmt.Errorf("sync before reporting: %w", err)
		}

		err := report(ctx, outputTokens)

		return w.scaleError(err)
	}, nil
}

func (w *waitWrapper) ConsumeEmbeddingRequests(
	ctx context.Context, user ids.UserID, model string, requests int,
) error {
	w.tb.Helper()

	if err := w.sync(ctx); err != nil {
		return fmt.Errorf("sync before consume: %w", err)
	}

	err := w.inner.ConsumeEmbeddingRequests(ctx, user, model, requests)

	return w.scaleError(err)
}

func (w *waitWrapper) sync(ctx context.Context) error {
	lastNow := w.mockNow()

	defer func() { w.recordedMock = lastNow }()

	if waitDiff := lastNow.Sub(w.recordedMock); waitDiff > 0 {
		timer := time.NewTimer(waitDiff)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-timer.C:
			return nil
		}
	}

	return nil
}

func (w *waitWrapper) scaleError(err error) error {
	if err == nil {
		return nil
	}

	errExceeded := new(ratelimiter.RateLimitExceededError)
	if !errors.As(err, &errExceeded) {
		return err
	}

	return ratelimiter.ErrRateLimitExceeded(errExceeded.RetryAt().Add(-w.diffSync))
}
