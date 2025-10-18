package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	AllocationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "allocator_allocations_total",
			Help: "Total allocation attempts",
		},
		[]string{"result"}, // success|failure
	)

	AllocationDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "allocator_allocation_duration_seconds",
			Help:    "Duration of allocation processing",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	prometheus.MustRegister(AllocationsTotal)
	prometheus.MustRegister(AllocationDuration)
}

func Register(mux *http.ServeMux) {
	mux.Handle("/metrics", promhttp.Handler())
}
