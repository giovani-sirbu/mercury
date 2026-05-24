package binanceAdaptor

import (
	"os"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/metrics"
)

// observeBinanceCall wraps an outbound Binance SDK call with timing +
// status-class metrics. Use it inside every public method on `Binance` that
// hits the network.
//
// endpoint is a stable string identifier ("create-order", "get-order",
// "get-exchange-info") — never the raw URL, because the URL embeds symbol
// names and would blow up label cardinality.
//
// The function returns the original error so call sites can do:
//
//	defer observeBinanceCall("create-order", "spot")(&err)
//
// (deferred-closure pattern: the helper returns a closure that observes on
// scope exit, and reads the final error value via pointer).
//
// Why a closure-returning-closure: Go has no try/finally. To capture the
// final value of `err` after named-return assignments, the observer needs a
// pointer that the deferred closure can re-read at exit time. The pattern
// keeps call sites to a single line.
func observeBinanceCall(endpoint, market string) func(*common.APIError) {
	start := time.Now()
	return func(errPtr *common.APIError) {
		dur := time.Since(start).Seconds()
		statusClass := classifyAPIError(errPtr)
		exchange := "binance"
		if market != "" && market != "spot" {
			exchange = "binance-" + market
		}
		metrics.ExchangeRequestDuration.
			WithLabelValues(serviceName(), exchange, endpoint, statusClass).
			Observe(dur)
		// Binance signals rate-limit via HTTP 429 → SDK surfaces as APIError
		// with Code == -1003 (TOO_MANY_REQUESTS) or HTTP-style 429. Both map
		// to "rate_limit" so the counter is honest regardless of which path.
		if errPtr != nil && (errPtr.Code == -1003 || errPtr.Code == 429) {
			metrics.ExchangeRateLimitHits.WithLabelValues(serviceName(), exchange).Inc()
		}
	}
}

// classifyAPIError maps the SDK's APIError shape onto the four-bucket
// status_class label. Avoids putting the raw error code into a label (the
// code set is unbounded in practice — Binance has dozens) while keeping the
// success/failure split queryable in PromQL.
func classifyAPIError(err *common.APIError) string {
	if err == nil {
		return "2xx"
	}
	switch {
	case err.Code == -1003 || err.Code == 429:
		return "rate_limit"
	case err.Code > 0 && err.Code < 1000:
		return "5xx"
	case err.Code < 0:
		// Negative codes are Binance's own error codes (e.g. -2010 for
		// insufficient funds). These are client errors — the caller did
		// something wrong, not the server.
		return "4xx"
	default:
		return "err"
	}
}

// serviceName reads SERVICE_NAME from the environment. Each service sets
// this in its config; mercury reads it directly here because the exchange
// adaptor has no other way to know which HERMATIC service is making the call.
func serviceName() string {
	name := os.Getenv("SERVICE_NAME")
	if name == "" {
		return "unknown"
	}
	return strings.ToLower(name)
}
