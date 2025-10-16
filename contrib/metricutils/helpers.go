package metricutils

import (
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

func NewCounterAndRegister(reg prometheus.Registerer, opts prometheus.CounterOpts) prometheus.Counter {
	c := prometheus.NewCounter(opts)
	reg.MustRegister(c)

	return c
}

func NewCounterVecAndRegister(reg prometheus.Registerer, opts prometheus.CounterOpts, labels []string) *prometheus.CounterVec {
	c := prometheus.NewCounterVec(opts, labels)
	reg.MustRegister(c)

	return c
}

func NewHistogramAndRegister(reg prometheus.Registerer, opts prometheus.HistogramOpts) prometheus.Histogram {
	h := prometheus.NewHistogram(opts)
	reg.MustRegister(h)

	return h
}

func NewGRPCAndRegister(reg prometheus.Registerer, buckets []float64) *grpcprom.ServerMetrics {
	metrics := grpcprom.NewServerMetrics(
		grpcprom.WithServerHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets(buckets),
		),
	)

	reg.MustRegister(metrics)

	return metrics
}
