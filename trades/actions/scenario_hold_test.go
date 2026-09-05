package actions_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestHold_StopLossNoLongerHeldOnClassicSignal: cooldown is first-fill
// only. A rebuy must proceed even when markers look expensive.
func TestHold_StopLossNoLongerHeldOnClassicSignal(t *testing.T) {
	trade := scenarioBuildTrade("stopLoss", 95000, false)
	trade.Strategy.Params.Cooldown = true
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.001")
	event.Params.OldPosition = "active"
	event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{
		HasFirstFillVerdict: true,
		AllowLongEntry:      false,
		MarketBearish:       true,
	}

	if _, err := actions.ShouldHold(event); err != nil {
		t.Fatalf("cooldown must not hold a rebuy, got %v", err)
	}
}

// TestHold_FirstFillExpensiveBlocksEntry parks the initial long buy when
// sophos says the 1h print is stretched and 15m has not turned.
func TestHold_FirstFillExpensiveBlocksEntry(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	trade.Strategy.Params.Cooldown = true

	event := scenarioBuildEvent(trade, "USDC", "100")
	event.Params.OldPosition = "new"
	event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{
		HasFirstFillVerdict: true,
		AllowLongEntry:      false,
	}

	if _, err := actions.ShouldHold(event); err == nil {
		t.Fatal("expected hold on an expensive first long fill")
	}
}

// TestHold_InverseFirstFillExpensiveBlocksEntry is the short-side mirror.
func TestHold_InverseFirstFillExpensiveBlocksEntry(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, true)
	trade.Strategy.Params.Cooldown = true

	event := scenarioBuildEvent(trade, "USDC", "100")
	event.Params.OldPosition = "new"
	event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{
		HasFirstFillVerdict: true,
		AllowShortEntry:     false,
	}

	if _, err := actions.ShouldHold(event); err == nil {
		t.Fatal("expected hold on an expensive first inverse fill")
	}
}

// TestHold_AIHoldActionBlocksRebuyAllowsTakeProfit proves the explicit
// AIAction=HOLD gate: HOLD means "no direction", so risk-adding positions
// (stopLoss rebuys) are short-circuited while the profitable close
// (takeProfit) is allowed to execute, for spot and inverse alike.
func TestHold_AIHoldActionBlocksRebuyAllowsTakeProfit(t *testing.T) {
	stopLoss := scenarioBuildTrade("stopLoss", 95000, false)
	stopLoss.Strategy.Params.UseAI = true
	scenarioAppendHistory(&stopLoss, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(stopLoss, "BTC", "0.001")
	event.Params.OldPosition = "active"
	event.Params.AIIndicators = aggragates.AIIndicators{AIAction: aggragates.ActionHold}

	if _, err := actions.ShouldHold(event); err == nil {
		t.Fatal("expected hold error on explicit AI HOLD action for stopLoss rebuy")
	}

	takeProfit := scenarioBuildTrade("takeProfit", 102000, false)
	takeProfit.Strategy.Params.UseAI = true
	scenarioAppendHistory(&takeProfit, "BUY", 0.001, 100000, "", 0)

	tpEvent := scenarioBuildEvent(takeProfit, "BTC", "0.001")
	tpEvent.Params.OldPosition = "active"
	tpEvent.Params.AIIndicators = aggragates.AIIndicators{AIAction: aggragates.ActionHold}

	if _, err := actions.ShouldHold(tpEvent); err != nil {
		t.Fatalf("expected explicit AI HOLD to allow takeProfit close, got %v", err)
	}

	inverseTP := scenarioBuildTrade("takeProfit", 95000, true)
	inverseTP.Strategy.Params.UseAI = true
	scenarioAppendHistory(&inverseTP, "SELL", 0.001, 100000, "", 0)

	invEvent := scenarioBuildEvent(inverseTP, "USDC", "100")
	invEvent.Params.OldPosition = "active"
	invEvent.Params.AIIndicators = aggragates.AIIndicators{AIAction: aggragates.ActionHold}

	if _, err := actions.ShouldHold(invEvent); err != nil {
		t.Fatalf("expected explicit AI HOLD to allow inverse takeProfit close, got %v", err)
	}
}

// TestHold_NewStatusBlocksOpposingAISignal verifies the "new status"
// special case: when the trade is about to enter and the AI signals the
// opposite direction strongly, the entry is blocked.
func TestHold_NewStatusBlocksOpposingAISignal(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	trade.Strategy.Params.UseAI = true

	event := scenarioBuildEvent(trade, "USDC", "100")
	event.Params.OldPosition = "new"
	event.Params.AIIndicators = aggragates.AIIndicators{AIAction: aggragates.ActionShort}

	_, err := actions.ShouldHold(event)
	if err == nil {
		t.Fatal("expected hold error on opposing AI signal for new spot trade")
	}
}

// TestHold_NewStatusAllowsAlignedAISignal covers the positive path: an
// AI signal aligned with the new trade direction lets the chain continue.
func TestHold_NewStatusAllowsAlignedAISignal(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	trade.Strategy.Params.UseAI = true

	event := scenarioBuildEvent(trade, "USDC", "100")
	event.Params.OldPosition = "new"
	event.Params.AIIndicators = aggragates.AIIndicators{AIAction: aggragates.ActionLong}

	if _, err := actions.ShouldHold(event); err != nil {
		t.Fatalf("expected aligned AI signal to allow new trade, got %v", err)
	}
}
