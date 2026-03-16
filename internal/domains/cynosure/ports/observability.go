package ports

import (
	"log/slog"

	"github.com/quenbyako/core"
	"go.opentelemetry.io/otel/trace"
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

//nolint:ireturn // opentelemetry details: impossible to retrieve exact struct
func (o *observeStack) Tracer() trace.Tracer { return o.m.Tracer(o.name) }

//nolint:ireturn // returns interface for hiding implementation details
func StackFromCore(m core.Metrics, name string) ObserveStack {
	return &observeStack{m: m, name: name}
}
