package actions

import (
	"github.com/giovani-sirbu/mercury/events"
)

const (
	ActionHold  = "HOLD"
	ActionLong  = "LONG"
	ActionShort = "SHORT"
)

// ShouldHold blocks the action chain when the AI (or the classic cooldown
// fallback) advises against acting on the current position. Holds are
// recorded by saveHoldLog as INFO trade-log entries.
func ShouldHold(event events.Events) (events.Events, error) {
	cool := event.Params.CoolDownIndicators
	ai := event.Params.AIIndicators

	// A trade that has not entered yet is gated only on a clearly opposing
	// signal.
	if event.Params.OldPosition == "new" {
		if ai.UseAI {
			isBearishSignal := ai.AIMarketBearish || ai.AIAction == ActionShort
			isBullishSignal := ai.AIMarketBullish || ai.AIAction == ActionLong

			if !event.Trade.Inverse && isBearishSignal {
				return saveHoldLog(event, "entry", "AI market is bearish")
			}
			if event.Trade.Inverse && isBullishSignal {
				return saveHoldLog(event, "entry", "AI market is bullish")
			}
		}
		return event, nil
	}

	holdReason := ""

	if ai.UseAI {
		if event.Trade.Inverse {
			if event.Trade.PositionType == "stopLoss" && ai.AIMarketBullish {
				holdReason = "AI market is bullish"
			}
			if event.Trade.PositionType == "takeProfit" && ai.AIMarketBearish {
				holdReason = "AI market is bearish"
			}
		} else {
			if event.Trade.PositionType == "stopLoss" && ai.AIMarketBearish {
				holdReason = "AI market is bearish"
			}
			if event.Trade.PositionType == "takeProfit" && ai.AIMarketBullish {
				holdReason = "AI market is bullish"
			}
		}

		// Explicit HOLD means "no direction", so it gates only positions that
		// add risk (rebuys/averaging). A profitable close reduces exposure and
		// is allowed to execute — same rule for inverse trades, where
		// takeProfit is still the profitable close.
		if holdReason == "" && ai.AIAction == ActionHold && event.Trade.PositionType != "takeProfit" {
			holdReason = "AI recommends HOLD"
		}
	} else {
		if event.Trade.Inverse {
			if event.Trade.PositionType == "stopLoss" && cool.MarketBullish {
				holdReason = "cooldown market is bullish"
			}
			if event.Trade.PositionType == "takeProfit" && cool.MarketBearish {
				holdReason = "cooldown market is bearish"
			}
		} else {
			if event.Trade.PositionType == "stopLoss" && cool.MarketBearish {
				holdReason = "cooldown market is bearish"
			}
			if event.Trade.PositionType == "takeProfit" && cool.MarketBullish {
				holdReason = "cooldown market is bullish"
			}
		}
	}

	if holdReason != "" {
		return saveHoldLog(event, event.Trade.PositionType, holdReason)
	}

	return event, nil
}
