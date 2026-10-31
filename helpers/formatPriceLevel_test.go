package helpers

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// tradeQuotedAt is a trade on a pair that quotes priceFilter decimals; zero is
// the pair whose PriceFilter is not known.
func tradeQuotedAt(priceFilter uint8) aggragates.Trades {
	var trade aggragates.Trades
	trade.StrategyPair.TradeFilters.PriceFilter = priceFilter
	return trade
}

func TestFormatPriceLevelFollowsThePairPriceFilter(t *testing.T) {
	cases := []struct {
		name        string
		priceFilter uint8
		price       float64
		want        string
	}{
		{"known decimals pad a short price", 2, 1.5, "1.50"},
		{"known decimals round a long price", 2, 1234.5678, "1234.57"},
		{"known decimals carry into the whole part", 1, 99.96, "100.0"},
		{"known decimals keep a sub-unit price", 8, 0.00001234, "0.00001234"},
		{"known decimals render a zero level", 2, 0, "0.00"},
		{"unknown decimals keep a short fraction", 0, 1.5, "1.5"},
		{"unknown decimals drop the fraction of a whole price", 0, 42, "42"},
		{"unknown decimals keep a long fraction exact", 0, 0.00001234, "0.00001234"},
		{"unknown decimals keep a large level exact", 0, 123456789.125, "123456789.125"},
		{"unknown decimals render a zero level", 0, 0, "0"},
	}

	for _, tc := range cases {
		got := FormatPriceLevel(tradeQuotedAt(tc.priceFilter), tc.price)
		if got != tc.want {
			t.Errorf("%s: FormatPriceLevel(priceFilter %d, %v) = %q, want %q", tc.name, tc.priceFilter, tc.price, got, tc.want)
		}
	}
}

func TestFormatPriceLevelTextIgnoresDriftBelowThePairDecimals(t *testing.T) {
	trade := tradeQuotedAt(2)
	first := FormatPriceLevel(trade, 100.001)
	second := FormatPriceLevel(trade, 100.004)
	if first != second {
		t.Errorf("levels that differ below the pair decimals rendered %q and %q, want one text", first, second)
	}
}
