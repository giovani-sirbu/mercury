package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPRequestDuration is the per-request server-side histogram, observed by
// the gin middleware in mercury/adaptors/gin. Use `histogram_quantile(0.99,
// rate(http_request_duration_seconds_bucket{service="agora"}[5m]))` for a
// per-service p99 latency line in Grafana.
//
// Labels are deliberately bounded:
//   - service: agora / hermes / hellenes / iris / sophos
//   - method:  GET / POST / etc. (always a small set)
//   - route:   gin's c.FullPath() — the template, NOT the realised URL. This
//              keeps cardinality at O(routes) instead of O(unique URLs).
//              Empty for routes that didn't match (NoRoute / 404 path).
//   - status:  HTTP status code as string ("200", "404", "500", ...)
var HTTPRequestDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP server request duration in seconds, observed at handler exit.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16), // 1ms → ~32s
	},
	[]string{"service", "method", "route", "status"},
)

// HTTPRequestsTotal complements the histogram with a plain counter so request
// rate is queryable without going through `_count` math on the histogram.
// Same label set so the two can be cross-referenced.
var HTTPRequestsTotal = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP server requests, partitioned by status.",
	},
	[]string{"service", "method", "route", "status"},
)

// HTTPInFlight tracks the current number of in-flight HTTP requests per
// service. Incremented on middleware entry, decremented on exit via defer
// so it stays accurate even if a handler panics.
//
// Why this matters even when we have duration + RPS: latency can climb
// either because individual requests are slow OR because requests are
// queuing behind a saturated worker. RPS + latency look the same in both
// cases; in-flight count tells them apart. A spike in in-flight without a
// matching duration spike = a flood of concurrent traffic; durations
// rising while in-flight stays flat = handlers genuinely slowing down.
var HTTPInFlight = promauto.With(Registry).NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "http_in_flight_requests",
		Help: "Number of HTTP requests currently being served per service.",
	},
	[]string{"service"},
)
