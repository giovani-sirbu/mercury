package ai

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// LegacyHoldReason is the UseAI flag's gate on an open position: the ML
// route's bullish/bearish read against the rung being armed, plus an
// explicit HOLD on anything that adds risk.
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

	// Explicit HOLD means "no direction", so it gates only positions that
	// add risk (rebuys/averaging). A profitable close reduces exposure and
	// is allowed to execute — same rule for inverse trades, where
	// takeProfit is still the profitable close.
	if ai.AIAction == aggragates.ActionHold && position != "takeProfit" {
		return "AI recommends HOLD"
	}
	return ""
}
