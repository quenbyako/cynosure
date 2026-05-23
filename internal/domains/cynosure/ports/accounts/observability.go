package accounts

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

//nolint:spancheck // isolated in a wrapper
func (o *observable) getAccount(
	ctx context.Context, accountID string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.accounts.get_account",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.accounts.account_id").String(accountID),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck // isolated in a wrapper
func (o *observable) getAccountsBatch(
	ctx context.Context, accounts []string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.accounts.get_accounts_batch",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.accounts.account_id").StringSlice(accounts),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck // isolated in a wrapper
func (o *observable) listAccounts(
	ctx context.Context, userID string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.accounts.list_accounts",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.accounts.user_id").String(userID),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck // isolated in a wrapper
func (o *observable) deleteAccount(
	ctx context.Context, accountID string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.accounts.delete_account",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.accounts.account_id").String(accountID),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck // isolated in a wrapper
func (o *observable) reactivateAccount(
	ctx context.Context, accountID string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.accounts.reactivate_account",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.accounts.account_id").String(accountID),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck // isolated in a wrapper
func (o *observable) saveAccount(
	ctx context.Context, accountID string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.accounts.save_account",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.accounts.account_id").String(accountID),
		),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck // isolated in a wrapper
func (o *observable) findAccountsByName(
	ctx context.Context, userID, name string,
) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.accounts.find_accounts_by_name",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Key("cynosure.accounts.user_id").String(userID),
			attribute.Key("cynosure.accounts.name").String(name),
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
	if c != nil && c.span != nil {
		c.span.End()
	}
}

func (c *spanCallback) recordError(err error) {
	if err != nil && c != nil && c.span != nil {
		c.span.RecordError(err)
		c.span.SetStatus(codes.Error, err.Error())
	}
}
