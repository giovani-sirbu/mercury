package actions

import (
	"fmt"
	"log"

	"github.com/giovani-sirbu/mercury/events"
)

// ======================================
// ✅ Constants for AI & trade actions
// ======================================

const (
	// Minimum signal strength below which AI will trigger a HOLD
	MinSignalStrength = 25.0

	// Trading actions
	ActionHold  = "HOLD"
	ActionLong  = "LONG"
	ActionShort = "SHORT"
)

// ShouldHold determines whether a trade should be held (not executed)
// based on cooldown indicators or AI indicators.
// It returns the (possibly modified) event and an error if a HOLD happens.
func ShouldHold(event events.Events) (events.Events, error) {
	// Extract indicators from the event Params
	cool := event.Params.CoolDownIndicators
	ai := event.Params.AIIndicators

	shouldHold := false // Flag to decide if we should hold the position
	holdReason := ""    // Explanation for the hold decision

	// ============================================================
	// ✅ 1) AI Mode: If AI signals are enabled, use them instead
	// ============================================================
	if ai.UseAI {
		// If current position is LONG but AI says market is bearish → hold
		if event.Trade.PositionType == "buy" && ai.AIMarketBearish {
			shouldHold = true
			holdReason = "AI: market is bearish"
		}

		// If current position is SHORT but AI says market is bullish → hold
		if event.Trade.PositionType == "sell" && ai.AIMarketBullish {
			shouldHold = true
			holdReason = "AI: market is bullish"
		}

		// If AI explicitly recommends HOLD
		if ai.AIAction == ActionHold {
			shouldHold = true
			holdReason = "AI: explicit HOLD recommendation"
		}

		// If AI signal strength is too weak → hold
		if ai.AISignalStrength < MinSignalStrength {
			shouldHold = true
			holdReason = fmt.Sprintf(
				"AI: signal strength %.2f below minimum %.2f",
				ai.AISignalStrength, MinSignalStrength)
		}

		// Apply inverse logic if strategy uses inverse positions
		if event.Trade.Inverse {
			if event.Trade.PositionType == "buy" && ai.AIMarketBullish {
				shouldHold = true
				holdReason = "Inverse: AI says market is bullish"
			}
			if event.Trade.PositionType == "sell" && ai.AIMarketBearish {
				shouldHold = true
				holdReason = "Inverse: AI says market is bearish"
			}
		}
	}

	// ================================================================
	// ✅ 2) Classic Cooldown: If AI is disabled, fallback to indicators
	// ================================================================
	if !ai.UseAI {
		// LONG position but market is bearish → hold
		if event.Trade.PositionType == "buy" && cool.MarketBearish {
			shouldHold = true
			holdReason = "Classic: market is bearish"
		}

		// SHORT position but market is bullish → hold
		if event.Trade.PositionType == "sell" && cool.MarketBullish {
			shouldHold = true
			holdReason = "Classic: market is bullish"
		}

		// Apply inverse logic if strategy uses inverse positions
		if event.Trade.Inverse {
			if event.Trade.PositionType == "buy" && cool.MarketBullish {
				shouldHold = true
				holdReason = "Inverse: classic market is bullish"
			}
			if event.Trade.PositionType == "sell" && cool.MarketBearish {
				shouldHold = true
				holdReason = "Inverse: classic market is bearish"
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

		// If AI was used and there are StayOutReasons → include them in log
		if ai.UseAI && len(ai.StayOutReasons) > 0 {
			log.Printf("📋 StayOutReasons: %v", ai.StayOutReasons)
			holdReason += fmt.Sprintf(" | StayOutReasons: %v", ai.StayOutReasons)
		}

		// Save error to persist the reason for holding
		err := fmt.Errorf("Position %s was held: %s",
			event.Trade.PositionType, holdReason)
		return SaveError(event, err)
	}

	// ✅ No hold — return the event as-is
	return event, nil
}
