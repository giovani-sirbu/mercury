package patterns

import (
	"strconv"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// formatPriceLevel renders a price the way the pair quotes it: the pair's
// PriceFilter decimals when known, the shortest exact form otherwise. Levels
// go into hold messages (never the score, which drifts every bar), so the
// text stays stable for the 24h dedup.
func formatPriceLevel(trade aggragates.Trades, price float64) string {
	if decimals := int(trade.StrategyPair.TradeFilters.PriceFilter); decimals > 0 {
		return strconv.FormatFloat(price, 'f', decimals, 64)
	}
	return strconv.FormatFloat(price, 'f', -1, 64)
}
