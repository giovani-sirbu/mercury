package ginAdaptors

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/giovani-sirbu/mercury/metrics"
)

// PrometheusMetrics is the middleware that records every HTTP request into
// HTTPRequestDuration, HTTPRequestsTotal and HTTPInFlight under serviceName.
// route is gin's template, never the realised URL, and an unmatched request
// is labelled "<nomatch>". Register it before any other middleware.
func PrometheusMetrics(serviceName string) gin.HandlerFunc {
	// Resolved once; the other vectors depend on the response.
	inFlight := metrics.HTTPInFlight.WithLabelValues(serviceName)

	return func(c *gin.Context) {
		// Deferred so a panicking handler cannot leak the gauge.
		inFlight.Inc()
		defer inFlight.Dec()

		start := time.Now()

		c.Next()

		// /metrics does not observe itself.
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
