package gateway

import (
	"context"
	"errors"
	"log/slog"

	"github.com/quenbyako/cynosure/contrib/onelog"
	"go.opentelemetry.io/otel/metric"
	noopMetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	noopTrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/gateway/usecases"
)

type SecretGetter interface {
	Get(ctx context.Context) ([]byte, error)
}

type appParams struct {
	log    slog.Handler
	meter  metric.MeterProvider
	tracer trace.TracerProvider
}

func (p *appParams) validate() error {
	var errs []error

	return errors.Join(errs...)
}

type AppOpts func(*appParams)

func WithObservability(
	log slog.Handler,
	meter metric.MeterProvider,
	tracer trace.TracerProvider,
) AppOpts {
	return func(p *appParams) {
		p.log = log
		p.meter = meter
		p.tracer = tracer
	}
}

func NewApp(ctx context.Context, opts ...AppOpts) *App {
	p := appParams{
		log:    slog.DiscardHandler,
		tracer: noopTrace.NewTracerProvider(),
		meter:  noopMetric.NewMeterProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}
	if err := p.validate(); err != nil {
		panic(err)
	}

	return must(buildApp(ctx, &p))
}

func newApp(
	p *appParams,
	usecase *usecases.Usecase,
) (*App, error) {

	return &App{
		log: onelog.Wrap(p.log),
	}, nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
