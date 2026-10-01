package cooldown

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

var trade25858 = testutil.Trade25858()

// spacingEvent is the event the gate leaves behind when it holds a depth: the
// depth-spacing event gates.SaveHoldLog writes beside the row, stamped at. The
// step it carries is deliberately wrong: the rule derives the step and must
// never read it back.
func spacingEvent(depth int, at time.Time) aggragates.TradesStrategyEvents {
	data := DepthSpacingEvent{Event: gates.EventHeld, Depth: depth, Step: 99, Hold: time.Hour}

	return NewDepthSpacingEvent(0, data, at)
}

// spacingState folds a ladder placed at the given stamps, with the given
// depth-spacing events already written, at the tick `now`.
func spacingState(placements []time.Time, written []aggragates.TradesStrategyEvents, now time.Time) depthSpacingState {
	return depthSpacingEligibleFrom(written, depthFills(testutil.DepthTrade(placements...)), now)
}

// tickOf is the event the engine hands the gate at the tick, with the updateTrade
// stand-in SaveHoldLog calls.
func tickOf(trade aggragates.Trades, at time.Time) events.Events {
	return events.Events{
		Trade:     trade,
		Timestamp: at.UnixMilli(),
		Events:    map[string]func(events.Events) (events.Events, error){"updateTrade": testutil.NopUpdateTrade},
	}
}

// heldAt runs the gate on the trade at the tick and, as it must refuse, writes
// its row and its event through gates.SaveHoldLog, the path production takes.
// It returns the trade as the writer left it and the hold the gate answered.
func heldAt(t *testing.T, trade aggragates.Trades, at time.Time) (aggragates.Trades, gates.Hold) {
	t.Helper()

	event := tickOf(trade, at)
	hold := DepthSpacingHold(event, "stopLoss")
	if !hold.Held() {
		t.Fatalf("the gate let the depth through at %s, want it parked", at)
	}
	written, err := gates.SaveHoldLog(event, "stopLoss", hold)
	if !errors.Is(err, events.ErrTradeHeld) {
		t.Fatalf("SaveHoldLog = %v, want the chain stopped by the hold", err)
	}

	return written.Trade, hold
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
	written := []aggragates.TradesStrategyEvents{
		spacingEvent(2, at(5, "14:40:00")),
		spacingEvent(3, at(5, "18:24:00")),
		spacingEvent(6, at(7, "22:51:00")),
	}

	// The fixture is only this shape while the constants keep it so.
	expiry2 := placements[1].Add(depthSpacingHoldFor(1))
	if gap := written[1].CreatedAt.Sub(expiry2); gap >= DepthSpacingWindow {
		t.Fatalf("depth 3 activated %s past the expiry, the fixture needs it inside %s", gap, DepthSpacingWindow)
	}
	expiry3 := placements[2].Add(depthSpacingHoldFor(2))
	if gap := written[2].CreatedAt.Sub(expiry3); gap < DepthSpacingWindow {
		t.Fatalf("depth 6 activated %s past the expiry, the fixture needs it at %s or more", gap, DepthSpacingWindow)
	}

	cases := []struct {
		depth    int
		now      time.Time
		wantStep int
	}{
		{2, written[0].CreatedAt, 1},
		{3, written[1].CreatedAt, 2},
		{6, written[2].CreatedAt, 1},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("depth %d", c.depth), func(t *testing.T) {
			state := spacingState(placements[:c.depth], written, c.now)
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

// An event written again for the same depth is gates.SaveHoldLog writing the
// standing hold once its row is a day old: one activation, at the earliest
// stamp, however the events are ordered. The re-log here lands a whole window
// past the previous expiry, so taking it for the activation would reset the
// count the first event keeps going.
func TestDepthSpacingCountsAReLoggedDepthOnce(t *testing.T) {
	start := testutil.At("09:00:00")
	placements := []time.Time{start, start.Add(10 * time.Minute)}
	previous := spacingEvent(1, start.Add(time.Minute))
	first := start.Add(11 * time.Minute)
	relog := first.Add(DepthSpacingBaseHold + DepthSpacingWindow)

	cases := []struct {
		name    string
		written []aggragates.TradesStrategyEvents
	}{
		{"once", []aggragates.TradesStrategyEvents{previous, spacingEvent(2, first)}},
		{"relogged", []aggragates.TradesStrategyEvents{
			previous, spacingEvent(2, first), spacingEvent(2, relog), spacingEvent(2, relog.Add(24*time.Hour)),
		}},
		{"newest first", []aggragates.TradesStrategyEvents{previous, spacingEvent(2, relog), spacingEvent(2, first)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state := spacingState(placements, c.written, relog)
			if state.step != 2 {
				t.Fatalf("step = %d, want 2 — depth 2 counts once, at its first event", state.step)
			}
			if got := depthSpacingActivations(c.written)[2]; !got.Equal(first) {
				t.Fatalf("activation = %s, want the earliest event %s", got, first)
			}
		})
	}
}

// The first held tick has no event yet and already reports its step: 1 on a
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
	written := []aggragates.TradesStrategyEvents{spacingEvent(1, start.Add(time.Minute))}
	if got := spacingState(placements, written, now).step; got != 2 {
		t.Errorf("previous activation in the window: step = %d, want 2", got)
	}
}

// An activation DepthSpacingWindow or more past the previous expiry starts
// over at step 1; one second short of it keeps counting.
func TestDepthSpacingResetsAfterTheWindow(t *testing.T) {
	start := testutil.At("09:00:00")
	activated := start.Add(time.Minute)
	expiry := start.Add(depthSpacingHoldFor(1))
	written := []aggragates.TradesStrategyEvents{spacingEvent(1, activated)}

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
			state := spacingState(placements, written, c.now)
			if state.step != c.wantStep {
				t.Fatalf("step = %d, want %d", state.step, c.wantStep)
			}
			if state.hold != depthSpacingHoldFor(c.wantStep) {
				t.Errorf("hold = %s, want %s", state.hold, depthSpacingHoldFor(c.wantStep))
			}
		})
	}
}

// Events with no stamp, no readable document, no depth or a kind the fold does
// not know are skipped, and a ladder with an unknown clock never holds at all.
func TestDepthSpacingSkipsUnreadableEventsAndFailsOpenOnUnknownClocks(t *testing.T) {
	start := testutil.At("09:00:00")
	placements := []time.Time{start, start.Add(time.Hour)}
	now := start.Add(time.Hour + time.Minute)
	stamp := start.Add(time.Minute)

	filed := func(data []byte, at time.Time) aggragates.TradesStrategyEvents {
		return aggragates.TradesStrategyEvents{
			Param:     aggragates.StrategyParamCooldown,
			Gate:      GateDepthSpacing,
			Data:      data,
			CreatedAt: at,
		}
	}
	unreadable := map[string]aggragates.TradesStrategyEvents{
		"no stamp":          spacingEvent(1, time.Time{}),
		"a torn document":   filed([]byte("{not json"), stamp),
		"no document":       filed(nil, stamp),
		"an empty document": aggragates.NewStrategyEvent(0, aggragates.StrategyParamCooldown, GateDepthSpacing, math.NaN(), stamp),
		"no depth":          NewDepthSpacingEvent(0, DepthSpacingEvent{Event: gates.EventHeld}, stamp),
		"a negative depth":  spacingEvent(-1, stamp),
		"an unknown kind":   NewDepthSpacingEvent(0, DepthSpacingEvent{Event: "released", Depth: 1}, stamp),
	}
	for name, event := range unreadable {
		written := []aggragates.TradesStrategyEvents{event}
		if got := spacingState(placements, written, now).step; got != 1 {
			t.Errorf("%s: step = %d, want 1 — the event carries nothing to count", name, got)
		}
	}
	if got := spacingState(placements, []aggragates.TradesStrategyEvents{spacingEvent(1, stamp)}, now).step; got != 2 {
		t.Fatalf("control: step = %d, want 2 — the readable event is counted", got)
	}

	unknown := testutil.DepthTrade(placements...)
	unknown.History[1].CreatedAt = time.Time{}
	if state := depthSpacingEligibleFrom(nil, depthFills(unknown), now); !state.eligibleFrom.IsZero() {
		t.Errorf("an unstamped fill must leave the gate open, got %s", state.eligibleFrom)
	}
	event := events.Events{Trade: unknown}
	if got := DepthSpacingHold(event, "stopLoss").Reason; got != "" {
		t.Errorf("no tick clock must never hold, got %q", got)
	}
}

// The events are the state and the rows are text: a hold row is never read
// back. A trade whose only record of a depth's hold is its row has no
// activation at that depth, so the next depth starts its cascade over; the
// same trade carrying the event beside the row escalates.
func TestDepthSpacingReadsTheEventsNeverTheRowText(t *testing.T) {
	start := testutil.At("09:00:00")
	activated := start.Add(time.Minute)
	tick := start.Add(time.Hour + time.Minute)

	trade := hbarTrade(1)
	trade.History = testutil.DepthTrade(start, start.Add(time.Hour)).History
	previous := DepthSpacingEvent{Event: gates.EventHeld, Depth: 1, Step: 1, Hold: DepthSpacingBaseHold}
	row := aggragates.TradesLogs{
		Message:   "Hold stopLoss: " + depthSpacingHoldMessage(previous),
		Type:      aggragates.LOG_INFO,
		CreatedAt: activated,
	}

	textOnly := trade
	textOnly.Logs = []aggragates.TradesLogs{row}
	paired := aggragates.AppendStrategyRow(trade, row, NewDepthSpacingEvent(0, previous, activated))

	stepOf := func(trade aggragates.Trades) int {
		data, ok := DepthSpacingHold(events.Events{Trade: trade, Timestamp: tick.UnixMilli()}, "stopLoss").Data.(DepthSpacingEvent)
		if !ok {
			t.Fatal("the gate must hold the second depth")
		}
		return data.Step
	}
	if got := stepOf(textOnly); got != 1 {
		t.Errorf("a hold row without its event: step = %d, want 1 — no activation at depth 1", got)
	}
	if got := stepOf(paired); got != 2 {
		t.Errorf("the row with its event: step = %d, want 2", got)
	}
}

// Only the depth-spacing events of the cooldown flag are activations. The
// depth priority's document carries a depth of its own, and a depth-spacing
// document filed under another gate or another flag is not this gate's.
func TestDepthSpacingActivationsOnlyReadTheDepthSpacingEvents(t *testing.T) {
	at := testutil.At("09:00:00")
	hold := DepthSpacingEvent{Event: gates.EventHeld, Depth: 4}
	priority := DepthPriorityEvent{Event: gates.EventHeld, PrioritySymbol: "LINK/USDT", PriorityDepth: 5, PriorityMaxDepth: 8, Depth: 1, MaxDepth: 8}
	written := []aggragates.TradesStrategyEvents{
		NewDepthPriorityEvent(0, priority, at.Add(-time.Hour)),
		NewFirstFillEvent(0, FirstFillEvent{Event: FirstFillArmed, Price: 101}, at.Add(-time.Hour)),
		aggragates.NewStrategyEvent(0, aggragates.StrategyParamSmartTakeLoss, GateDepthSpacing, hold, at.Add(-time.Hour)),
		aggragates.NewStrategyEvent(0, aggragates.StrategyParamCooldown, GateDepthPriority, hold, at.Add(-time.Hour)),
		spacingEvent(3, at),
	}

	got := depthSpacingActivations(written)
	if len(got) != 1 || !got[3].Equal(at) {
		t.Fatalf("activations = %v, want depth 3 at %s alone", got, at)
	}
}

// The step keeps counting past the hold cap; only the hold is capped.
func TestDepthSpacingStepCountsPastTheHoldCap(t *testing.T) {
	start := testutil.At("09:00:00")
	placements := []time.Time{start}
	written := []aggragates.TradesStrategyEvents{spacingEvent(1, start.Add(time.Minute))}
	state := spacingState(placements, written, start.Add(time.Minute))

	const depths = 12
	for depth := 2; depth <= depths; depth++ {
		fill := state.eligibleFrom
		placements = append(placements, fill)
		written = append(written, spacingEvent(depth, fill.Add(time.Minute)))
		state = spacingState(placements, written, fill.Add(time.Minute))
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

// The message stays byte-identical across ticks once the hold's row and event
// exist, and the tick that writes them already printed the same string: the
// standing hold then collapses in gates.SaveHoldLog and writes nothing more.
func TestDepthSpacingMessageIsStableOnceTheHoldIsWritten(t *testing.T) {
	start := testutil.At("09:00:00")
	trade := hbarTrade(1)
	trade.History = testutil.DepthTrade(start, start.Add(time.Hour)).History
	trade.StrategyEvents = []aggragates.TradesStrategyEvents{spacingEvent(1, start.Add(time.Minute))}

	written, first := heldAt(t, trade, start.Add(time.Hour+time.Minute))
	if !strings.Contains(first.Reason, "(depth 2, step 2)") {
		t.Fatalf("first held tick = %q, want depth 2 step 2", first.Reason)
	}
	if len(written.Logs) != 1 || len(written.StrategyEvents) != 2 {
		t.Fatalf("the first held tick wrote %d rows and %d events, want its one pair beside the seeded event", len(written.Logs), len(written.StrategyEvents))
	}

	for _, later := range []time.Duration{2 * time.Minute, 30 * time.Minute, 2 * time.Hour} {
		event := tickOf(written, start.Add(time.Hour+later))
		again := DepthSpacingHold(event, "stopLoss")
		if again.Reason != first.Reason {
			t.Fatalf("after %s:\n got %q\nwant %q", later, again.Reason, first.Reason)
		}
		collapsed, err := gates.SaveHoldLog(event, "stopLoss", again)
		if !errors.Is(err, events.ErrHoldNotPersisted) {
			t.Fatalf("after %s: SaveHoldLog = %v, want the standing hold collapsed", later, err)
		}
		if len(collapsed.Trade.Logs) != 1 || len(collapsed.Trade.StrategyEvents) != 2 {
			t.Fatalf("after %s: %d rows and %d events, want nothing written", later, len(collapsed.Trade.Logs), len(collapsed.Trade.StrategyEvents))
		}
	}
}

// The cascade end to end on what gates.SaveHoldLog wrote: depth 1 is held on
// the first tick after its fill, depth 2 fills the instant that hold lifts and
// is held in turn, so its event is the second activation and escalates.
func TestDepthSpacingEscalatesOnTheActivationsSaveHoldLogWrote(t *testing.T) {
	first := testutil.At("09:00:00")
	expiry := first.Add(depthSpacingHoldFor(1))
	trade := hbarTrade(1)
	trade.History = testutil.DepthTrade(first).History

	held, one := heldAt(t, trade, first.Add(time.Minute))
	if !strings.Contains(one.Reason, "(depth 1, step 1)") {
		t.Fatalf("depth 1 held as %q, want step 1", one.Reason)
	}

	held.History = testutil.DepthTrade(first, expiry).History
	held, two := heldAt(t, held, expiry.Add(time.Minute))
	if !strings.Contains(two.Reason, "(depth 2, step 2)") {
		t.Fatalf("depth 2 held as %q, want step 2", two.Reason)
	}

	// Each hold left its event on the tick it was written: the activations.
	activations := depthSpacingActivations(held.StrategyEvents)
	if len(activations) != 2 || !activations[1].Equal(first.Add(time.Minute)) || !activations[2].Equal(expiry.Add(time.Minute)) {
		t.Fatalf("activations = %v, want depth 1 and depth 2 at the ticks they were held on", activations)
	}
}

// A ladder on its first activation still parks its next depth, for the base
// hold and the same release price the first cascade level asks for. The row
// prints step 1: the tick is folded in as the activation it is, so the step
// is never below it.
func TestDepthSpacingAFirstActivationParksForTheBaseHoldAndPriceReleases(t *testing.T) {
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
	got := DepthSpacingHold(held, "stopLoss").Reason
	if got != want {
		t.Fatalf("row = %q, want %q", got, want)
	}
	if again := DepthSpacingHold(held, "stopLoss").Reason; again != got {
		t.Fatalf("the row must be byte-stable while the hold stands: %q then %q", got, again)
	}

	paid := held
	paid.Trade.PositionPrice = release
	if got := DepthSpacingHold(paid, "stopLoss").Reason; got != "" {
		t.Fatalf("at the release price the hold must lift, got %q", got)
	}

	expired := events.Events{Trade: trade, Timestamp: trade25858[0].Add(DepthSpacingBaseHold).UnixMilli()}
	if got := DepthSpacingHold(expired, "stopLoss").Reason; got != "" {
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
	written := []aggragates.TradesStrategyEvents{spacingEvent(4, pause.Add(11*time.Minute))}

	state := spacingState(placements, written, pause.Add(16*time.Minute))
	if state.step != 2 {
		t.Fatalf("step = %d, want 2 — two activations on a five-depth ladder", state.step)
	}
}

// A ladder with no events at all never prints step 0: the row reads step 1.
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
			got := DepthSpacingHold(event, "stopLoss").Reason
			if !strings.Contains(got, want) {
				t.Fatalf("row = %q, want it to contain %q", got, want)
			}
		})
	}
}

// The hold hands gates.SaveHoldLog the depth spacing event that goes beside
// its row: the ladder's depth, the escalation step and the hold that step
// earned, and the release price only when the ladder's rows could price one.
// The expectations come from the fold and the release rule, which have their
// own tests, so no calibration value is restated here. The event the writer
// stores is the very one NewDepthSpacingEvent builds.
func TestDepthSpacingHoldCarriesItsEvent(t *testing.T) {
	first := testutil.At("09:00:00")
	second := first.Add(5 * time.Minute)
	tick := second.Add(time.Minute)
	trade := testutil.DepthTrade(first, second)
	trade.ID = 21
	trade.PositionPrice = trade.History[1].Price
	state := spacingState([]time.Time{first, second}, nil, tick)
	release, priced := depthSpacingReleasePrice(trade, trade.History[1].Price, state.step)
	if !priced {
		t.Fatal("fixture drifted: the fixture ladder row must price a release")
	}
	event := events.Events{Trade: trade, Timestamp: tick.UnixMilli()}

	hold := DepthSpacingHold(event, "stopLoss")
	data := DepthSpacingEvent{Event: gates.EventHeld, Depth: 2, Step: state.step, Hold: state.hold, Release: release}
	if hold.Param != aggragates.StrategyParamCooldown || hold.Gate != GateDepthSpacing || hold.Data != data {
		t.Fatalf("hold names %q/%q with %+v, want the cooldown depthSpacing event %+v", hold.Param, hold.Gate, hold.Data, data)
	}
	want := fmt.Sprintf("cooldown: depths too close (depth 2, step %d), next add parked for %s or until %s",
		state.step, state.hold, strconv.FormatFloat(release, 'f', -1, 64))
	if hold.Reason != want {
		t.Fatalf("reason = %q, want %q", hold.Reason, want)
	}

	// Without a ladder row there is no release price to name: the event
	// omits it and the message stops at the wait.
	unpriced := trade
	unpriced.StrategyPair.StrategySettings = nil
	hold = DepthSpacingHold(events.Events{Trade: unpriced, Timestamp: tick.UnixMilli()}, "stopLoss")
	data.Release = 0
	if hold.Data != data {
		t.Fatalf("data = %+v, want the event without a release %+v", hold.Data, data)
	}
	want = fmt.Sprintf("cooldown: depths too close (depth 2, step %d), next add parked for %s", state.step, state.hold)
	if hold.Reason != want {
		t.Fatalf("reason = %q, want %q", hold.Reason, want)
	}

	event.Events = map[string]func(events.Events) (events.Events, error){"updateTrade": testutil.NopUpdateTrade}
	written, err := gates.SaveHoldLog(event, "stopLoss", DepthSpacingHold(event, "stopLoss"))
	if err == nil {
		t.Fatal("a depth spacing hold must stop the chain")
	}
	data.Release = release
	stored := []aggragates.TradesStrategyEvents{NewDepthSpacingEvent(21, data, tick)}
	if !reflect.DeepEqual(written.Trade.StrategyEvents, stored) {
		t.Fatalf("events = %+v, want %+v", written.Trade.StrategyEvents, stored)
	}
	if len(written.Trade.Logs) != 1 || !written.Trade.Logs[0].CreatedAt.Equal(tick) {
		t.Fatalf("rows = %+v, want the one row carrying the event's stamp", written.Trade.Logs)
	}
}
