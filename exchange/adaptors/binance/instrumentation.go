package binanceAdaptor

import (
	"os"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/metrics"
)

// observeBinanceCall times an outbound Binance call: it returns a closure to
// defer with the final error. endpoint is a stable name, never the raw URL.
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
		// A rate limit surfaces as code -1003 or 429.
		if errPtr != nil && (errPtr.Code == -1003 || errPtr.Code == 429) {
			metrics.ExchangeRateLimitHits.WithLabelValues(serviceName(), exchange).Inc()
		}
	}
}

// classifyAPIError maps an APIError onto the bounded status_class label.
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
		// Negative codes are Binance's own client errors.
		return "4xx"
	default:
		return "err"
	}
}

// serviceName reads SERVICE_NAME from the environment.
func serviceName() string {
	name := os.Getenv("SERVICE_NAME")
	if name == "" {
		return "unknown"
	}
	return strings.ToLower(name)
}
