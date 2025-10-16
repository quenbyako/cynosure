package metricutils

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func ObserveDuration(h prometheus.Histogram) func(time.Duration) {
	return func(d time.Duration) { h.Observe(d.Seconds()) }
}
