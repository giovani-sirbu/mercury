package tests

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestHold_StopLossHeldOnBearishClassicSignal blocks an exit when the
// classic cooldown indicator says the market is bearish but the engine is
// trying to take stopLoss. ShouldHold returns an error so the action chain
// short-circuits and the trade stays open.
func TestHold_StopLossHeldOnBearishClassicSignal(t *testing.T) {
	trade := scenarioBuildTrade("stopLoss", 95000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.001")
	event.Params.OldPosition = "active"
	event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{MarketBearish: true}

	_, err := actions.ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error on bearish classic signal for spot stopLoss")
	}
}

// TestHold_TakeProfitHeldOnBullishClassicSignal mirrors the prior test:
// take-profit on a bullish market is held off so the trade can ride higher.
func TestHold_TakeProfitHeldOnBullishClassicSignal(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 102000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.001")
	event.Params.OldPosition = "active"
	event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{MarketBullish: true}

	_, err := actions.ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error on bullish classic signal for spot takeProfit")
	}
}

// TestHold_InverseStopLossHeldOnBullishSignal covers the inverse mirror:
// inverse stopLoss + bullish market = hold (don't cut while market is up).
func TestHold_InverseStopLossHeldOnBullishSignal(t *testing.T) {
	trade := scenarioBuildTrade("stopLoss", 105000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "100")
	event.Params.OldPosition = "active"
	event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{MarketBullish: true}

	_, err := actions.ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error on bullish classic signal for inverse stopLoss")
	}
}

// TestHold_AIHoldActionBlocksAnyClose proves the explicit AIAction=HOLD
// gate: regardless of position type or direction, an explicit HOLD signal
// short-circuits the action chain.
func TestHold_AIHoldActionBlocksAnyClose(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 102000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.001")
	event.Params.OldPosition = "active"
	event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIAction: actions.ActionHold}

	_, err := actions.ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error on explicit AI HOLD action")
	}
}

// TestHold_NewStatusBlocksOpposingAISignal verifies the "new status"
// special case: when the trade is about to enter and the AI signals the
// opposite direction strongly, the entry is blocked.
func TestHold_NewStatusBlocksOpposingAISignal(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)

	event := scenarioBuildEvent(trade, "USDC", "100")
	event.Params.OldPosition = "new"
	event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIAction: actions.ActionShort}

	_, err := actions.ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error on opposing AI signal for new spot trade")
	}
}

// TestHold_NewStatusAllowsAlignedAISignal covers the positive path: an
// AI signal aligned with the new trade direction lets the chain continue.
func TestHold_NewStatusAllowsAlignedAISignal(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)

	event := scenarioBuildEvent(trade, "USDC", "100")
	event.Params.OldPosition = "new"
	event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIAction: actions.ActionLong}

	if _, err := actions.ShouldHold(event); err != nil {
		t.Fatalf("expected aligned AI signal to allow new trade, got %v", err)
	}
}
