package adaptors

import (
	"fmt"
	"os"

	binanceAdaptor "github.com/giovani-sirbu/mercury/exchange/adaptors/binance"
	ccxtAdaptor "github.com/giovani-sirbu/mercury/exchange/adaptors/ccxt"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// EXCHANGE_REST_BACKEND env var selects which REST client backs the spot
// Actions function-bag. Two values are recognised:
//
//	"ccxt"           - default. Routes Binance + Crypto.com through the
//	                   official github.com/ccxt/ccxt/go/v4 library. Provides
//	                   the multi-exchange surface we need now that Binance has
//	                   retired /api/v3/userDataStream (which the legacy
//	                   adshao/go-binance library still calls).
//	"binance-legacy" - revert switch. Forces Binance back through the
//	                   adshao/go-binance adaptor. Crypto.com is rejected
//	                   under this mode because the legacy adaptor doesn't
//	                   know about it. Use only to A/B compare during the
//	                   migration window — remove once CCXT is proven.
//
// Reading the env var per call costs one map lookup; mercury's call sites
// here are not hot-path (per-trade, not per-tick) so we keep the read inline
// rather than caching at package init.

// GetExchangeActions returns the spot Actions function-bag for the given
// exchange, honouring the EXCHANGE_REST_BACKEND env flag.
func GetExchangeActions(e aggregates.Exchange) (aggregates.Actions, error) {
	if e.Name == "" {
		return aggregates.Actions{}, fmt.Errorf("missing required payload")
	}
	backend := os.Getenv("EXCHANGE_REST_BACKEND")
	switch e.Name {
	case "binance":
		if backend == "binance-legacy" {
			return binanceAdaptor.GetBinanceActions(e), nil
		}
		return ccxtAdaptor.GetCCXTActions(e), nil
	case "cryptocom":
		// Crypto.com is CCXT-only — no legacy adaptor exists.
		return ccxtAdaptor.GetCCXTActions(e), nil
	default:
		return aggregates.Actions{}, fmt.Errorf("exchange not allowed: %s", e.Name)
	}
}

// GetFuturesExchangeActions returns the futures Actions function-bag.
// Futures stays on the legacy adshao/go-binance path in this PR regardless
// of EXCHANGE_REST_BACKEND — porting futures to CCXT is deferred to a
// follow-up. The CCXT futures adaptor in this repo just forwards to the
// legacy implementation today, so the routing here mirrors that decision
// explicitly.
func GetFuturesExchangeActions(e aggregates.Exchange) (aggregates.FuturesActions, error) {
	if e.Name == "" {
		return aggregates.FuturesActions{}, fmt.Errorf("missing required payload")
	}
	if e.Name == "binance" {
		return binanceAdaptor.GetFuturesBinanceActions(e), nil
	}
	return aggregates.FuturesActions{}, fmt.Errorf("exchange not allowed for futures: %s", e.Name)
}
