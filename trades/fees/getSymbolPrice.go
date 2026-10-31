package fees

import (
	"fmt"
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
)

// getSymbolPrice returns the price of asset quoted in USDC: the in-process
// snapshot event.WsPrices first (hermes fills it from its WS map, agora from
// hermes' /prices), then the exchange API as a last resort for callers that
// carry no snapshot.
func getSymbolPrice(event events.Events, asset string) (float64, error) {
	if slices.Contains([]string{"USDT", "USDC"}, asset) {
		return event.Trade.PositionPrice, nil
	}

	symbol := fmt.Sprintf("%s/USDC", asset)
	precision := int(event.Trade.StrategyPair.TradeFilters.PriceFilter)

	if p, ok := event.WsPrices[symbol]; ok && p > 0 {
		return helpers.ToFixed(p, precision), nil
	}

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
