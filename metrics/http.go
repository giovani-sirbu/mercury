package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPRequestDuration times server requests; route is gin's template, never the realised URL.
var HTTPRequestDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP server request duration in seconds, observed at handler exit.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16), // 1ms → ~32s
	},
	[]string{"service", "method", "route", "status"},
)

// HTTPRequestsTotal counts server requests with the histogram's label set.
var HTTPRequestsTotal = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP server requests, partitioned by status.",
	},
	[]string{"service", "method", "route", "status"},
)

// HTTPInFlight is the number of requests currently being served per service.
var HTTPInFlight = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "http_in_flight_requests",
		Help: "Number of HTTP requests currently being served per service.",
	},
	[]string{"service"},
)
