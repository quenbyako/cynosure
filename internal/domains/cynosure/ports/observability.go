package ports

import (
	"log/slog"

	"github.com/quenbyako/core"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type ObserveStack interface {
	Logger() slog.Handler
	Tracer() trace.Tracer
}

type observeStack struct {
	m    core.Metrics
	name string
}

func (o *observeStack) Logger() slog.Handler { return o.m }

func (o *observeStack) Tracer() trace.Tracer { return o.m.Tracer(o.name) }

type noopObserveStack struct{}

func (noopObserveStack) Logger() slog.Handler { return slog.DiscardHandler }

func (noopObserveStack) Tracer() trace.Tracer { return noop.NewTracerProvider().Tracer("") }

// NoOpObserveStack returns an empty observer stack that does nothing.
//
//nolint:ireturn // hiding implementation details
func NoOpObserveStack() ObserveStack {
	return noopObserveStack{}
}

// StackFromCore returns an observer stack that uses core metrics.
//
//nolint:ireturn // returns interface for hiding implementation details
func StackFromCore(m core.Metrics, name string) ObserveStack {
	return &observeStack{m: m, name: name}
}
