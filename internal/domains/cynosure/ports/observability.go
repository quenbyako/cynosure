package ports

import (
	"github.com/quenbyako/core"
	"go.opentelemetry.io/otel/log"
	nooplog "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

type ObserveStack interface {
	Logger() log.Logger
	Tracer() trace.Tracer
}

type observeStack struct {
	m    core.Metrics
	name string
}

func (o *observeStack) Logger() log.Logger { return o.m.Logger(o.name) }

func (o *observeStack) Tracer() trace.Tracer { return o.m.Tracer(o.name) }

type noopObserveStack struct{}

func (noopObserveStack) Logger() log.Logger { return nooplog.NewLoggerProvider().Logger("") }

func (noopObserveStack) Tracer() trace.Tracer { return nooptrace.NewTracerProvider().Tracer("") }

// NoOpObserveStack returns an empty observer stack that does nothing.
func NoOpObserveStack() ObserveStack {
	return noopObserveStack{}
}

// StackFromCore returns an observer stack that uses core metrics.
func StackFromCore(m core.Metrics, name string) ObserveStack {
	return &observeStack{m: m, name: name}
}
