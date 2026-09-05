package actions

import (
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The isolation matrix: one flag on at a time against a payload that could
// trigger EVERY gate, and exactly that flag's row appears. This is the
// contract the run-97/98 audit found broken (regime holds fired for any
// strategy that fetched the verdict for another flag's sake).

// ownershipTrade is a long, 4 fills deep on a 7-deep ladder: deep enough for
// the crash park (4), the 15m shock hold (3), the STL arming zone (7-3=4) and
// the profit-hold floor (4).
func ownershipTrade(position string, fills int) aggragates.Trades {
	trade := testutil.NewHoldTrade(position, false)
	trade.Strategy.Params = aggragates.StrategyParams{}
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{{Percentage: 2, Tolerance: 0.5, Depths: 7}}
	trade.PositionPrice = 100
	trade.History = nil
	for i := 0; i < fills; i++ {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: 1, Price: 100 - float64(i), OrderId: int64(i + 1),
		})
	}
	return trade
}

func ownershipEvent(trade aggragates.Trades, ai aggragates.AIIndicators, cool aggragates.CoolDownIndicators) events.Events {
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: ai, CoolDownIndicators: cool},
	}
}

// fullHoldPayload carries every signal that could hold a long stopLoss.
func fullHoldPayload() aggragates.AIIndicators {
	return aggragates.AIIndicators{
		HasRegimeVerdict:       true,
		EnterAllowed:           false,
		AddAllowed:             false,
		Regime:                 regime.ShockDown,
		Regimes:                map[string]string{"4h": regime.DownPersist, "1h": regime.DownPersist, "15m": regime.ShockDown},
		CrashActive:            true,
		CrashScore:             90,
		HasContinuationVerdict: true,
		DownContinuationRisk:   95,
		ReversalUpEvidence:     0,
		DailyNatrPct:           2,
		AIAction:               aggragates.ActionHold,
		AIMarketBearish:        true,
		PatternAction:          aggragates.ActionShort,
		PatternName:            "asc_triangle",
		PatternDisplayName:     "ascending triangle",
		PatternDirection:       "long",
		PatternScore:           71,
		PatternLevel:           96000,
		PatternLevelKind:       "resistance",
		PatternTakeProfit:      104500,
	}
}

func expensiveCooldown() aggragates.CoolDownIndicators {
	return aggragates.CoolDownIndicators{HasFirstFillVerdict: true, AllowLongEntry: false, AllowShortEntry: false}
}

var holdFamilyPrefixes = []string{"cooldown:", "regime:", "pattern:", "fibonacci:", "crash-guard:", "smart-take-loss:", "AI ", "Capitulation"}

func assertOnlyFamily(t *testing.T, logs []aggragates.TradesLogs, want string) {
	t.Helper()
	if want == "" {
		if len(logs) != 0 {
			t.Fatalf("expected no row, got %v", messages(logs))
		}
		return
	}
	if len(logs) != 1 {
		t.Fatalf("expected exactly one row, got %v", messages(logs))
	}
	if !strings.Contains(logs[0].Message, want) {
		t.Fatalf("expected a %q row, got %q", want, logs[0].Message)
	}
	for _, prefix := range holdFamilyPrefixes {
		if strings.Contains(want, prefix) {
			continue
		}
		if strings.Contains(logs[0].Message, prefix) {
			t.Fatalf("row %q leaks the %q family", logs[0].Message, prefix)
		}
	}
}

func TestShouldHoldOwnershipMatrixStopLoss(t *testing.T) {
	cases := []struct {
		name   string
		params aggragates.StrategyParams
		want   string
	}{
		{"nothing on", aggragates.StrategyParams{}, ""},
		{"cooldown is inert after the first fill", aggragates.StrategyParams{Cooldown: true}, ""},
		{"regimeHold", aggragates.StrategyParams{RegimeHold: true}, "regime: market in shock (15m shock-down, depth 4)"},
		{"crashGuard", aggragates.StrategyParams{CrashGuard: true}, "crash-guard: deep trade, no new capital during a flush"},
		{"smartTakeLoss", aggragates.StrategyParams{SmartTakeLoss: true}, "smart-take-loss: HTF continuation, no add"},
		{"useAI", aggragates.StrategyParams{UseAI: true}, "AI market is bearish"},
		{"usePatterns", aggragates.StrategyParams{UsePatterns: true}, "pattern: ascending triangle found (resistance 96000.0000), preventing stopLoss"},
		{"useForceTrailing", aggragates.StrategyParams{UseForceTrailing: true}, ""},
		{"powerLawQuantiles", aggragates.StrategyParams{PowerLawQuantiles: true}, ""},
	}
	for _, c := range cases {
		trade := ownershipTrade("stopLoss", 4)
		trade.Strategy.Params = c.params
		held, err := ShouldHold(ownershipEvent(trade, fullHoldPayload(), expensiveCooldown()))
		if (c.want != "") != (err != nil) {
			t.Fatalf("%s: held=%v want %q", c.name, err, c.want)
		}
		assertOnlyFamily(t, held.Trade.Logs, c.want)
	}
}

func TestShouldHoldOwnershipMatrixTakeProfit(t *testing.T) {
	ai := fullHoldPayload()
	ai.Regimes = map[string]string{"4h": "mixed", "1h": "mixed", "15m": regime.UpPersist}
	ai.Regime = regime.UpPersist
	ai.AIMarketBearish = false
	ai.AIMarketBullish = true

	cases := []struct {
		name   string
		params aggragates.StrategyParams
		want   string
	}{
		{"nothing on", aggragates.StrategyParams{}, ""},
		{"cooldown", aggragates.StrategyParams{Cooldown: true}, ""},
		{"regimeHold", aggragates.StrategyParams{RegimeHold: true}, "regime: rides the trend (15m uptrend-persist)"},
		{"crashGuard never holds an exit", aggragates.StrategyParams{CrashGuard: true}, ""},
		{"smartTakeLoss never holds an exit", aggragates.StrategyParams{SmartTakeLoss: true}, ""},
		{"useAI", aggragates.StrategyParams{UseAI: true}, "AI market is bullish"},
		{"usePatterns", aggragates.StrategyParams{UsePatterns: true}, "pattern: ascending triangle in play, riding to target 104500.0000"},
	}
	for _, c := range cases {
		trade := ownershipTrade("takeProfit", 4)
		trade.Strategy.Params = c.params
		held, err := ShouldHold(ownershipEvent(trade, ai, expensiveCooldown()))
		if (c.want != "") != (err != nil) {
			t.Fatalf("%s: held=%v want %q", c.name, err, c.want)
		}
		assertOnlyFamily(t, held.Trade.Logs, c.want)
	}
}

// With every flag off nothing holds, at any depth: there is no fill cap, the
// chain runs as the legacy engine ran it and only funds stop it.
func TestShouldHoldAllFlagsOffHoldsNothing(t *testing.T) {
	for _, fills := range []int{4, 7} {
		trade := ownershipTrade("stopLoss", fills)
		if held, err := ShouldHold(ownershipEvent(trade, fullHoldPayload(), expensiveCooldown())); err != nil {
			t.Fatalf("all flags off must hold nothing at depth %d, got %v", fills, messages(held.Trade.Logs))
		}
	}
}

// RegimeHold has no seat on the first fill, whatever the labels say; with
// cooldown on, the only row is the cooldown's.
func TestShouldHoldRegimeHoldNeverFiresOnEntry(t *testing.T) {
	labels := []string{regime.Shock, regime.ShockDown, regime.ShockUp, regime.DownPersist, regime.UpPersist}
	for _, inverse := range []bool{false, true} {
		for _, label := range labels {
			ai := fullHoldPayload()
			ai.Regimes = map[string]string{"4h": label, "1h": label, "15m": label}
			trade := ownershipTrade("buy", 0)
			trade.Inverse = inverse
			trade.Strategy.Params = aggragates.StrategyParams{RegimeHold: true}
			event := ownershipEvent(trade, ai, expensiveCooldown())
			event.Params.OldPosition = "new"
			held, err := ShouldHold(event)
			if err != nil || len(held.Trade.Logs) != 0 {
				t.Fatalf("inverse=%v %s: RegimeHold must not touch the first fill, got %v %v", inverse, label, err, messages(held.Trade.Logs))
			}

			trade.Strategy.Params = aggragates.StrategyParams{RegimeHold: true, Cooldown: true}
			event = ownershipEvent(trade, ai, expensiveCooldown())
			event.Params.OldPosition = "new"
			held, err = ShouldHold(event)
			if err == nil {
				t.Fatalf("inverse=%v %s: the cooldown must hold the refused first fill", inverse, label)
			}
			assertOnlyFamily(t, held.Trade.Logs, "cooldown: trying to get a better entry price")
		}
	}
}

// A crash-guard-only strategy receives the regime block on the wire and
// must never write a regime row.
func TestShouldHoldCrashGuardOnlyProducesNoRegimeRows(t *testing.T) {
	shallow := ownershipTrade("stopLoss", 2)
	shallow.Strategy.Params = aggragates.StrategyParams{CrashGuard: true}
	if held, err := ShouldHold(ownershipEvent(shallow, fullHoldPayload(), expensiveCooldown())); err != nil {
		t.Fatalf("crashGuard alone must not hold a shallow add, got %v", messages(held.Trade.Logs))
	}
	deep := ownershipTrade("stopLoss", 4)
	deep.Strategy.Params = aggragates.StrategyParams{CrashGuard: true}
	held, err := ShouldHold(ownershipEvent(deep, fullHoldPayload(), expensiveCooldown()))
	if err == nil {
		t.Fatal("crashGuard must park the deep add during a flush")
	}
	for _, row := range held.Trade.Logs {
		if strings.Contains(row.Message, "regime:") {
			t.Fatalf("a crash-guard-only strategy wrote a regime row: %q", row.Message)
		}
	}
	assertOnlyFamily(t, held.Trade.Logs, "crash-guard: deep trade, no new capital during a flush")
}

// Capitulation is the crash guard's: without CrashGuard the shock hold
// stands and no episode row is written.
func TestShouldHoldCapitulationRequiresCrashGuard(t *testing.T) {
	trade := capTrade(false, 3, 100, 79)
	trade.Strategy.Params = aggragates.StrategyParams{RegimeHold: true}
	held, err := ShouldHold(capEvent(trade, capShockAI(false), capReclaimBucket(false)))
	if err == nil {
		t.Fatal("without CrashGuard the shock hold must stand")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "regime: market in shock") {
		t.Fatalf("unexpected hold %q", held.Trade.Logs[0].Message)
	}
	for _, prefix := range []string{crashguard.CapitulationTaggedPrefix, crashguard.CapitulationAllowedPrefix, crashguard.CapitulationFreezeOnPrefix} {
		if hasCapitulationPrefix(held.Trade.Logs, prefix) {
			t.Fatalf("capitulation wrote %q without CrashGuard", prefix)
		}
	}
}

// Capitulation can only bypass a regime hold: with CrashGuard alone there is
// nothing to bypass and nothing is written.
func TestShouldHoldCapitulationNeedsARegimeHold(t *testing.T) {
	trade := capTrade(false, 3, 100, 79)
	trade.Strategy.Params = aggragates.StrategyParams{CrashGuard: true}
	held, err := ShouldHold(capEvent(trade, capShockAI(false), capReclaimBucket(false)))
	if err != nil {
		t.Fatalf("CrashGuard alone on a shallow reclaim must hold nothing, got %v", err)
	}
	if len(held.Trade.Logs) != 0 {
		t.Fatalf("no row may be written, got %v", messages(held.Trade.Logs))
	}
}

// A force-trailing re-anchor reads as the rung it re-arms: the gates have
// power on it and the row keeps the raw position name.
func TestShouldHoldForceTrailingStatesRunTheGates(t *testing.T) {
	sl := ownershipTrade("forceTrailingStopLoss", 4)
	sl.Strategy.Params = aggragates.StrategyParams{RegimeHold: true}
	held, err := ShouldHold(ownershipEvent(sl, fullHoldPayload(), expensiveCooldown()))
	if err == nil || held.Trade.Logs[0].Message != "Hold forceTrailingStopLoss: regime: market in shock (15m shock-down, depth 4)" {
		t.Fatalf("regimeHold must gate a force-trailing stopLoss, got %v %v", err, messages(held.Trade.Logs))
	}

	sl = ownershipTrade("forceTrailingStopLoss", 4)
	sl.Strategy.Params = aggragates.StrategyParams{CrashGuard: true}
	held, err = ShouldHold(ownershipEvent(sl, fullHoldPayload(), expensiveCooldown()))
	if err == nil || !strings.Contains(held.Trade.Logs[0].Message, "crash-guard: deep trade") {
		t.Fatalf("crashGuard must gate a force-trailing stopLoss, got %v %v", err, messages(held.Trade.Logs))
	}

	ai := fullHoldPayload()
	ai.Regimes = map[string]string{"4h": "mixed", "1h": "mixed", "15m": regime.UpPersist}
	tp := ownershipTrade("forceTrailingTakeProfit", 4)
	tp.Strategy.Params = aggragates.StrategyParams{RegimeHold: true}
	held, err = ShouldHold(ownershipEvent(tp, ai, expensiveCooldown()))
	if err == nil || held.Trade.Logs[0].Message != "Hold forceTrailingTakeProfit: regime: rides the trend (15m uptrend-persist)" {
		t.Fatalf("regimeHold must gate a force-trailing takeProfit, got %v %v", err, messages(held.Trade.Logs))
	}

	tp = ownershipTrade("forceTrailingTakeProfit", 4)
	if held, err := ShouldHold(ownershipEvent(tp, ai, expensiveCooldown())); err != nil {
		t.Fatalf("all flags off must not hold a force-trailing takeProfit, got %v", messages(held.Trade.Logs))
	}
}
