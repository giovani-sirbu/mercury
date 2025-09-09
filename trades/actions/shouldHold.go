package actions

import (
	"fmt"
	"log"

	"github.com/giovani-sirbu/mercury/events"
)

const (
	ActionHold  = "HOLD"
	ActionLong  = "LONG"
	ActionShort = "SHORT"
)

func ShouldHold(event events.Events) (events.Events, error) {
	cool := event.Params.CoolDownIndicators
	ai := event.Params.AIIndicators

	shouldHold := false
	holdReason := ""

	// ======================================================
	// ✅ 0) Special logic: If trade is "new", restrict HOLD
	// ======================================================
	if event.Params.OldPosition == "new" {
		if ai.UseAI {
			isBearishSignal := ai.AIMarketBearish || ai.AIAction == ActionShort
			isBullishSignal := ai.AIMarketBullish || ai.AIAction == ActionLong

			if (!event.Trade.Inverse && isBearishSignal) ||
				(event.Trade.Inverse && isBullishSignal) {

				shouldHold = true
				holdReason = "AI (new status): avoid entering due to clear opposing signal"

				log.Printf("🔒 NEW STATUS HOLD for %s position on %s: %s",
					event.Trade.PositionType, event.Trade.Symbol, holdReason)

				if len(ai.StayOutReasons) > 0 {
					log.Printf("📋 StayOutReasons: %v", ai.StayOutReasons)
					holdReason += fmt.Sprintf(" | StayOutReasons: %v", ai.StayOutReasons)
				}

				err := fmt.Errorf("Position %s was held (new status): %s",
					event.Trade.PositionType, holdReason)
				return SaveError(event, err)
			}
		}
		// ✅ Not strongly bearish/bullish — let the trade start
		return event, nil
	}

	// ============================================================
	// ✅ 1) AI Mode: If AI signals are enabled, use them instead
	// ============================================================
	if ai.UseAI {
		if event.Trade.Inverse {
			if event.Trade.PositionType == "buy" && ai.AIMarketBullish {
				shouldHold = true
				holdReason = "Inverse: AI says market is bullish"
			}
			if event.Trade.PositionType == "sell" && ai.AIMarketBearish {
				shouldHold = true
				holdReason = "Inverse: AI says market is bearish"
			}
		} else {
			if event.Trade.PositionType == "buy" && ai.AIMarketBearish {
				shouldHold = true
				holdReason = "AI: market is bearish"
			}
			if event.Trade.PositionType == "sell" && ai.AIMarketBullish {
				shouldHold = true
				holdReason = "AI: market is bullish"
			}
			if ai.AIAction == ActionHold {
				shouldHold = true
				holdReason = "AI: explicit HOLD recommendation"
			}
		}
	}

	// ================================================================
	// ✅ 2) Classic Cooldown: If AI is disabled, fallback to indicators
	// ================================================================
	if !ai.UseAI {
		if event.Trade.Inverse {
			if event.Trade.PositionType == "buy" && cool.MarketBullish {
				shouldHold = true
				holdReason = "Inverse: classic market is bullish"
			}
			if event.Trade.PositionType == "sell" && cool.MarketBearish {
				shouldHold = true
				holdReason = "Inverse: classic market is bearish"
			}
		} else {
			if event.Trade.PositionType == "buy" && cool.MarketBearish {
				shouldHold = true
				holdReason = "Classic: market is bearish"
			}
			if event.Trade.PositionType == "sell" && cool.MarketBullish {
				shouldHold = true
				holdReason = "Classic: market is bullish"
			}
		}
	}

	// ================================================================
	// ✅ 3) Final HOLD check: log it, attach reasons, and save error
	// ================================================================
	if shouldHold {
		log.Printf(
			"🔒 HOLD triggered for %s position on %s: %s",
			event.Trade.PositionType, event.Trade.Symbol, holdReason)

		if ai.UseAI && len(ai.StayOutReasons) > 0 {
			log.Printf("📋 StayOutReasons: %v", ai.StayOutReasons)
			holdReason += fmt.Sprintf(" | StayOutReasons: %v", ai.StayOutReasons)
		}

		err := fmt.Errorf("Position %s was held: %s",
			event.Trade.PositionType, holdReason)
		return SaveError(event, err)
	}

	return event, nil
}
