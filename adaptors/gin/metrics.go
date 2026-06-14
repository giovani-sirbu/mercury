package ginAdaptors

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/giovani-sirbu/mercury/metrics"
)

// PrometheusMetrics is gin middleware that records every HTTP request into
// the shared mercury metrics.HTTPRequestDuration histogram + the
// HTTPRequestsTotal counter.
//
// Register globally before any other middleware (after CorrelationID is fine —
// metrics don't depend on the correlation id, they just want the
// fast-path-safe timing wrapper):
//
//	r.Use(adapter.CorrelationID)
//	r.Use(adapter.PrometheusMetrics("agora"))
//
// The serviceName parameter is the static label used for the `service` field
// on every metric — same name you'd grep for in logs.
//
// Cardinality: route is taken from c.FullPath(), which is gin's template
// ("/trades/:tradeId") not the realised URL ("/trades/123"). Cardinality is
// O(routes), not O(unique URLs). When a request doesn't match any route
// (404), FullPath() returns "" — we substitute "<nomatch>" so the metric
// still has a usable label value.
func PrometheusMetrics(serviceName string) gin.HandlerFunc {
	// Pre-resolve the in-flight gauge so the per-request fast path doesn't
	// repeatedly look up the label combination. The HTTPRequestDuration /
	// HTTPRequestsTotal vectors get looked up per-request because their
	// label set depends on the response (method, route, status) — those
	// can't be cached.
	inFlight := metrics.HTTPInFlight.WithLabelValues(serviceName)

	return func(c *gin.Context) {
		// Track in-flight count from the moment the middleware fires until
		// the response is fully written. defer guarantees the Dec runs
		// even if a downstream handler panics — without it a panic-storm
		// would leak the gauge to +∞.
		inFlight.Inc()
		defer inFlight.Dec()

		start := time.Now()

		c.Next()

		// /metrics itself self-observes — skip to avoid feedback noise.
		route := c.FullPath()
		if route == "/metrics" {
			return
		}
		if route == "" {
			route = "<nomatch>"
		}

		status := strconv.Itoa(c.Writer.Status())
		duration := time.Since(start).Seconds()

		metrics.HTTPRequestDuration.
			WithLabelValues(serviceName, c.Request.Method, route, status).
			Observe(duration)
		metrics.HTTPRequestsTotal.
			WithLabelValues(serviceName, c.Request.Method, route, status).
			Inc()
	}
}
