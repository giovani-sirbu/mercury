package cooldown

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

var trade25858 = testutil.Trade25858()

// spacingRow is the log row the gate leaves behind for a depth. The step it
// prints is deliberately wrong: the rule derives the step and must never read
// it back.
func spacingRow(depth int, at time.Time) aggragates.TradesLogs {
	return aggragates.TradesLogs{
		Message:   fmt.Sprintf("Hold stopLoss: %s%d, step 99), next add parked for 1h0m0s", depthSpacingMarker, depth),
		CreatedAt: at,
	}
}

// spacingState folds a ladder placed at the given stamps, with the given rows
// already logged, at the tick `now`.
func spacingState(placements []time.Time, rows []aggragates.TradesLogs, now time.Time) depthSpacingState {
	return depthSpacingEligibleFrom(rows, depthFills(testutil.DepthTrade(placements...)), now)
}

// The hold is there from the first fill, and the tick that meets it is the
// first activation: step 1, the base hold.
func TestDepthSpacingHoldsTheSecondDepthFromTheFirstFill(t *testing.T) {
	now := testutil.At("13:48:33")
	state := spacingState(trade25858[:1], nil, now)

	if state.step != 1 {
		t.Errorf("step = %d, want 1 — the first activation", state.step)
	}
	if state.hold != DepthSpacingBaseHold {
		t.Errorf("hold = %s, want the base %s", state.hold, DepthSpacingBaseHold)
	}
	want := testutil.At("13:41:08").Add(DepthSpacingBaseHold)
	if !state.eligibleFrom.Equal(want) {
		t.Fatalf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
	}
	if !now.Before(state.eligibleFrom) {
		t.Fatal("the second depth of trade 32309 must fall inside the hold")
	}
}

// The recorded ladder of trade 109059: six fills over two days, the gate
// activated at depths 2, 3 and 6. Depth 3 activated inside the window past
// depth 2's expiry, so the cascade deepens; depth 6 activated days later, so
// it starts over. Fills that the gate never held (4 and 5) count for nothing.
func TestDepthSpacingCountsActivationsOnTheRecordedTrade109059(t *testing.T) {
	at := func(day int, clock string) time.Time {
		parsed, err := time.Parse("2006-01-02 15:04:05", fmt.Sprintf("2026-05-%02d %s", day, clock))
		if err != nil {
			t.Fatal(err)
		}
		return parsed.UTC()
	}
	placements := []time.Time{
		at(5, "07:20:28"), at(5, "14:06:49"), at(5, "15:08:28"),
		at(5, "19:08:38"), at(6, "11:28:37"), at(7, "21:56:13"),
	}
	rows := []aggragates.TradesLogs{
		spacingRow(2, at(5, "14:40:00")),
		spacingRow(3, at(5, "18:24:00")),
		spacingRow(6, at(7, "22:51:00")),
	}

	// The fixture is only this shape while the constants keep it so.
	expiry2 := placements[1].Add(depthSpacingHoldFor(1))
	if gap := rows[1].CreatedAt.Sub(expiry2); gap >= DepthSpacingWindow {
		t.Fatalf("depth 3 activated %s past the expiry, the fixture needs it inside %s", gap, DepthSpacingWindow)
	}
	expiry3 := placements[2].Add(depthSpacingHoldFor(2))
	if gap := rows[2].CreatedAt.Sub(expiry3); gap < DepthSpacingWindow {
		t.Fatalf("depth 6 activated %s past the expiry, the fixture needs it at %s or more", gap, DepthSpacingWindow)
	}

	cases := []struct {
		depth    int
		now      time.Time
		wantStep int
	}{
		{2, rows[0].CreatedAt, 1},
		{3, rows[1].CreatedAt, 2},
		{6, rows[2].CreatedAt, 1},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("depth %d", c.depth), func(t *testing.T) {
			state := spacingState(placements[:c.depth], rows, c.now)
			if state.step != c.wantStep {
				t.Fatalf("step = %d, want %d", state.step, c.wantStep)
			}
			wantHold := depthSpacingHoldFor(c.wantStep)
			if state.hold != wantHold {
				t.Errorf("hold = %s, want %s", state.hold, wantHold)
			}
			if want := placements[c.depth-1].Add(wantHold); !state.eligibleFrom.Equal(want) {
				t.Errorf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
			}
		})
	}
}

// A row logged again for the same depth is gates.SaveHoldLog re-logging the
// standing hold: one activation, at the earliest stamp, however the rows are
// ordered.
func TestDepthSpacingCountsAReLoggedDepthOnce(t *testing.T) {
	start := testutil.At("09:00:00")
	placements := []time.Time{start, start.Add(time.Hour), start.Add(2 * time.Hour)}
	first := start.Add(30 * time.Minute)

	cases := []struct {
		name string
		rows []aggragates.TradesLogs
	}{
		{"once", []aggragates.TradesLogs{spacingRow(1, first)}},
		{"relogged", []aggragates.TradesLogs{spacingRow(1, first), spacingRow(1, first.Add(6*time.Hour)), spacingRow(1, first.Add(12*time.Hour))}},
		{"newest first", []aggragates.TradesLogs{spacingRow(1, first.Add(6*time.Hour)), spacingRow(1, first)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Depth 2 is the current depth, held on its own first tick.
			state := spacingState(placements[:2], c.rows, start.Add(time.Hour+time.Minute))
			if state.step != 2 {
				t.Fatalf("step = %d, want 2 — depth 1 counts once", state.step)
			}
		})
	}
	if got := depthSpacingActivations(cases[1].rows)[1]; !got.Equal(first) {
		t.Fatalf("activation = %s, want the earliest row %s", got, first)
	}
}

// The first held tick has no row yet and already reports its step: 1 on a
// fresh trade, 2 when a previous activation is still in the window.
func TestDepthSpacingFirstHeldTickReportsItsStep(t *testing.T) {
	start := testutil.At("09:00:00")
	placements := []time.Time{start, start.Add(time.Hour)}
	now := start.Add(time.Hour + time.Minute)

	if got := spacingState(placements[:1], nil, start.Add(time.Minute)).step; got != 1 {
		t.Errorf("fresh trade: step = %d, want 1", got)
	}
	if got := spacingState(placements, nil, now).step; got != 1 {
		t.Errorf("no previous activation: step = %d, want 1", got)
	}
	rows := []aggragates.TradesLogs{spacingRow(1, start.Add(time.Minute))}
	if got := spacingState(placements, rows, now).step; got != 2 {
		t.Errorf("previous activation in the window: step = %d, want 2", got)
	}
}

// An activation DepthSpacingWindow or more past the previous expiry starts
// over at step 1; one second short of it keeps counting.
func TestDepthSpacingResetsAfterTheWindow(t *testing.T) {
	start := testutil.At("09:00:00")
	activated := start.Add(time.Minute)
	expiry := start.Add(depthSpacingHoldFor(1))
	rows := []aggragates.TradesLogs{spacingRow(1, activated)}

	cases := []struct {
		name     string
		now      time.Time
		wantStep int
	}{
		{"inside the hold", expiry.Add(-time.Minute), 2},
		{"a second short of the window", expiry.Add(DepthSpacingWindow - time.Second), 2},
		{"exactly the window", expiry.Add(DepthSpacingWindow), 1},
		{"well past the window", expiry.Add(3 * DepthSpacingWindow), 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			placements := []time.Time{start, c.now.Add(-time.Second)}
			state := spacingState(placements, rows, c.now)
			if state.step != c.wantStep {
				t.Fatalf("step = %d, want %d", state.step, c.wantStep)
			}
			if state.hold != depthSpacingHoldFor(c.wantStep) {
				t.Errorf("hold = %s, want %s", state.hold, depthSpacingHoldFor(c.wantStep))
			}
		})
	}
}

// Rows with no stamp or no readable depth are skipped, and a ladder with an
// unknown clock never holds at all.
func TestDepthSpacingSkipsUnreadableRowsAndFailsOpenOnUnknownClocks(t *testing.T) {
	start := testutil.At("09:00:00")
	placements := []time.Time{start, start.Add(time.Hour)}
	now := start.Add(time.Hour + time.Minute)

	unstamped := spacingRow(1, time.Time{})
	bad := aggragates.TradesLogs{Message: "Hold stopLoss: " + depthSpacingMarker + "x, step 1)", CreatedAt: start.Add(time.Minute)}
	noDepth := aggragates.TradesLogs{Message: "Hold stopLoss: " + depthSpacingMarker, CreatedAt: start.Add(time.Minute)}
	zero := spacingRow(0, start.Add(time.Minute))
	for name, row := range map[string]aggragates.TradesLogs{"no stamp": unstamped, "bad depth": bad, "no depth": noDepth, "depth zero": zero} {
		if got := spacingState(placements, []aggragates.TradesLogs{row}, now).step; got != 1 {
			t.Errorf("%s: step = %d, want 1 — the row carries nothing to count", name, got)
		}
	}

	unknown := testutil.DepthTrade(placements...)
	unknown.History[1].CreatedAt = time.Time{}
	if state := depthSpacingEligibleFrom(nil, depthFills(unknown), now); !state.eligibleFrom.IsZero() {
		t.Errorf("an unstamped fill must leave the gate open, got %s", state.eligibleFrom)
	}
	event := events.Events{Trade: unknown}
	if got := DepthSpacingHoldReason(event, "stopLoss"); got != "" {
		t.Errorf("no tick clock must never hold, got %q", got)
	}
}

// The step keeps counting past the hold cap; only the hold is capped.
func TestDepthSpacingStepCountsPastTheHoldCap(t *testing.T) {
	start := testutil.At("09:00:00")
	placements := []time.Time{start}
	rows := []aggragates.TradesLogs{spacingRow(1, start.Add(time.Minute))}
	state := spacingState(placements, rows, start.Add(time.Minute))

	const depths = 12
	for depth := 2; depth <= depths; depth++ {
		fill := state.eligibleFrom
		placements = append(placements, fill)
		rows = append(rows, spacingRow(depth, fill.Add(time.Minute)))
		state = spacingState(placements, rows, fill.Add(time.Minute))
		if state.step != depth {
			t.Fatalf("depth %d: step = %d, want %d", depth, state.step, depth)
		}
	}
	if state.hold != depthSpacingMaxHold {
		t.Fatalf("hold = %s, want the cap %s", state.hold, depthSpacingMaxHold)
	}
	if want := placements[depths-1].Add(depthSpacingMaxHold); !state.eligibleFrom.Equal(want) {
		t.Fatalf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
	}
}

// The message stays byte-identical across ticks once the row exists, and the
// tick that writes it already printed the same string.
func TestDepthSpacingMessageIsStableOnceTheRowExists(t *testing.T) {
	start := testutil.At("09:00:00")
	trade := hbarTrade(1)
	trade.History = testutil.DepthTrade(start, start.Add(time.Hour)).History
	trade.Logs = []aggragates.TradesLogs{spacingRow(1, start.Add(time.Minute))}
	tick := func(at time.Time) events.Events {
		return events.Events{Trade: trade, Timestamp: at.UnixMilli()}
	}

	first := DepthSpacingHoldReason(tick(start.Add(time.Hour+time.Minute)), "stopLoss")
	if !strings.Contains(first, "(depth 2, step 2)") {
		t.Fatalf("first held tick = %q, want depth 2 step 2", first)
	}

	trade.Logs = append(trade.Logs, aggragates.TradesLogs{Message: "Hold stopLoss: " + first, CreatedAt: start.Add(time.Hour + time.Minute)})
	for _, later := range []time.Duration{2 * time.Minute, 30 * time.Minute, 2 * time.Hour} {
		if got := DepthSpacingHoldReason(tick(start.Add(time.Hour+later)), "stopLoss"); got != first {
			t.Fatalf("after %s:\n got %q\nwant %q", later, got, first)
		}
	}
}

// A ladder outside a cascade still parks its next depth, for the base hold and
// the same release price the first cascade level asks for. The row prints the
// effective step, so it reads "step 1" while the raw step is zero.
func TestDepthSpacingStepZeroStillParksAndPriceReleases(t *testing.T) {
	trade := hbarTrade(1)
	trade.History = testutil.DepthTrade(trade25858[0]).History
	lastFill := trade.History[0].Price
	release, ok := depthSpacingReleasePrice(trade, lastFill, 1)
	if !ok {
		t.Fatal("no release price at the base level")
	}
	tick := trade25858[0].Add(time.Hour).UnixMilli()

	held := events.Events{Trade: trade, Timestamp: tick}
	held.Trade.PositionPrice = release + 0.01
	want := fmt.Sprintf(
		"cooldown: depths too close (depth 1, step 1), next add parked for %s or until %s",
		DepthSpacingBaseHold, strconv.FormatFloat(release, 'f', -1, 64),
	)
	got := DepthSpacingHoldReason(held, "stopLoss")
	if got != want {
		t.Fatalf("row = %q, want %q", got, want)
	}
	if again := DepthSpacingHoldReason(held, "stopLoss"); again != got {
		t.Fatalf("the row must be byte-stable while the hold stands: %q then %q", got, again)
	}

	paid := held
	paid.Trade.PositionPrice = release
	if got := DepthSpacingHoldReason(paid, "stopLoss"); got != "" {
		t.Fatalf("at the release price a step-0 hold must lift, got %q", got)
	}

	expired := events.Events{Trade: trade, Timestamp: trade25858[0].Add(DepthSpacingBaseHold).UnixMilli()}
	if got := DepthSpacingHoldReason(expired, "stopLoss"); got != "" {
		t.Fatalf("the hold must lift at the base expiry, got %q", got)
	}
}

// A ladder whose depths land a full window past the previous expiry is
// genuinely spaced: no tick is ever held. That distance is base + window, and
// it is written as such rather than as a multiple of the base hold — the
// window is a calibration knob and has been both narrower and wider than the
// hold, so any fixed multiple is only accidentally far enough.
func TestDepthSpacingNeverHoldsALadderAFullWindowPastEachExpiry(t *testing.T) {
	start := testutil.At("09:00:00")
	spacing := DepthSpacingBaseHold + DepthSpacingWindow
	var placements []time.Time
	for i := 0; i < 7; i++ {
		placements = append(placements, start.Add(time.Duration(i)*spacing))
	}
	last := placements[len(placements)-1]

	state := spacingState(placements, nil, last)
	if state.step != 1 || state.hold != DepthSpacingBaseHold {
		t.Fatalf("a well-spaced ladder must stay at base: step %d hold %s", state.step, state.hold)
	}
	if want := last.Add(DepthSpacingBaseHold); !state.eligibleFrom.Equal(want) {
		t.Fatalf("eligibleFrom = %s, want %s", state.eligibleFrom, want)
	}
	// The next depth at the same cadence is already past the expiry.
	if next := last.Add(spacing); next.Before(state.eligibleFrom) {
		t.Error("the next depth at this cadence must not be parked")
	}
}

// Escalating from the base hold reaches the ceiling a few fast depths in.
// The hold clamps there and stays clamped: past the ceiling the gate would
// make the trade sit out the bottom of the move.
// The schedule is asserted as a rule, not as a list of durations: the base
// hold, the factor and the window are calibration knobs that move, and a table
// of literals turns every calibration change into a red suite that says
// nothing.
func TestDepthSpacingClampsTheHoldAtTheCeiling(t *testing.T) {
	if got := depthSpacingHoldFor(1); got != DepthSpacingBaseHold {
		t.Errorf("the first fast depth costs %s, want the base %s", got, DepthSpacingBaseHold)
	}

	previous := DepthSpacingBaseHold
	for step := 2; step <= 40; step++ {
		// The factor may be fractional, so the expectation scales the same way
		// depthSpacingHoldFor does — through float64, not as a Duration.
		want := time.Duration(float64(previous) * depthSpacingFactor)
		if want > depthSpacingMaxHold {
			want = depthSpacingMaxHold
		}

		got := depthSpacingHoldFor(step)
		if got != want {
			t.Fatalf("depthSpacingHoldFor(%d) = %s, want %s", step, got, want)
		}
		if got > depthSpacingMaxHold {
			t.Fatalf("depthSpacingHoldFor(%d) = %s, past the %s ceiling", step, got, depthSpacingMaxHold)
		}
		previous = got
	}

	if depthSpacingHoldFor(40) != depthSpacingMaxHold {
		t.Error("a long cascade must sit at the ceiling, not overflow past it")
	}
	if depthSpacingHoldFor(0) != 0 {
		t.Error("no ladder at all earns no hold")
	}
}

// A depth whose placement stamp was never persisted voids the whole read:
// live-testing memory trades arrive exactly like this.
func TestDepthFillsVoidTheReadOnAnUnstampedDepth(t *testing.T) {
	unstamped := testutil.DepthTrade(trade25858[0], trade25858[1])
	unstamped.History[1].CreatedAt = time.Time{}
	if fills := depthFills(unstamped); fills != nil {
		t.Fatalf("an unstamped depth must void the read, got %v", fills)
	}
}

// Partial fills update the same exchange order and are one depth, not two —
// the membership rule mirrors ladder.CountFilledEntries. Accounting rows
// (an impasse child's profit marked onto the parent at the sentinel price)
// are bookkeeping and never a depth either.
func TestDepthFillsCountDistinctOrdersAndSkipAccountingRows(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])
	trade.History = append(trade.History,
		// A partial top-up of depth 2, hours later: same order id.
		aggragates.TradesHistory{Type: "BUY", Quantity: 0.5, Price: 99, OrderId: 2, CreatedAt: testutil.At("18:00:00")},
		// A child's profit transfer at the sentinel price.
		aggragates.TradesHistory{Type: "BUY", Quantity: 3, Price: 1e-13, OrderId: 77, CreatedAt: testutil.At("18:30:00")},
		// The exit leg of a long is not an entry.
		aggragates.TradesHistory{Type: "SELL", Quantity: 2, Price: 120, OrderId: 78, CreatedAt: testutil.At("19:00:00")},
	)

	fills := depthFills(trade)
	if len(fills) != 2 {
		t.Fatalf("expected 2 depths, got %v", fills)
	}
	if !fills[0].At.Equal(trade25858[0]) || !fills[1].At.Equal(trade25858[1]) {
		t.Fatalf("depths = %v, want the two placements", fills)
	}
	// The price travels with the stamp: the release leg measures from the
	// newest fill, so the wrong row here would discount from the wrong price.
	if fills[1].Price != 99 {
		t.Errorf("newest fill price = %v, want the depth-2 price 99", fills[1].Price)
	}
}

// History that arrives out of order still folds correctly: the ladder is
// sorted before it is read.
func TestDepthFillsSortRehydratedHistory(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[1], trade25858[0])

	fills := depthFills(trade)
	if len(fills) != 2 || !fills[0].At.Equal(trade25858[0]) || !fills[1].At.Equal(trade25858[1]) {
		t.Fatalf("fills = %v, want them oldest first", fills)
	}
}

// The step is not the trade's depth: a ladder the gate held at only some of
// its depths has five filled entries and a lower step.
func TestDepthSpacingStepLagsTheLadderDepth(t *testing.T) {
	start := testutil.At("09:00:00")
	pause := start.Add(DepthSpacingBaseHold + DepthSpacingWindow)
	placements := []time.Time{
		start, pause,
		pause.Add(5 * time.Minute), pause.Add(10 * time.Minute), pause.Add(15 * time.Minute),
	}
	rows := []aggragates.TradesLogs{spacingRow(4, pause.Add(11*time.Minute))}

	state := spacingState(placements, rows, pause.Add(16*time.Minute))
	if state.step != 2 {
		t.Fatalf("step = %d, want 2 — two activations on a five-depth ladder", state.step)
	}
}

// A ladder with no rows at all never prints step 0: the row reads step 1.
func TestDepthSpacingRowNeverPrintsStepZero(t *testing.T) {
	start := testutil.At("09:00:00")
	spaced := DepthSpacingBaseHold + time.Minute
	cases := []struct {
		name       string
		placements []time.Time
	}{
		{"fills spaced wider than the hold", []time.Time{start, start.Add(spaced), start.Add(2 * spaced)}},
		{"third fill a day after the second", []time.Time{start, start.Add(spaced), start.Add(spaced + 25*time.Hour)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			trade := hbarTrade(1)
			trade.History = testutil.DepthTrade(c.placements...).History
			event := events.Events{
				Trade:     trade,
				Timestamp: c.placements[len(c.placements)-1].Add(time.Minute).UnixMilli(),
			}
			want := fmt.Sprintf("(depth %d, step 1),", len(c.placements))
			got := DepthSpacingHoldReason(event, "stopLoss")
			if !strings.Contains(got, want) {
				t.Fatalf("row = %q, want it to contain %q", got, want)
			}
		})
	}
}
