package ratelimiter

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	cynosureUserID            attribute.Key = "cynosure.user_id"
	cynosureRatelimiterAmount attribute.Key = "cynosure.ratelimiter.amount"
)

type observable struct {
	t trace.Tracer
}

func newObservable(stack ports.ObserveStack) *observable {
	if stack == nil {
		stack = ports.NoOpObserveStack()
	}

	return &observable{
		t: stack.Tracer(),
	}
}

// trace callbacks

//nolint:spancheck,ireturn // intentional polymorphism: returns internal span interface
func (o *observable) consume(
	ctx context.Context, user ids.UserID, amount int,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.ratelimiter.consume",
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
