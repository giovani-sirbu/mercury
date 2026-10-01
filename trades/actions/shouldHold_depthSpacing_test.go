package actions

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

var trade25858 = testutil.Trade25858()

// depthEvent runs the trade through ShouldHold at the given tick.
func depthEvent(trade aggragates.Trades, now time.Time) events.Events {
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params:    aggragates.Params{OldPosition: "active"},
		Timestamp: now.UnixMilli(),
	}
}

// depthRow is the log row the gate leaves for a depth, stamped when it first
// held. The step it prints is wrong on purpose: the rule derives its own.
func depthRow(trade aggragates.Trades, depth int, at time.Time) aggragates.TradesLogs {
	return aggragates.TradesLogs{
		TradeID:   trade.ID,
		Type:      aggragates.LOG_INFO,
		Message:   fmt.Sprintf("Hold stopLoss: cooldown: depths too close (depth %d, step 99), next add parked for 1h0m0s", depth),
		CreatedAt: at,
	}
}

// heldDepth is the trade with the pair the gate leaves behind when it holds a
// depth, stamped at: the row an operator reads and the depth-spacing event the
// later depths count the activation from. The event carries the same wrong
// step as the row: the rule derives its own.
func heldDepth(trade aggragates.Trades, depth int, at time.Time) aggragates.Trades {
	data := cooldown.DepthSpacingEvent{Event: gates.EventHeld, Depth: depth, Step: 99, Hold: time.Hour}

	return aggragates.AppendStrategyRow(trade, depthRow(trade, depth, at), cooldown.NewDepthSpacingEvent(trade.ID, data, at))
}

// The second depth is gated from the first fill: that is the depth a fold
// seeded at the first fill let through, minutes after the entry.
func TestDepthSpacingHoldsTheSecondDepthFromTheFirstFill(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0])
	eligible := testutil.At("13:41:08").Add(cooldown.DepthSpacingBaseHold)

	if _, err := ShouldHold(depthEvent(trade, testutil.At("13:48:33"))); err == nil {
		t.Fatal("the 13:48:33 depth of trade 32309 must be parked")
	}
	if _, err := ShouldHold(depthEvent(trade, eligible.Add(-time.Second))); err == nil {
		t.Fatal("a second under the expiry must still be parked")
	}
	if _, err := ShouldHold(depthEvent(trade, eligible)); err != nil {
		t.Fatalf("the hold must lift exactly one base hold after the fill, got %v", err)
	}
}

// The escalated hold (a depth that filled inside the hold of the depth before
// it, the price release having bought it out, after the gate had activated at
// that depth) still parks the next depth past the point an unescalated one
// would have freed it, and it does lift eventually.
//
// The exact escalated duration is NOT asserted here: it is base * factor, and
// the factor is unexported — the schedule itself is pinned in the cooldown
// package (TestDepthSpacingClampsTheHoldAtTheCeiling). What this test owns is
// the wiring: that ShouldHold honours the escalation at all. Still parked one
// base hold past the fill is exactly that evidence, since the first level
// would have freed it there under any factor above one.
//
// The activations are the depth-spacing events the trade carries. The rows
// beside them are text: the same ladder with its rows alone has no activation
// to count, reads as a first activation at every tick and is freed there.
func TestDepthSpacingEscalatesWhenADepthFillsInsideThePreviousHold(t *testing.T) {
	first := testutil.At("09:00:00")
	expiry := first.Add(cooldown.DepthSpacingBaseHold)
	bought := expiry.Add(-time.Minute)
	trade := testutil.DepthTrade(first, bought)
	trade = heldDepth(trade, 1, first.Add(time.Minute))
	atBase := bought.Add(cooldown.DepthSpacingBaseHold)

	if _, err := ShouldHold(depthEvent(trade, atBase)); err == nil {
		t.Fatal("the escalated hold must park the next depth past one base hold")
	}
	if _, err := ShouldHold(depthEvent(trade, bought.Add(30*24*time.Hour))); err != nil {
		t.Fatalf("the escalated hold must lift, got %v", err)
	}

	rowsOnly := testutil.DepthTrade(first, bought)
	rowsOnly.Logs = []aggragates.TradesLogs{depthRow(rowsOnly, 1, first.Add(time.Minute))}
	if _, err := ShouldHold(depthEvent(rowsOnly, atBase)); err != nil {
		t.Fatalf("rows without their events count no activation, so the base hold lifts here, got %v", err)
	}
}

// The mirror: a depth that fills the instant the previous hold lifts waited it
// out. That hold ended by time, the price release never bought the depth, and
// the next activation is step 1: the depth is parked for its own base hold and
// freed exactly there, where an escalated one would still be parked.
func TestDepthSpacingKeepsStepOneWhenADepthFillsTheInstantTheHoldLifts(t *testing.T) {
	first := testutil.At("09:00:00")
	expiry := first.Add(cooldown.DepthSpacingBaseHold)
	trade := testutil.DepthTrade(first, expiry)
	trade = heldDepth(trade, 1, first.Add(time.Minute))
	eligible := expiry.Add(cooldown.DepthSpacingBaseHold)

	if _, err := ShouldHold(depthEvent(trade, eligible.Add(-time.Second))); err == nil {
		t.Fatal("the depth carries its own base hold: a second under it must still be parked")
	}
	if _, err := ShouldHold(depthEvent(trade, eligible)); err != nil {
		t.Fatalf("a depth that waited the hold out is at step 1 and frees one base hold after its fill, got %v", err)
	}
}

// One fill carries a base hold and nothing more: past it the ladder is free,
// however long the trade has been open.
func TestDepthSpacingReleasesASingleFillAfterTheBaseHold(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0])
	for _, elapsed := range []time.Duration{cooldown.DepthSpacingBaseHold, 4 * time.Hour, 30 * 24 * time.Hour} {
		if _, err := ShouldHold(depthEvent(trade, trade25858[0].Add(elapsed))); err != nil {
			t.Fatalf("one fill parks nothing past the base hold (+%s), got %v", elapsed, err)
		}
	}
}

// A full window past each expiry is genuinely spaced: nothing escalates and
// nothing is parked. The distance is base + window rather than a multiple of
// the base hold — the window has been both narrower and wider than the hold
// across calibrations, so a fixed multiple is only accidentally far enough.
// The gate activated at every depth, a minute after its fill, so each has an
// activation to count and none of them escalates.
func TestDepthSpacingNeverHoldsALadderAFullWindowPastEachExpiry(t *testing.T) {
	start := testutil.At("09:00:00")
	spacing := cooldown.DepthSpacingBaseHold + cooldown.DepthSpacingWindow
	var placements []time.Time
	for i := 0; i < 7; i++ {
		placements = append(placements, start.Add(time.Duration(i)*spacing))
	}
	trade := testutil.DepthTrade(placements...)
	for depth, placed := range placements {
		trade = heldDepth(trade, depth+1, placed.Add(time.Minute))
	}
	next := placements[len(placements)-1].Add(spacing)

	if _, err := ShouldHold(depthEvent(trade, next)); err != nil {
		t.Fatalf("a well-spaced ladder must never be parked, got %v", err)
	}
}

// Unknown clocks fail open, the posture every cooldown gate keeps: a tick with
// no time, or a depth whose placement stamp was never persisted, parks
// nothing. Live-testing memory trades arrive exactly like this.
func TestDepthSpacingNeverHoldsOnUnknownClocks(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])

	noTick := depthEvent(trade, time.Time{})
	noTick.Timestamp = 0
	if _, err := ShouldHold(noTick); err != nil {
		t.Fatalf("a zero tick clock must never park a depth, got %v", err)
	}

	unstamped := testutil.DepthTrade(trade25858[0], trade25858[1])
	unstamped.History[1].CreatedAt = time.Time{}
	if _, err := ShouldHold(depthEvent(unstamped, trade25858[1].Add(time.Minute))); err != nil {
		t.Fatalf("an unstamped depth must never park the ladder, got %v", err)
	}
}

func TestDepthSpacingIsInertWithoutTheCooldownFlag(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])
	trade.Strategy.Params.Cooldown = false

	held, err := ShouldHold(depthEvent(trade, trade25858[1].Add(time.Minute)))
	if err != nil {
		t.Fatalf("depth spacing must not fire without params.Cooldown, got %v", err)
	}
	if len(held.Trade.Logs) != 0 || len(held.Trade.StrategyEvents) != 0 {
		t.Fatalf("no row and no event may be written with the flag off, got %v and %d events", messages(held.Trade.Logs), len(held.Trade.StrategyEvents))
	}
}

// stopLoss only: a gate on new capital must never defer an exit, and the
// first fill has no previous depth to be close to.
func TestDepthSpacingOnlyGatesStopLoss(t *testing.T) {
	for _, position := range []string{"takeProfit", "forceTrailingTakeProfit", "buy", "sell"} {
		trade := testutil.DepthTrade(trade25858[0], trade25858[1])
		trade.PositionType = position
		if _, err := ShouldHold(depthEvent(trade, trade25858[1].Add(time.Minute))); err != nil {
			t.Fatalf("depth spacing must not gate %q, got %v", position, err)
		}
	}

	// The force-trailing re-anchor of a stopLoss IS the depth it re-arms.
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])
	trade.PositionType = "forceTrailingStopLoss"
	if _, err := ShouldHold(depthEvent(trade, trade25858[1].Add(time.Minute))); err == nil {
		t.Fatal("a force-trailing stopLoss re-anchor must be gated like the depth it re-arms")
	}
}

// The row names the cooldown family (the flag that owns the gate) and stays
// byte-identical while the hold stands, so gates.SaveHoldLog collapses it to
// one entry instead of one per tick.
func TestDepthSpacingWritesOneStableCooldownRow(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])
	// The clock half of the row alone: without a ladder row there is no
	// release price to print, and the price half is pinned by the release
	// tests in the cooldown package.
	trade.StrategyPair.StrategySettings = nil

	held, err := ShouldHold(depthEvent(trade, trade25858[1].Add(time.Minute)))
	if err == nil {
		t.Fatal("expected the depth to be parked")
	}
	if len(held.Trade.Logs) != 1 {
		t.Fatalf("expected one row, got %v", messages(held.Trade.Logs))
	}
	row := held.Trade.Logs[0]
	if row.Type != aggragates.LOG_INFO {
		t.Errorf("row type = %q, want %q", row.Type, aggragates.LOG_INFO)
	}
	// Everything the row owes an operator except the wait itself: the family,
	// the depth and the escalation step. The wait is base * factor and the
	// factor is unexported, so pinning it here would only re-encode the
	// schedule the cooldown package already tests — and this test is about
	// the row, not the calibration. That it stays byte-identical tick to tick
	// is asserted below, which is the property SaveHoldLog depends on.
	want := "Hold stopLoss: cooldown: depths too close (depth 2, step 1), next add parked for "
	if !strings.HasPrefix(row.Message, want) {
		t.Fatalf("row = %q, want the prefix %q", row.Message, want)
	}
	if held.Trade.PositionType != "active" {
		t.Errorf("position restored to %q, want the old position", held.Trade.PositionType)
	}
	// The event is written beside the row, and the standing hold collapses
	// the pair together: one event, not one per tick.
	assertNewestPair(t, held.Trade, aggragates.StrategyParamCooldown, cooldown.GateDepthSpacing, gates.EventHeld)
	if len(held.Trade.StrategyEvents) != 1 {
		t.Fatalf("expected one event, got %d", len(held.Trade.StrategyEvents))
	}

	held.Trade.PositionType = "stopLoss"
	again, err := ShouldHold(depthEvent(held.Trade, trade25858[1].Add(2*time.Minute)))
	if err == nil {
		t.Fatal("expected the depth to still be parked on the next tick")
	}
	if len(again.Trade.Logs) != 1 || len(again.Trade.StrategyEvents) != 1 {
		t.Fatalf("a standing hold must not write a row or an event per tick, got %v and %d events",
			messages(again.Trade.Logs), len(again.Trade.StrategyEvents))
	}
	for _, prefix := range []string{"pattern:", "smartTakeLoss:"} {
		if strings.Contains(row.Message, prefix) {
			t.Fatalf("row %q leaks the %q family", row.Message, prefix)
		}
	}
}

// An inverse trade enters on SELL: the same gate, the other side.
func TestDepthSpacingReadsTheInverseEntrySide(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])
	trade.Inverse = true
	for i := range trade.History {
		trade.History[i].Type = "SELL"
	}

	if _, err := ShouldHold(depthEvent(trade, trade25858[1].Add(time.Minute))); err == nil {
		t.Fatal("an inverse ladder must be held on its SELL entries")
	}
}

// The depth in the row is the trade's depth, not the step. They coincide only
// on a ladder the gate held at every entry — which is what every other test
// here builds, and why the bug survived. A ladder with one real pause separates
// them: five filled entries, two activations.
//
// It matters because the row is the only operator-visible output of this gate,
// and the depth an operator reads it against is ladder.CountFilledEntries for
// the same trade on the same tick.
func TestDepthSpacingRowReportsTheLadderDepthNotTheStep(t *testing.T) {
	start := testutil.At("09:00:00")
	pause := start.Add(cooldown.DepthSpacingBaseHold + cooldown.DepthSpacingWindow)
	ladder := []time.Time{
		start, pause,
		pause.Add(5 * time.Minute), pause.Add(10 * time.Minute), pause.Add(15 * time.Minute),
	}
	// The gate held the fourth entry; the fifth is the tick under test.
	trade := heldDepth(testutil.DepthTrade(ladder...), 4, pause.Add(11*time.Minute))

	held, err := ShouldHold(depthEvent(trade, pause.Add(16*time.Minute)))
	if err == nil {
		t.Fatal("expected the sixth entry to be parked")
	}
	row := held.Trade.Logs[len(held.Trade.Logs)-1].Message
	if !strings.Contains(row, "(depth 5,") {
		t.Errorf("row = %q, want the ladder depth 5", row)
	}
	if !strings.Contains(row, "step 2)") {
		t.Errorf("row = %q, want step 2 beside it — two activations on a five-depth ladder", row)
	}
}
