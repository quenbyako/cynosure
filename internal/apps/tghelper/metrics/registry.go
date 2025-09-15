package metrics

import (
	"context"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type Metrics interface {
	Gatherer() prometheus.Gatherer

	trace.TracerProvider
	metric.MeterProvider
}

type MetricsPullFuncs struct {
	// nothing here yet
	SomeOutStorage func(metric.Meter)
}

type metrics struct {
	register prometheus.Gatherer
	metric.MeterProvider
	trace.TracerProvider
	pullRegistred atomic.Bool
}

var _ Metrics = (*metrics)(nil)

type metricsParams struct {
	traceURL *url.URL
}

type RegisterOption func(*metricsParams)

func WithTraceURL(u *url.URL) RegisterOption {
	return func(p *metricsParams) { p.traceURL = u }
}

func RegisterPushMetrics(ctx context.Context, service, version string, opts ...RegisterOption) (Metrics, func(MetricsPullFuncs)) {
	var p metricsParams
	for _, opt := range opts {
		opt(&p)
	}

	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(service),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		panic(err)
	}

	var traceProvider trace.TracerProvider = noop.NewTracerProvider()
	if u := p.traceURL; u != nil {
		var exporter sdktrace.SpanExporter
		var err error
		switch u.Scheme {
		case "http", "https":
			exporter, err = otlptracehttp.New(
				ctx,
				otlptracehttp.WithEndpointURL(u.String()),
			)
		case "grpc":
			exporter, err = otlptracegrpc.New(
				ctx,
				otlptracegrpc.WithEndpoint(u.Host),
				otlptracegrpc.WithInsecure(),
			)
		default:
			panic("unsupported trace exporter protocol")
		}
		if err != nil {
			panic(err)
		}

		traceProvider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(
				exporter,
				sdktrace.WithMaxExportBatchSize(sdktrace.DefaultMaxExportBatchSize),
				sdktrace.WithBatchTimeout(sdktrace.DefaultScheduleDelay*time.Millisecond),
				sdktrace.WithMaxExportBatchSize(sdktrace.DefaultMaxExportBatchSize),
			),
			sdktrace.WithResource(r),
		)
	}

	promreg := prometheus.NewRegistry()
	prometheusExporter, err := otelprometheus.New(
		otelprometheus.WithRegisterer(promreg),
	)
	if err != nil {
		panic(err)
	}
	metricProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(r),
		sdkmetric.WithReader(prometheusExporter),
	)

	m := &metrics{
		register:       promreg,
		MeterProvider:  metricProvider,
		TracerProvider: traceProvider,
	}

	return m, m.registerPullMetrics
}

func (m *metrics) registerPullMetrics(funcs MetricsPullFuncs) {
	if m.pullRegistred.CompareAndSwap(false, true) {
		// =============================
		// add metrics registration here
		// =============================

	} else {
		panic("pull metrics already registred")
	}
}

func (m *metrics) Gatherer() prometheus.Gatherer {
	if !m.pullRegistred.Load() {
		panic("pull metrics not registred")
	}

	return m.register
}
