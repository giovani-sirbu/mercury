package actions

import (
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/quantities"
	"math"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/patterns"
	"github.com/giovani-sirbu/mercury/trades/gates/smarttakeloss"
)

// strategy3Params is strategy 3's configuration: the cooldown gates and the
// legacy ML gate, with patterns and the smart take loss off.
func strategy3Params() aggragates.StrategyParams {
	return aggragates.StrategyParams{
		Cooldown:      true,
		UseAI:         true,
		UsePatterns:   false,
		SmartTakeLoss: false,
	}
}

func strategy3Event(position string, fills int, ai aggragates.AIIndicators) events.Events {
	trade := testutil.NewHoldTrade(position, false)
	trade.Strategy.Params = strategy3Params()
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{Percentage: 2, Tolerance: 0.25, Depths: 8, MinDepths: 6},
	}
	trade.PositionPrice = 100
	for i := 0; i < fills; i++ {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: 1, Price: 100 - float64(i)*2, OrderId: int64(i + 1),
		})
	}
	old := "active"
	if position == "buy" && fills == 0 {
		old = "new"
	}
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: old, AIIndicators: ai},
	}
}

func lastHold(t *testing.T, event events.Events) (string, bool) {
	t.Helper()
	held, err := ShouldHold(event)
	if err == nil {
		return "", false
	}
	if len(held.Trade.Logs) == 0 {
		t.Fatalf("hold returned %v without a log row", err)
	}
	return held.Trade.Logs[len(held.Trade.Logs)-1].Message, true
}

// Under strategy 3's flags only the cooldown gates and the legacy AI veto read
// the first fill. With both clear — a first-fill verdict that allows the long,
// no sibling ladder keeping the wallet, and an AI verdict that does not refuse
// the long — a payload carrying every other family's signal still lets the
// first fill through: a long pattern verdict scored over HoldMinScore with its
// target ahead, a quiet slow-decline verdict with its sell band, and a bearish
// dynamic params block. The same payload holds the fill once the smart take
// loss flag is on, so its slow-decline reading is one that flag's gate acts
// on; and a cooldown verdict that refuses the long holds the fill at the tick
// price with the cooldown row alone. The next test holds it on the AI veto.
func TestShouldHoldStrategy3FirstFillReadsOnlyCooldownAndTheAIVeto(t *testing.T) {
	payload := aggragates.AIIndicators{
		AIMarketBullish:    true,
		PatternName:        "asc_triangle",
		PatternDisplayName: "ascending triangle",
		PatternDirection:   aggragates.SideLong,
		PatternScore:       patterns.HoldMinScore + 10,
		PatternLevel:       96,
		PatternLevelKind:   "resistance",
		PatternTakeProfit:  110,
		SmartTakeLoss: aggragates.SmartTakeLossIndicators{
			SlowDeclineExit:     true,
			SlowDeclineLegQuiet: true,
			SlowDeclineSellBand: 104,
		},
		DynamicParams: aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
	}
	allowed := aggragates.CoolDownIndicators{HasFirstFillVerdict: true, AllowLongEntry: true}

	entry := strategy3Event("buy", 0, payload)
	entry.Params.CoolDownIndicators = allowed
	if msg, held := lastHold(t, entry); held {
		t.Fatalf("with the cooldown gates and the AI veto clear the first fill must pass, got %q", msg)
	}

	withExit := strategy3Event("buy", 0, payload)
	withExit.Trade.Strategy.Params.SmartTakeLoss = true
	withExit.Params.CoolDownIndicators = allowed
	if msg, held := lastHold(t, withExit); !held || !strings.Contains(msg, smarttakeloss.SlowDeclineEntryHoldReason) {
		t.Fatalf("the smart take loss flag must hold the same payload on its slow-decline verdict, got held=%v msg=%q", held, msg)
	}

	expensive := strategy3Event("buy", 0, payload)
	expensive.Params.CoolDownIndicators = aggragates.CoolDownIndicators{HasFirstFillVerdict: true, AllowLongEntry: false}
	msg, held := lastHold(t, expensive)
	if !held || msg != "Hold entry: cooldown: trying to get a better entry price: reference 100.0000, enters above 102.0408 or below 97.7995 after a bounce" {
		t.Fatalf("expected the cooldown hold alone, got held=%v msg=%q", held, msg)
	}
}

// Without a cooldown verdict the legacy ML gate holds the first fill of a
// UseAI strategy on its own.
func TestShouldHoldStrategy3FallsBackToLegacyWithoutVerdict(t *testing.T) {
	entry := strategy3Event("buy", 0, aggragates.AIIndicators{AIMarketBearish: true})
	msg, held := lastHold(t, entry)
	if !held || !strings.Contains(msg, "AI market is bearish") {
		t.Fatalf("expected legacy bearish hold, got held=%v msg=%q", held, msg)
	}
}

// The close estimate simulates the NET quantity (gross minus the base-asset
// commission already taken) and charges the closing leg once: buy 1 @100
// with a 0.001 base fee, close @200 -> 0.999 * 200 - 100 - 0.1 = 99.7.
func TestHasProfitSimulatesNetQuantityAndOneClosingLeg(t *testing.T) {
	trade := aggragates.Trades{Symbol: "SOL/USDT", PositionPrice: 200}
	trade.History = []aggragates.TradesHistory{{
		Type: "BUY", Quantity: 1, Price: 100, OrderId: 1,
		Fees: []aggragates.TradesFees{{Asset: "SOL", Fee: 0.001}},
	}}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 3, MinNotional: 10, PriceFilter: 2}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0}}

	got, err := HasProfit(events.Events{Trade: trade})
	if err != nil {
		t.Fatalf("expected the close to clear min profit, got %v", err)
	}
	if math.Abs(got.Trade.Profit-99.7) > 1e-9 {
		t.Errorf("net profit = %v, want 99.7 (gross 99.8 minus one closing leg 0.1)", got.Trade.Profit)
	}

	quantity, side := quantities.SimulatedCloseQuantity(events.Events{Trade: trade})
	if side != "sell" || math.Abs(quantity-0.999) > 1e-9 {
		t.Errorf("SimulatedCloseQuantity = %v %s, want 0.999 sell", quantity, side)
	}
}
