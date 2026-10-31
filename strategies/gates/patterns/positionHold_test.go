package patterns

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// positionEvent is an open long trade in position at price, its last fill at 100000.
func positionEvent(position string, price float64) events.Events {
	trade := testutil.NewHoldTrade(position, false)
	trade.PositionPrice = price
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}
	return events.Events{Trade: trade, Params: aggragates.Params{OldPosition: "buy"}}
}

// The gate's hold is keyed by the rule that spoke: a chart pattern on a trailing buy or a take
// profit is HOLD_PATTERN, the fibonacci wait HOLD_FIBONACCI; the first fill and a release hold nothing.
func TestPositionHoldIsKeyedByTheRuleThatSpoke(t *testing.T) {
	pattern := testutil.FibAI()
	pattern.PatternName, pattern.PatternDisplayName, pattern.PatternDirection, pattern.PatternScore = "bull_flag", "bull flag", "long", 80
	pattern.PatternTakeProfit = 120
	for _, c := range []struct {
		name     string
		position string
		price    float64
		ai       aggragates.AIIndicators
		key      string
		reason   string
	}{
		{"a pattern on a trailing buy", "trailingBuy", 106.5, pattern, aggragates.LogKeyHoldPattern, "pattern: bull flag found, preventing the trailing buy"},
		{"the fibonacci wait", "trailingBuy", 106.5, testutil.FibAI(), aggragates.LogKeyHoldFibonacci, "fibonacci: waiting for a better price (next level 106.18)"},
		{"a pattern riding to its target", "takeProfit", 106.5, pattern, aggragates.LogKeyHoldPattern, "pattern: bull flag in play, riding to target 120.00"},
		{"nothing on a take profit without a pattern", "takeProfit", 106.5, testutil.FibAI(), "", ""},
	} {
		hold := PositionHold(positionEvent(c.position, c.price), c.position, c.ai)
		if hold.Key != c.key || hold.Reason != c.reason || hold.Param != "" {
			t.Errorf("%s: hold %+v, want %q keyed %q with no event", c.name, hold, c.reason, c.key)
		}
	}
	entry := positionEvent("trailingBuy", 106.5)
	entry.Params.OldPosition = "new"
	if hold := PositionHold(entry, "trailingBuy", pattern); hold.Held() {
		t.Errorf("the first fill is never the gate's, got %+v", hold)
	}
}
