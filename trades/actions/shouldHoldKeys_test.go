package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// Every hold ShouldHold writes is keyed by the gate that spoke (an aggragates.LogKeyHold
// constant), its message still "Hold <position>: <reason>": the depth priority, the first fill
// and its verdict, the depth spacing, the smartSell entry, the AI on the entry and on a position,
// the chart pattern and the fibonacci wait.
func TestEveryHoldRowCarriesTheKeyOfItsGate(t *testing.T) {
	trades := reserveGridTrades()
	view := reserveGridView(trades)
	keeper := reserveGridKeeper(t, view)
	waiting := reserveGridTradeOf(t, trades, reserveGridShape[1].symbol)
	_, ownCost := ladder.NextEntryCost(waiting, keeper.RemainingCost)
	waiting.PositionType = "trailingBuy"

	firstFill := ownershipTrade("buy", 0)
	firstFill.Strategy.Params = aggragates.StrategyParams{Cooldown: true}
	firstFillEvent := ownershipEvent(firstFill, fullHoldPayload(), expensiveCooldown())
	firstFillEvent.Params.OldPosition = "new"

	futures := firstFill
	futures.Strategy.TradeType = aggragates.Futures
	verdict := fullHoldPayload()
	verdict.AIAction = aggragates.ActionLong
	futuresEvent := ownershipEvent(futures, verdict, expensiveCooldown())
	futuresEvent.Params.OldPosition = "new"

	aiEntry := ownershipTrade("buy", 0)
	aiEntry.Strategy.Params = aggragates.StrategyParams{UseAI: true}
	aiEntryEvent := ownershipEvent(aiEntry, fullHoldPayload(), expensiveCooldown())
	aiEntryEvent.Params.OldPosition = "new"

	aiPosition := ownershipTrade("trailingBuy", 4)
	aiPosition.Strategy.Params = aggragates.StrategyParams{UseAI: true}
	pattern := ownershipTrade("trailingBuy", 4)
	pattern.Strategy.Params = aggragates.StrategyParams{UsePatterns: true}

	for _, c := range []struct {
		name  string
		event events.Events
		key   string
	}{
		{"the depth priority", priorityEvent(waiting, "buy", view, keeper.RemainingCost+ownCost-1), aggragates.LogKeyHoldDepthPriority},
		{"the first fill", firstFillEvent, aggragates.LogKeyHoldFirstFill},
		{"the first fill's verdict on futures", futuresEvent, aggragates.LogKeyHoldFirstFillVerdict},
		{"the depth spacing", depthEvent(testutil.DepthTrade(trade25858[0]), testutil.At("13:48:33")), aggragates.LogKeyHoldDepthSpacing},
		{"the smartSell entry", stlEntryEvent(aggragates.StrategyParams{SmartSell: true}, slowDeclinePayload(true)), aggragates.LogKeyHoldSmartSellEntry},
		{"the AI on the entry", aiEntryEvent, aggragates.LogKeyHoldAI},
		{"the AI on a position", ownershipEvent(aiPosition, fullHoldPayload(), expensiveCooldown()), aggragates.LogKeyHoldAI},
		{"the chart pattern", ownershipEvent(pattern, fullHoldPayload(), expensiveCooldown()), aggragates.LogKeyHoldPattern},
		{"the fibonacci wait", patternEvent("trailingBuy", false, 106.5, testutil.FibAI()), aggragates.LogKeyHoldFibonacci},
	} {
		held, err := ShouldHold(c.event)
		if err == nil || len(held.Trade.Logs) == 0 {
			t.Fatalf("%s: fixture drifted: the gate must hold, got %v %v", c.name, err, messages(held.Trade.Logs))
		}
		row := held.Trade.Logs[len(held.Trade.Logs)-1]
		if row.MessageKey != c.key || row.Message[:5] != "Hold " {
			t.Errorf("%s: the row %q is keyed %q, want %q", c.name, row.Message, row.MessageKey, c.key)
		}
	}
}
