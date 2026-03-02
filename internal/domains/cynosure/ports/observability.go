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
	name string
	m    core.Metrics
}

func (o *observeStack) Logger() slog.Handler { return o.m }

func (o *observeStack) Tracer() trace.Tracer { return o.m.Tracer(o.name) }

func StackFromCore(m core.Metrics, name string) ObserveStack {
	return &observeStack{m: m, name: name}
}
