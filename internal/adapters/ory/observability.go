package ory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/quenbyako/core"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

const (
	eventOryRequestStarted  = "cynosure.adapters.ory.request_started"
	eventOryRequestFinished = "cynosure.adapters.ory.request_finished"
)

type observable struct {
	t trace.Tracer
	l slog.Handler
}

func newObservable(stack ports.ObserveStack) *observable {
	if stack == nil {
		panic("required observability stack")
	}

	o := &observable{
		t: stack.Tracer(),
		l: stack.Logger(),
	}

	return o
}

// trace callbacks

type initiateAuthCallback interface {
	span
}

func (o *observable) initiateAuth(ctx context.Context, name string) (context.Context, initiateAuthCallback) {
	ctx, span := o.t.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	return ctx, &spanCallback{span: span}
}

type issueTokenStepCallback interface {
	span
}

func (o *observable) step(ctx context.Context, name string) (context.Context, issueTokenStepCallback) {
	ctx, span := o.t.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	return ctx, &spanCallback{span: span}
}

// utilities

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
	if c != nil && err != nil && c.span != nil {
		c.span.RecordError(err)
	}
}

type eventBuilder struct {
	ctx context.Context
	h   slog.Handler
	r   slog.Record
}

func (l *observable) event(ctx context.Context, level slog.Level, eventType string) *eventBuilder {
	if !l.l.Enabled(ctx, level) {
		return nil
	}

	app, _ := core.AppNameFromContext(ctx)
	version, _ := core.VersionFromContext(ctx)

	event := slog.NewRecord(time.Now(), level, "", 0)
	event.AddAttrs(
		slog.String("service", omitOK(app.Name())),
		slog.String("version", version.String()),
		slog.String("event.name", eventType),
	)

	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		event.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	return &eventBuilder{ctx: ctx, h: l.l, r: event}
}

func (e *eventBuilder) Context(attrs ...attribute.KeyValue) *eventBuilder {
	if e != nil {
		e.r.AddAttrs(slog.GroupAttrs("context", attrsToSlog(attrs...)...))
	}

	return e
}

func (e *eventBuilder) Msgf(format string, v ...any) { e.Msg(fmt.Sprintf(format, v...)) }
func (e *eventBuilder) Msg(msg string) {
	e.r.Message = msg
	e.h.Handle(e.ctx, e.r)
}

func omitOK[T any](s T, ok bool) T {
	if !ok {
		return *new(T)
	}

	return s
}

func attrsToSlog(attrs ...attribute.KeyValue) []slog.Attr {
	res := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		res[i] = slog.Attr{
			Key:   string(attr.Key),
			Value: valueSlog(attr.Value),
		}
	}

	return res
}

func valueSlog(attrs attribute.Value) slog.Value {
	switch attrs.Type() {
	case attribute.BOOL:
		return slog.BoolValue(attrs.AsBool())
	case attribute.BOOLSLICE:
		return slog.AnyValue(attrs.AsBoolSlice())
	case attribute.FLOAT64:
		return slog.Float64Value(attrs.AsFloat64())
	case attribute.FLOAT64SLICE:
		return slog.AnyValue(attrs.AsFloat64Slice())
	case attribute.INT64:
		return slog.Int64Value(attrs.AsInt64())
	case attribute.INT64SLICE:
		return slog.AnyValue(attrs.AsInt64Slice())
	case attribute.STRING:
		return slog.StringValue(attrs.AsString())
	case attribute.STRINGSLICE:
		return slog.AnyValue(attrs.AsStringSlice())
	default:
		return slog.AnyValue(attrs)
	}
}
