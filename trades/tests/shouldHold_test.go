package tests

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestShouldHold(t *testing.T) {
	t.Run("NoHold_NoIndicators", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "stopLoss"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		_, err := actions.ShouldHold(event)
		AssertNoError(t, err)
	})

	t.Run("AI_NewTrade_BearishHolds_Normal", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "buy"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "new"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIMarketBearish: true}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})

	t.Run("AI_NewTrade_BullishHolds_Inverse", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, true, nil)
		trade.PositionType = "buy"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "new"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIMarketBullish: true}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})

	t.Run("AI_NewTrade_NoOpposingSignal", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "buy"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "new"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIMarketBullish: true}
		_, err := actions.ShouldHold(event)
		AssertNoError(t, err)
	})

	t.Run("AI_StopLoss_BearishHolds", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "stopLoss"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIMarketBearish: true}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})

	t.Run("AI_TakeProfit_BullishHolds", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "takeProfit"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIMarketBullish: true}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})

	t.Run("AI_ExplicitHold", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "stopLoss"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIAction: "HOLD"}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})

	t.Run("AI_ExplicitHold_TakeProfitPasses", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "takeProfit"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIAction: "HOLD"}
		_, err := actions.ShouldHold(event)
		AssertNoError(t, err)
	})

	t.Run("AI_ExplicitHold_TakeProfitPasses_Inverse", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, true, nil)
		trade.PositionType = "takeProfit"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIAction: "HOLD"}
		_, err := actions.ShouldHold(event)
		AssertNoError(t, err)
	})

	t.Run("AI_ExplicitHold_InverseStopLossHolds", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, true, nil)
		trade.PositionType = "stopLoss"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIAction: "HOLD"}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})

	t.Run("Classic_StopLoss_BearishHolds", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "stopLoss"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{MarketBearish: true}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})

	t.Run("Classic_TakeProfit_BullishHolds", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "takeProfit"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{MarketBullish: true}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})

	t.Run("Classic_NoHold", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PositionType = "stopLoss"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{MarketBullish: true}
		_, err := actions.ShouldHold(event)
		AssertNoError(t, err)
	})

	t.Run("Inverse_StopLoss_BullishHolds_AI", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, true, nil)
		trade.PositionType = "stopLoss"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.AIIndicators = aggragates.AIIndicators{UseAI: true, AIMarketBullish: true}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})

	t.Run("Inverse_TakeProfit_BearishHolds_Classic", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, true, nil)
		trade.PositionType = "takeProfit"
		event := MakeEvent(trade, "USDT", "1000", []string{"shouldHold"})
		event.Params.OldPosition = "active"
		event.Params.CoolDownIndicators = aggragates.CoolDownIndicators{MarketBearish: true}
		_, err := actions.ShouldHold(event)
		AssertError(t, err)
	})
}
