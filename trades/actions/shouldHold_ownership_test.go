package actions

import (
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The isolation matrix: one flag on at a time against a payload that could
// trigger EVERY gate, and exactly that flag's row appears. A gate answers to
// its own flag, never to a payload fetched for another flag's sake.

// ownershipTrade is a long ladder with `fills` filled entries, part-way down
// its depths.
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

// fullHoldPayload carries every signal that could hold a long stopLoss, the
// smart take loss's slow-decline verdict, which holds a first fill only, and
// both dynamic params reads bearish, which hold nothing at all.
func fullHoldPayload() aggragates.AIIndicators {
	return aggragates.AIIndicators{
		SmartTakeLoss:      aggragates.SmartTakeLossIndicators{SlowDeclineExit: true},
		DynamicParams:      aggragates.DynamicParamsIndicators{Timeframe: "1D", Guppy: -1, BMSB: -1, Valid: true},
		AIAction:           aggragates.ActionHold,
		AIMarketBearish:    true,
		PatternAction:      aggragates.ActionShort,
		PatternName:        "asc_triangle",
		PatternDisplayName: "ascending triangle",
		PatternDirection:   "long",
		PatternScore:       71,
		PatternLevel:       96000,
		PatternLevelKind:   "resistance",
		PatternTakeProfit:  104500,
	}
}

func expensiveCooldown() aggragates.CoolDownIndicators {
	return aggragates.CoolDownIndicators{HasFirstFillVerdict: true, AllowLongEntry: false, AllowShortEntry: false}
}

var holdFamilyPrefixes = []string{"cooldown:", "pattern:", "fibonacci:", "smartTakeLoss:", "AI ", dynamicparams.RowPrefix}

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
		{"smartTakeLoss holds no open position, verdict or not: it forces exits outside ShouldHold", aggragates.StrategyParams{SmartTakeLoss: true}, ""},
		{"useAI", aggragates.StrategyParams{UseAI: true}, "AI market is bearish"},
		{"usePatterns", aggragates.StrategyParams{UsePatterns: true}, "pattern: ascending triangle found (resistance 96000.0000), preventing stopLoss"},
		{"useForceTrailing", aggragates.StrategyParams{UseForceTrailing: true}, ""},
		{"dynamicParams shapes the rows and holds no add, both reads bearish", aggragates.StrategyParams{DynamicParams: true}, ""},
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
	ai.AIMarketBearish = false
	ai.AIMarketBullish = true

	cases := []struct {
		name   string
		params aggragates.StrategyParams
		want   string
	}{
		{"nothing on", aggragates.StrategyParams{}, ""},
		{"cooldown", aggragates.StrategyParams{Cooldown: true}, ""},
		{"smartTakeLoss never holds an exit, verdict or not", aggragates.StrategyParams{SmartTakeLoss: true}, ""},
		{"useAI", aggragates.StrategyParams{UseAI: true}, "AI market is bullish"},
		{"usePatterns", aggragates.StrategyParams{UsePatterns: true}, "pattern: ascending triangle in play, riding to target 104500.0000"},
		{"dynamicParams never holds an exit, both reads bearish", aggragates.StrategyParams{DynamicParams: true}, ""},
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

// The first fill is the cooldown's: against the full payload a refused first
// fill, long or inverse, is held and the only row is the cooldown's.
func TestShouldHoldCooldownAloneHoldsTheRefusedFirstFill(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		trade := ownershipTrade("buy", 0)
		trade.Inverse = inverse
		trade.Strategy.Params = aggragates.StrategyParams{Cooldown: true}

		event := ownershipEvent(trade, fullHoldPayload(), expensiveCooldown())
		event.Params.OldPosition = "new"

		held, err := ShouldHold(event)
		if err == nil {
			t.Fatalf("inverse=%v: the cooldown must hold the refused first fill", inverse)
		}
		assertOnlyFamily(t, held.Trade.Logs, "cooldown: trying to get a better entry price")
	}
}

// DynamicParams has no seat on the first fill: with both reads bearish it
// holds no new ladder, long or inverse, and writes no row. It sizes that
// first entry for the raised rows; it never refuses it.
func TestShouldHoldDynamicParamsHoldsNoFirstFill(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		trade := ownershipTrade("buy", 0)
		trade.Inverse = inverse
		trade.Strategy.Params = aggragates.StrategyParams{DynamicParams: true}

		event := ownershipEvent(trade, fullHoldPayload(), expensiveCooldown())
		event.Params.OldPosition = "new"

		held, err := ShouldHold(event)
		if err != nil || len(held.Trade.Logs) != 0 {
			t.Fatalf("inverse=%v: DynamicParams must not touch the first fill, got %v %v", inverse, err, messages(held.Trade.Logs))
		}
	}
}

// A force-trailing re-anchor reads as the position it re-arms: the gates
// have power on it and the row keeps the raw position name.
func TestShouldHoldForceTrailingStatesRunTheGates(t *testing.T) {
	sl := ownershipTrade("forceTrailingStopLoss", 4)
	sl.Strategy.Params = aggragates.StrategyParams{UseAI: true}
	held, err := ShouldHold(ownershipEvent(sl, fullHoldPayload(), expensiveCooldown()))
	if err == nil || held.Trade.Logs[0].Message != "Hold forceTrailingStopLoss: AI market is bearish" {
		t.Fatalf("useAI must gate a force-trailing stopLoss, got %v %v", err, messages(held.Trade.Logs))
	}

	ai := fullHoldPayload()
	ai.AIMarketBearish = false
	ai.AIMarketBullish = true
	tp := ownershipTrade("forceTrailingTakeProfit", 4)
	tp.Strategy.Params = aggragates.StrategyParams{UseAI: true}
	held, err = ShouldHold(ownershipEvent(tp, ai, expensiveCooldown()))
	if err == nil || held.Trade.Logs[0].Message != "Hold forceTrailingTakeProfit: AI market is bullish" {
		t.Fatalf("useAI must gate a force-trailing takeProfit, got %v %v", err, messages(held.Trade.Logs))
	}

	tp = ownershipTrade("forceTrailingTakeProfit", 4)
	if held, err := ShouldHold(ownershipEvent(tp, ai, expensiveCooldown())); err != nil {
		t.Fatalf("all flags off must not hold a force-trailing takeProfit, got %v", messages(held.Trade.Logs))
	}
}
