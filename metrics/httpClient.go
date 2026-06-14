package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPClientRequestDuration observes the duration of every outbound HTTP
// call made by a service. Complements the server-side
// http_request_duration_seconds — server-side answers "how fast did *I*
// respond", client-side answers "how fast did the downstream respond to
// *me*". When a route's server p99 climbs, the corresponding outbound
// client p99 tells you whether you're the bottleneck or your downstream is.
//
// Labels:
//   - service:        caller (agora, hermes, etc.)
//   - target_host:    req.URL.Host — bounded because each service maps
//                     to a stable hostname (agora:1001, hermes:1002, ...)
//   - method:         GET / POST / etc.
//   - status_class:   2xx / 3xx / 4xx / 5xx / transport_error (no response)
var HTTPClientRequestDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_client_request_duration_seconds",
		Help:    "Outbound HTTP client request duration, per caller and target host.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 16), // 1ms → ~32s
	},
	[]string{"service", "target_host", "method", "status_class"},
)

// HTTPClientRequestsTotal counts outbound HTTP requests. Combined with the
// histogram's _count series this is redundant; we keep it for symmetry
// with HTTPRequestsTotal and easier "calls per second" queries.
var HTTPClientRequestsTotal = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_client_requests_total",
		Help: "Total outbound HTTP client requests, partitioned by status_class.",
	},
	[]string{"service", "target_host", "method", "status_class"},
)
