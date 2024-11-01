package sender

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/goverland-labs/goverland-inbox-push/internal/metrics"
)

var metricHandleHistogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: "sender",
		Name:      "handle_duration_seconds",
		Help:      "Handle feed item event duration seconds",
		Buckets:   []float64{.001, .005, .01, .025, .05, .1, .5, 1, 2.5, 5, 10},
	}, []string{"type", "error"},
)

var metricPushCounter = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: "sender",
		Name:      "stats",
		Help:      "Some stats counters by system",
	}, []string{"subject", "method", "error"},
)

func collectStats(subject, method string, err error) {
	metricPushCounter.
		WithLabelValues(subject, method, metrics.ErrLabelValue(err)).
		Inc()
}
