package ratelimiter

import (
	"context"

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

func (t *portWrapped) _PortWrapped() {}

// Wrap wraps the given port with observability tools.
func Wrap(client Port, observable ports.ObserveStack) PortWrapped {
	if observable == nil {
		observable = ports.NoOpObserveStack()
	}

	t := portWrapped{
		w: client,
		t: newObservable(observable),
	}

	return &t
}

// Consume consumes rate limit of messages for the given user.
func (t *portWrapped) Consume(ctx context.Context, user ids.UserID, n int) (err error) {
	ctx, span := t.t.consume(ctx, user, n)
	defer span.end()

	err = t.w.Consume(ctx, user, n)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap adapter errors
	return err
}
