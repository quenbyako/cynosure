package metricutils

import "github.com/prometheus/client_golang/prometheus"

// Observer is the interface that wraps the Observe method, which is used by
// Histogram and Summary to add observations.
//
// NOTE: just for a smaller imports list.
type Observer = prometheus.Observer

// Counter is a Metric that represents a single numerical value that only ever
// goes up. That implies that it cannot be used to count items whose number can
// also go down, e.g. the number of currently running goroutines. Those
// "counters" are represented by Gauges.
//
// A Counter is typically used to count requests served, tasks completed, errors
// occurred, etc.
//
// To create Counter instances, use [prometheus.NewCounter].
type Counter interface {
	// Inc increments the counter by 1. Use Add to increment it by arbitrary
	// non-negative values.
	Inc()
	// Add adds the given value to the counter. It panics if the value is <
	// 0.
	Add(float64)
}

var _ Counter = prometheus.Counter(nil)

type NoOpCounter struct{}

var _ Counter = NoOpCounter{}

func (NoOpCounter) Inc()        {}
func (NoOpCounter) Add(float64) {}

type NoOpObserver struct{}

var _ prometheus.Observer = NoOpObserver{}

func (NoOpObserver) Observe(float64) {}
