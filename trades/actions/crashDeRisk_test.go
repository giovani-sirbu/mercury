package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// deepTrade holds five distinct filled entries at 100/98/96/94/92, one unit
// each: 480 quote invested, average entry 96.
func deepTrade(takeLoss bool, takeLossPercentage float64) aggragates.Trades {
	trade := aggragates.Trades{
		Symbol:        "BTC/USDT",
		PositionType:  "stopLoss",
		PositionPrice: 92,
		Strategy: aggragates.Strategies{
			Params: aggragates.StrategyParams{TakeLoss: takeLoss},
		},
		StrategyPair: aggragates.StrategiesPairs{
			StrategySettings: []aggragates.StrategySettings{
				{Percentage: 2, Tolerance: 0.5, TakeLossPercentage: takeLossPercentage},
			},
		},
	}
	prices := []float64{100, 98, 96, 94, 92}
	for i, price := range prices {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: 1, Price: price, OrderId: int64(i + 1),
		})
	}
	return trade
}

func TestCrashDeRiskSellsDeepTradeAtProfit(t *testing.T) {
	position, ok := CrashDeRiskPosition(deepTrade(false, 0), 97)
	if !ok || position != "sellLoss" {
		t.Fatalf("deep profitable trade must de-risk, got %q/%v", position, ok)
	}
}

func TestCrashDeRiskSellsBoundedLossOnlyWithTakeLoss(t *testing.T) {
	// At 90 the estimate is 450-480 = -30 quote on 480 invested: -6.25%.
	if _, ok := CrashDeRiskPosition(deepTrade(false, 10), 90); ok {
		t.Fatal("loss sale without TakeLoss must not de-risk")
	}
	position, ok := CrashDeRiskPosition(deepTrade(true, 10), 90)
	if !ok || position != "sellLoss" {
		t.Fatalf("bounded loss with TakeLoss must de-risk, got %q/%v", position, ok)
	}
	if _, ok := CrashDeRiskPosition(deepTrade(true, 5), 90); ok {
		t.Fatal("loss beyond TakeLossPercentage must not de-risk")
	}
}

func TestCrashDeRiskIgnoresShallowAndInverseTrades(t *testing.T) {
	shallow := deepTrade(true, 10)
	shallow.History = shallow.History[:3]
	if _, ok := CrashDeRiskPosition(shallow, 97); ok {
		t.Fatal("a three-entry trade is not deep")
	}

	inverse := deepTrade(true, 10)
	inverse.Inverse = true
	if _, ok := CrashDeRiskPosition(inverse, 97); ok {
		t.Fatal("inverse trades never de-risk — the flush moves in their favor")
	}
}

func TestShouldHoldCrashParksDeepRebuyAndFreesProfitExit(t *testing.T) {
	crash := aggragates.AIIndicators{
		UseAI:            true,
		HasRegimeVerdict: true,
		AddAllowed:       true, // regime alone would allow the add
		CrashActive:      true,
		CrashScore:       85,
	}

	rebuy := events.Events{
		Trade: deepTrade(true, 10),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForHold,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: crash},
	}
	if _, err := ShouldHold(rebuy); err == nil {
		t.Fatal("deep rebuy must be parked while the flush signal is armed")
	}

	profitExit := events.Events{
		Trade:  deepTrade(true, 10),
		Params: aggragates.Params{OldPosition: "active", AIIndicators: crash},
	}
	profitExit.Trade.PositionType = "takeProfit"
	if _, err := ShouldHold(profitExit); err != nil {
		t.Fatalf("crash must call off KEEP_TRAILING, got %v", err)
	}
}
