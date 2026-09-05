package actions

import (
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/quantities"
	"math"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// strategy3Params is the configuration runs 90/93/94 actually ran, plus
// RegimeHold: after the ownership refactor the add/exit gates those runs
// exercised answer to that flag and to nothing else.
func strategy3Params() aggragates.StrategyParams {
	return aggragates.StrategyParams{
		Cooldown:      true,
		UseAI:         true,
		UsePatterns:   false,
		CrashGuard:    true,
		SmartTakeLoss: false,
		RegimeHold:    true,
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

// regimeVerdict builds the served set with the long add-veto pair (4h+2h);
// the 1h lens is fetched only for the crash detector and never names an add
// veto, so it is pinned to "mixed" here.
func regimeVerdict(fourHour, twoHour, fifteen string, addAllowed bool) aggragates.AIIndicators {
	return aggragates.AIIndicators{
		HasRegimeVerdict: true,
		AddAllowed:       addAllowed,
		EnterAllowed:     fourHour != regime.DownPersist,
		Regimes:          map[string]string{"4h": fourHour, "2h": twoHour, "1h": "mixed", "15m": fifteen},
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

// The whole regime hold family on an open position must be reachable for
// strategy 3 through RegimeHold, with usePatterns OFF.
func TestShouldHoldStrategy3RegimeGatesAreLive(t *testing.T) {
	cases := []struct {
		name     string
		event    events.Events
		wantHold string
	}{
		{
			name:     "add veto when sophos refuses the add",
			event:    strategy3Event("stopLoss", 2, regimeVerdict("mixed", regime.DownPersist, "mixed", false)),
			wantHold: "regime: add not allowed (2h downtrend-persist)",
		},
		{
			name:     "depth-aware 15m shock hold",
			event:    strategy3Event("stopLoss", regime.ShockHoldMinDepth, regimeVerdict("mixed", "mixed", regime.ShockDown, true)),
			wantHold: "regime: market in shock (15m shock-down, depth 3)",
		},
		{
			name:     "legacy ML HOLD still gates the add on top of the regime lens",
			event:    withAIAction(strategy3Event("stopLoss", 2, regimeVerdict("mixed", "mixed", "mixed", true)), aggragates.ActionHold),
			wantHold: "AI recommends HOLD",
		},
	}
	for _, c := range cases {
		msg, held := lastHold(t, c.event)
		if !held {
			t.Fatalf("%s: expected a hold", c.name)
		}
		if !strings.Contains(msg, c.wantHold) {
			t.Errorf("%s: hold %q does not contain %q", c.name, msg, c.wantHold)
		}
	}

	// And the gates must let the chain through when the verdict agrees.
	clear := strategy3Event("stopLoss", 2, regimeVerdict("mixed", "mixed", "mixed", true))
	if _, held := lastHold(t, clear); held {
		t.Fatal("a permissive verdict must not hold the add")
	}
	entry := strategy3Event("buy", 0, regimeVerdict(regime.UpPersist, "mixed", "mixed", true))
	if _, held := lastHold(t, entry); held {
		t.Fatal("a 4h uptrend must not veto the first fill")
	}
}

// The first fill answers to the cooldown lens only: a 4h downtrend with no
// cooldown verdict passes, and an expensive first fill holds without a word
// of regime in the row.
func TestShouldHoldStrategy3EntryIsCooldownOnly(t *testing.T) {
	entry := strategy3Event("buy", 0, regimeVerdict(regime.DownPersist, regime.DownPersist, regime.ShockDown, false))
	if msg, held := lastHold(t, entry); held {
		t.Fatalf("a 4h downtrend must not veto the first fill any more, got %q", msg)
	}

	expensive := strategy3Event("buy", 0, regimeVerdict(regime.DownPersist, "mixed", "mixed", false))
	expensive.Params.CoolDownIndicators = aggragates.CoolDownIndicators{HasFirstFillVerdict: true, AllowLongEntry: false}
	msg, held := lastHold(t, expensive)
	if !held || msg != "Hold entry: cooldown: first fill expensive" {
		t.Fatalf("expected the cooldown hold alone, got held=%v msg=%q", held, msg)
	}
	if strings.Contains(msg, "regime") {
		t.Fatalf("no regime text may appear on a first fill, got %q", msg)
	}
}

func withAIAction(event events.Events, action string) events.Events {
	event.Params.AIIndicators.AIAction = action
	return event
}

// Without a verdict (older sophos, failed pattern leg) the legacy ML gate is
// the only entry gate for a UseAI strategy — no regime strings may appear.
func TestShouldHoldStrategy3FallsBackToLegacyWithoutVerdict(t *testing.T) {
	entry := strategy3Event("buy", 0, aggragates.AIIndicators{AIMarketBearish: true})
	msg, held := lastHold(t, entry)
	if !held || !strings.Contains(msg, "AI market is bearish") || strings.Contains(msg, "regime") {
		t.Fatalf("expected legacy bearish hold, got held=%v msg=%q", held, msg)
	}
}

// A previously-armed deep trade must not stay parked forever when the regime
// verdict is missing (sophos outage): the sticky hold degrades open like
// every other AI gate.
func TestShouldHoldCrashStickyFailsOpenWithoutVerdict(t *testing.T) {
	event := events.Events{
		Trade: testutil.DeepTrade(true),
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: aggragates.AIIndicators{}},
	}
	event.Trade.Logs = []aggragates.TradesLogs{{
		Message: crashguard.TransitionMessage(aggragates.AIIndicators{CrashActive: true, CrashScore: 66}),
	}}
	if _, err := ShouldHold(event); err != nil {
		t.Fatalf("missing verdict must not keep the sticky park, got %v", err)
	}
}

// A capitulation tag whose reclaim never fired granted no add: a later
// gate-approved fill must not freeze the ladder, and a tick the gates already
// approved is never turned into a hold by the override.
func TestShouldHoldCapitulationTaggedWithoutGrantNeverInventsHold(t *testing.T) {
	falling := capEvent(capTrade(false, 3, 100, 79), capShockAI(false), capFallingBucket())
	held, err := ShouldHold(falling)
	if err == nil {
		t.Fatal("falling 5m bar must keep the shock hold")
	}
	if !hasCapitulationPrefix(held.Trade.Logs, crashguard.CapitulationTaggedPrefix) || hasCapitulationPrefix(held.Trade.Logs, crashguard.CapitulationAllowedPrefix) {
		t.Fatalf("expected a tagged-but-not-allowed episode, got %#v", messages(held.Trade.Logs))
	}

	// A regime-approved add fills at depth 4 later on (no hold to bypass).
	later := capTrade(false, 4, 79, 78)
	later.Logs = held.Trade.Logs
	approved := capEvent(later, regimeVerdict("mixed", "mixed", "mixed", true), capReclaimBucket(false))
	if _, err := ShouldHold(approved); err != nil {
		t.Fatalf("a gate-approved add must never be frozen by a stale tag, got %v", err)
	}
	if hasCapitulationPrefix(approved.Trade.Logs, crashguard.CapitulationFreezeOnPrefix) {
		t.Fatal("no freeze row may be written without a granted capitulation add")
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
