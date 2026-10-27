package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ExchangeRequestDuration times outbound exchange REST calls; endpoint is a
// stable family name, never the raw URL.
var ExchangeRequestDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "exchange_request_duration_seconds",
		Help:    "Outbound exchange REST/HTTP request duration in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 13), // 10ms → ~80s
	},
	[]string{"service", "exchange", "endpoint", "status_class"},
)

// ExchangeRateLimitHits counts 429 responses from the exchange API.
var ExchangeRateLimitHits = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "exchange_rate_limit_hits_total",
		Help: "Total HTTP 429 responses from exchange APIs.",
	},
	[]string{"service", "exchange"},
)
