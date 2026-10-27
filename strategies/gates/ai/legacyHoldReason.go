package ai

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// LegacyHoldReason is the gate on an open position: the ML read against the depth being armed, plus
// an explicit HOLD on anything that adds risk.
func LegacyHoldReason(event events.Events, position string, ai aggragates.AIIndicators) string {
	if event.Trade.Inverse {
		if position == "stopLoss" && ai.AIMarketBullish {
			return "AI market is bullish"
		}
		if position == "takeProfit" && ai.AIMarketBearish {
			return "AI market is bearish"
		}
	} else {
		if position == "stopLoss" && ai.AIMarketBearish {
			return "AI market is bearish"
		}
		if position == "takeProfit" && ai.AIMarketBullish {
			return "AI market is bullish"
		}
	}

	// HOLD gates only the positions that add risk; the profitable close runs.
	if ai.AIAction == aggragates.ActionHold && position != "takeProfit" {
		return "AI recommends HOLD"
	}
	return ""
}
