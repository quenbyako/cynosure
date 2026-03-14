package oauthhandler

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type observable struct {
	t trace.Tracer
	l slog.Handler
}

func newObservable(stack ports.ObserveStack) *observable {
	if stack == nil {
		panic("required observable stack")
	}

	return &observable{
		t: stack.Tracer(),
		l: stack.Logger(),
	}
}

// trace callbacks

type registerClientCallback interface {
	span
}

func (o *observable) registerClient(ctx context.Context, resourceURL, clientName string) (context.Context, registerClientCallback) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.oauth.register_client",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			semconv.URLFull(resourceURL),
			attribute.Key("cynosure.oauth.client_name").String(clientName),
		),
	)

	return ctx, &spanCallback{span: span}
}

type refreshTokenCallback interface {
	span
}

func (o *observable) refreshToken(ctx context.Context, clientID, authURL string) (context.Context, refreshTokenCallback) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.oauth.refresh_token",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			semconv.URLFull(authURL),
			attribute.Key("cynosure.oauth.client_id").String(clientID),
		),
	)

	return ctx, &spanCallback{span: span}
}

type exchangeCallback interface {
	span
}

func (o *observable) exchange(ctx context.Context, clientID, authURL string) (context.Context, exchangeCallback) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.oauth.exchange",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			semconv.URLFull(authURL),
			attribute.Key("cynosure.oauth.client_id").String(clientID),
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
