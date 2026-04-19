package identitymanager

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type observable struct {
	t trace.Tracer
	l log.Logger
}

func newObservable(stack ports.ObserveStack) *observable {
	if stack == nil {
		stack = ports.NoOpObserveStack()
	}

	return &observable{
		t: stack.Tracer(),
		l: stack.Logger(),
	}
}

// trace callbacks

//nolint:spancheck,ireturn // intentional polymorphism: returns internal span interface
func (o *observable) hasUser(
	ctx context.Context, userID string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.identity.has_user",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.identity.user_id").String(userID),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck,ireturn // isolated in a wrapper
func (o *observable) lookupUser(
	ctx context.Context, telegramID string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.identity.lookup_user",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.identity.telegram_id").String(telegramID),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck,ireturn // intentional polymorphism: returns internal span interface
func (o *observable) createUser(
	ctx context.Context, telegramID, nickname, firstName, lastName string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.identity.create_user",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.identity.telegram_id").String(telegramID),
			attribute.Key("cynosure.identity.nickname").String(nickname),
			attribute.Key("cynosure.identity.first_name").String(firstName),
			attribute.Key("cynosure.identity.last_name").String(lastName),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck,ireturn // intentional polymorphism: returns internal span interface
func (o *observable) issueToken(
	ctx context.Context, userID string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.identity.issue_token",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.identity.user_id").String(userID),
		),
	)

	return ctx, &spanCallback{span: span}
}

// log callbacks

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
