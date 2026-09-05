package fees

import (
	"fmt"
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
)

// getSymbolPrice returns the real-time price of `asset` quoted in USDC.
//
// Lookup order (cheapest first):
//  1. event.WsPrices — in-process snapshot. Hermes populates it from its own WS
//     map; agora populates it by calling hermes' GET /prices (TTL-cached in
//     agora/helpers/getWsPrices.go). Callers that do not populate WsPrices (e.g.
//     sisyphus backtesting on a virtual exchange) fall through to step 2.
//  2. exchange API — last-resort network call via event.Exchange.Client().
//
// A prior version held a package-level `var wsPrices` map to memoise results across calls.
// That variable was shared by every concurrent goroutine calling GetFees and was never
// protected by a lock, producing a real race condition on a financial code path. It has
// been removed; the stateful cache now lives where it belongs — on the event.
//
// A second prior version read from Dragonfly at key "ws-symbols-price" as an
// intermediate fallback. That key has no producer since hermes stopped publishing
// the snapshot to the shared cache (/prices HTTP handoff replaces it), so the
// branch was always a miss and has been removed.
func getSymbolPrice(event events.Events, asset string) (float64, error) {
	if slices.Contains([]string{"USDT", "USDC"}, asset) {
		return event.Trade.PositionPrice, nil
	}

	symbol := fmt.Sprintf("%s/USDC", asset)
	precision := int(event.Trade.StrategyPair.TradeFilters.PriceFilter)

	// 1. In-process snapshot.
	if p, ok := event.WsPrices[symbol]; ok && p > 0 {
		return helpers.ToFixed(p, precision), nil
	}

	// 2. Exchange API fallback.
	client, err := event.Exchange.Client()
	if err != nil {
		return 0, err
	}
	price, priceErr := client.GetPrice(symbol)
	if priceErr != nil {
		return 0, priceErr
	}
	return helpers.ToFixed(price, precision), nil
}
