package dynamicparams_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// flaggedTrade is a long spot parent ladder with DynamicParams on — the one
// shape the flag applies to — on the three-row ladder.
func flaggedTrade() aggragates.Trades {
	trade := aggragates.Trades{PositionPrice: 100, PositionType: "buy"}
	trade.Strategy.TradeType = aggragates.Spot
	trade.Strategy.Params.DynamicParams = true
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, PriceFilter: 2, MinNotional: 5}
	trade.StrategyPair.StrategySettings = threeRowLadder()
	return trade
}

// Long spot parent ladders only, and only with the flag on.
func TestAppliesToLongSpotParentLaddersOnly(t *testing.T) {
	cases := []struct {
		name   string
		change func(*aggragates.Trades)
		want   bool
	}{
		{"a long spot parent with the flag", func(*aggragates.Trades) {}, true},
		{"a strategy whose trade type was never loaded", func(trade *aggragates.Trades) { trade.Strategy.TradeType = "" }, true},
		{"the flag off", func(trade *aggragates.Trades) { trade.Strategy.Params.DynamicParams = false }, false},
		{"an inverse ladder", func(trade *aggragates.Trades) { trade.Inverse = true }, false},
		{"a futures strategy", func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures }, false},
		{"an impasse child", func(trade *aggragates.Trades) { trade.ParentID = 7 }, false},
	}

	for _, c := range cases {
		trade := flaggedTrade()
		c.change(&trade)
		if got := dynamicparams.Applies(trade); got != c.want {
			t.Errorf("%s: Applies = %v, want %v", c.name, got, c.want)
		}
	}
}
