package actions

import (
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
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

// The second depth is gated from the first fill: this is what trade 32309
// escaped, its depth 2 landing 7m25s after the entry.
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

// The escalated hold (a depth that filled the instant the first hold lifted)
// still parks the next depth past the point an unescalated one would have
// freed it, and it does lift eventually.
//
// The exact escalated duration is NOT asserted here: it is base * factor, and
// the factor is unexported — the schedule itself is pinned in the cooldown
// package (TestDepthSpacingClampsTheHoldAtTheCeiling). What this test owns is
// the wiring: that ShouldHold honours the escalation at all. Still parked one
// base hold past the expiry is exactly that evidence, since step 1 would have
// freed it there under any factor above one.
func TestDepthSpacingEscalatesWhenADepthFillsTheInstantTheHoldLifts(t *testing.T) {
	first := testutil.At("09:00:00")
	expiry := first.Add(cooldown.DepthSpacingBaseHold)
	trade := testutil.DepthTrade(first, expiry)

	if _, err := ShouldHold(depthEvent(trade, expiry.Add(cooldown.DepthSpacingBaseHold))); err == nil {
		t.Fatal("the escalated hold must park the next depth past one base hold")
	}
	if _, err := ShouldHold(depthEvent(trade, expiry.Add(30*24*time.Hour))); err != nil {
		t.Fatalf("the escalated hold must lift, got %v", err)
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
func TestDepthSpacingNeverHoldsALadderAFullWindowPastEachExpiry(t *testing.T) {
	start := testutil.At("09:00:00")
	spacing := cooldown.DepthSpacingBaseHold + cooldown.DepthSpacingWindow
	var placements []time.Time
	for i := 0; i < 7; i++ {
		placements = append(placements, start.Add(time.Duration(i)*spacing))
	}
	trade := testutil.DepthTrade(placements...)
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
	if len(held.Trade.Logs) != 0 {
		t.Fatalf("no row may be written with the flag off, got %v", messages(held.Trade.Logs))
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
	want := "Hold stopLoss: cooldown: depths too close (depth 2, step 2), next add parked for "
	if !strings.HasPrefix(row.Message, want) {
		t.Fatalf("row = %q, want the prefix %q", row.Message, want)
	}
	if held.Trade.PositionType != "active" {
		t.Errorf("position restored to %q, want the old position", held.Trade.PositionType)
	}

	held.Trade.PositionType = "stopLoss"
	again, err := ShouldHold(depthEvent(held.Trade, trade25858[1].Add(2*time.Minute)))
	if err == nil {
		t.Fatal("expected the depth to still be parked on the next tick")
	}
	if len(again.Trade.Logs) != 1 {
		t.Fatalf("a standing hold must not write a row per tick, got %v", messages(again.Trade.Logs))
	}
	for _, prefix := range []string{"regime:", "pattern:", "crash-guard:", "smartTakeLoss:"} {
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

// The depth in the row is the trade's depth, not the escalation counter. They
// coincide only on a ladder where every entry after the first was fast — which
// is what every other test here builds, and why the bug survived. A ladder with
// one real pause separates them: five filled entries, a lower escalation step.
//
// It matters because the row is the only operator-visible output of this gate,
// and regime and crash-guard both print ladder.CountFilledEntries for the
// same trade on the same tick.
func TestDepthSpacingRowReportsTheLadderDepthNotTheEscalationStep(t *testing.T) {
	// One real pause — a full window past the first hold's expiry, which is
	// what resets the escalation — then three fast depths behind it.
	start := testutil.At("09:00:00")
	pause := start.Add(cooldown.DepthSpacingBaseHold + cooldown.DepthSpacingWindow)
	ladder := []time.Time{
		start, pause,
		pause.Add(5 * time.Minute), pause.Add(10 * time.Minute), pause.Add(15 * time.Minute),
	}
	trade := testutil.DepthTrade(ladder...)

	held, err := ShouldHold(depthEvent(trade, pause.Add(16*time.Minute)))
	if err == nil {
		t.Fatal("expected the sixth entry to be parked")
	}
	row := held.Trade.Logs[0].Message
	if !strings.Contains(row, "(depth 5,") {
		t.Errorf("row = %q, want the ladder depth 5", row)
	}
	if !strings.Contains(row, "step 4)") {
		t.Errorf("row = %q, want the escalation step 4 beside it — the pause reset it", row)
	}
}
