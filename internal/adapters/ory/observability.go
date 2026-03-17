// Package ory observability utilities.
package ory

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type observable struct {
	tracer trace.Tracer
}

func newObservable(stack ports.ObserveStack) *observable {
	if stack == nil {
		return &observable{tracer: noop.NewTracerProvider().Tracer("")}
	}

	return &observable{
		tracer: stack.Tracer(),
	}
}

// span defines the common interface for span operations.
type span interface {
	End()
	RecordError(err error)
}

// Returns internal interface; wrapper ensures End() is called eventually.
//
//nolint:ireturn,spancheck
func (o *observable) initiateAuth(ctx context.Context, name string) (context.Context, span) {
	ctx, s := o.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))

	return ctx, &spanWrapper{s: s}
}

// Returns internal interface; wrapper ensures End() is called eventually.
//
//nolint:ireturn,spancheck
func (o *observable) step(ctx context.Context, name string) (context.Context, span) {
	ctx, s := o.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))

	return ctx, &spanWrapper{s: s}
}

type spanWrapper struct {
	s trace.Span
}

func (w *spanWrapper) End() {
	if w != nil && w.s != nil {
		w.s.End()
	}
}

func (w *spanWrapper) RecordError(err error) {
	if w != nil && err != nil && w.s != nil {
		w.s.RecordError(err)
	}
}
