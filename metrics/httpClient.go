package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPClientRequestDuration times outbound HTTP calls per caller and target host.
var HTTPClientRequestDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_client_request_duration_seconds",
		Help:    "Outbound HTTP client request duration, per caller and target host.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16), // 1ms → ~32s
	},
	[]string{"service", "target_host", "method", "status_class"},
)

// HTTPClientRequestsTotal counts outbound HTTP requests with the histogram's label set.
var HTTPClientRequestsTotal = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_client_requests_total",
		Help: "Total outbound HTTP client requests, partitioned by status_class.",
	},
	[]string{"service", "target_host", "method", "status_class"},
)
