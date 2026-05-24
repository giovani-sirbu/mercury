package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ExchangeRequestDuration is observed around every outbound REST call to a
// crypto exchange. Tracks both latency and call volume.
//
// Labels:
//   - service:      which HERMATIC service made the call
//   - exchange:     "binance" / "binance-futures" / future additions
//   - endpoint:     a stable identifier for the endpoint family ("get-order",
//                   "place-order", "balance") — NEVER the raw URL (that would
//                   bake symbol names into label cardinality).
//   - status_class: "2xx" / "4xx" / "5xx" / "err" (transport error) — keeps
//                   the success/failure split clear without storing every code.
var ExchangeRequestDuration = promauto.With(Registry).NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "exchange_request_duration_seconds",
		Help:    "Outbound exchange REST/HTTP request duration in seconds.",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 13), // 10ms → ~80s
	},
	[]string{"service", "exchange", "endpoint", "status_class"},
)

// ExchangeRateLimitHits counts 429 responses from the exchange API. A
// non-zero rate is your "back off cron or increase delay" signal.
var ExchangeRateLimitHits = promauto.With(Registry).NewCounterVec(
	prometheus.CounterOpts{
		Name: "exchange_rate_limit_hits_total",
		Help: "Total HTTP 429 responses from exchange APIs.",
	},
	[]string{"service", "exchange"},
)
