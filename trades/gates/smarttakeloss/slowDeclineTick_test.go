package smarttakeloss

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/swingdecline"
)

// The tests of the gate's tick, the three features together: what one tick
// writes when more than one of them has something to say, what a pending ladder
// is judged on, what a hold resets, and which readings latch. The staircase
// (stairBlock) is the swing decline's series; the quiet decline reads the
// verdict and the leg on and quiet the blocks below serve beside it.

// withVerdict is the block with the quiet decline's verdict served beside
// whatever it carries: the verdict, its reasons, and the fill window the quiet
// decline goes pending inside.
func withVerdict(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.SlowDeclineExit = true
	block.SlowDeclineExitReasons = slowDeclineReasons
	block.SlowDeclineFillFrom = fillWindowFrom.UnixMilli()
	return block
}

// withLegOnAndQuiet is the block with the leg on and quiet served, what holds a
// pending ladder's new fill on the quiet decline.
func withLegOnAndQuiet(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.SlowDeclineLegQuiet = true
	block.SlowDeclineExitReasons = slowDeclineReasons
	return block
}

// withBreaks is the block with what sophos names for a reading that missed.
func withBreaks(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.SlowDeclineBreakReasons = slowDeclineBreakReasons
	return block
}

// withoutTheBand is the block with no sell band served: sophos did not read the
// window, so the quiet decline has nothing to say.
func withoutTheBand(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.SlowDeclineSellBand = 0
	return block
}

// withoutTheSeries is the block with no series served.
func withoutTheSeries(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.SwingDeclineOpens, block.SwingDeclineCloses = nil, nil
	return block
}

// fifthFillAt is the stamp of the staircase ladder's fifth fill, inside bar 30.
var fifthFillAt = stairOpen(30).Add(17*time.Minute + 9*time.Second)

// pendingAtFourthWithTheFifth is the staircase ladder pending from its fourth
// fill on the findings' features, a fifth fill landed inside bar 30 and not
// judged yet: no latch is written, and none is needed for the judgement.
func pendingAtFourthWithTheFifth(findings ...Finding) aggragates.Trades {
	return withFill(withRows(stairFourth(), PendingRow(stairPrices[3], findings...)), stairPrices[4], fifthFillAt)
}

// Whichever feature triggers, the gate goes pending from the newest fill on it,
// and one tick writes one row naming every feature that did — the verdict and
// the shape together on the same tick, no longer one after the other; the
// sophos latch among them when it is served on that tick. The quiet decline
// does not latch, the swing decline does.
func TestTheGateGoesPendingOnWhicheverFeaturesTrigger(t *testing.T) {
	both := withVerdict(stairFourthBlock())
	reasons := stairReading(t, stairFourth(), both, true).Reasons
	sophosLatch := withVerdict(stairFourthBlock())
	sophosLatch.SlowDeclineIndecision, sophosLatch.SlowDeclineBreakReasons = true, indecisionReasons
	windowAfterTheFill := withVerdict(stairFourthBlock())
	windowAfterTheFill.SlowDeclineFillFrom = stairOpen(25).UnixMilli()
	for name, tc := range map[string]struct {
		block aggragates.SmartTakeLossIndicators
		want  *Row
		on    []string
	}{
		"both declines": {
			both, row(PendingRow(stairPrices[3], indecisionFinding(nil), quietFinding(slowDeclineReasons), swingFinding(reasons))),
			[]string{FeatureQuietDecline, FeatureSwingDecline},
		},
		"the quiet decline, the series not closed to the fill's bar": {
			withVerdict(stairBlock(stairTurns, 23)), row(PendingRow(stairPrices[3], quietFinding(slowDeclineReasons))),
			[]string{FeatureQuietDecline},
		},
		"the swing decline, no verdict": {
			stairFourthBlock(), row(PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(reasons))),
			[]string{FeatureSwingDecline},
		},
		"the swing decline, the fill window opening after the fill": {
			windowAfterTheFill, row(PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(reasons))),
			[]string{FeatureSwingDecline},
		},
		"both declines and the sophos latch": {
			sophosLatch, row(PendingRow(stairPrices[3], indecisionFinding(indecisionReasons), quietFinding(slowDeclineReasons), swingFinding(reasons))),
			[]string{FeatureQuietDecline, FeatureSwingDecline},
		},
		"neither": {stairBlock(stairTurns, 23), nil, nil},
	} {
		ticked, got := engineTick(stairFourth(), "", underTheBand, stairOpen(26), withBlock(tc.block))
		if tc.want == nil {
			assertUntouched(t, got, "")
			continue
		}
		assertRow(t, got, *tc.want)
		assertNoSale(t, got, "")
		if len(ticked.Logs) != 1 || len(ticked.StrategyEvents) != 1 {
			t.Errorf("%s: one row and one event over the tick, got %d and %d", name, len(ticked.Logs), len(ticked.StrategyEvents))
		}
		if st := rebuildState(ticked); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !reflect.DeepEqual(st.pendingFeatures, tc.on) {
			t.Errorf("%s: pending from the fourth fill on %v, got %+v", name, tc.on, st)
		}
	}
}

// row is the address of a row, for the tables whose expectation is none.
func row(built Row) *Row {
	return &built
}

// The quiet decline never latches the ladder: a ladder it makes pending is not
// latched by that tick or any after it, however it reads — the row of its going
// pending names no indecision, and the take profit it reads is the newest
// fill's alone.
func TestTheQuietDeclineNeverLatches(t *testing.T) {
	block := withVerdict(stairBlock(stairTurns, 23))
	pending, got := engineTick(stairFourth(), "", underTheBand, stairOpen(26), withBlock(block))
	assertRow(t, got, PendingRow(stairPrices[3], quietFinding(slowDeclineReasons)))
	if latchNamed(got) || rebuildState(pending).indecision {
		t.Fatalf("the quiet decline writes no latch, got %+v", got)
	}
	for _, reading := range []aggragates.SmartTakeLossIndicators{block, withLegOnAndQuiet(block), withBreaks(block), withVerdict(stairBlock(stairTurns, 23))} {
		assertUntouched(t, Apply(pending, "", underTheBand, withBlock(reading)), "")
	}
	if rebuildState(pending).indecision {
		t.Fatal("nothing the quiet decline reads latches the ladder")
	}
}

// A ladder pending on either feature, or both, sells at the band on the first
// tick at or over it, under the one reason: the exit row names the decline
// features the ladder is pending on, whichever made it so.
func TestAPendingLadderSellsAtTheBandWhicheverFeatureMadeItPending(t *testing.T) {
	withIndecision(t, false)
	for name, tc := range map[string]struct {
		findings []Finding
		sold     string
	}{
		"the quiet decline": {[]Finding{quietFinding(nil)}, "smartTakeLoss: sell at slow decline bollinger band 187.00 (quiet decline)"},
		"the swing decline": {[]Finding{swingFinding(nil)}, "smartTakeLoss: sell at slow decline bollinger band 187.00 (swing decline)"},
		"both":              {[]Finding{quietFinding(nil), swingFinding(nil)}, "smartTakeLoss: sell at slow decline bollinger band 187.00 (quiet decline and swing decline)"},
	} {
		trade := withRows(stairFourth(), PendingRow(stairPrices[3], tc.findings...))
		block := withBlock(stairFourthBlock())
		assertUntouched(t, Apply(trade, "", underTheBand, block), "")
		got := Apply(trade, "", slowDeclineBand+1, block)
		assertForced(t, got, reasonSellBand)
		if len(got.Rows) != 0 {
			t.Errorf("%s: a pending judged ladder writes no row at the band, got %+v", name, got.Rows)
		}
		if exit := ExitRow(trade, got.Reason, slowDeclineBand+1); exit.Message != tc.sold || exit.Gate != GateSlowDecline || exit.Event != EventSold {
			t.Errorf("%s: the exit row reads %+v, want %q under the slow decline gate", name, exit, tc.sold)
		}
	}
}

// A new fill a pending ladder takes is judged on the readings that apply to it.
// The quiet decline applies on a ladder it watches and answers on a tick that
// serves the band; the swing decline applies while the series is served and
// answers once it has closed the fill's bar. Any reading that answers holding
// CONFIRMS the fill at once, on the features that held; the fill is CANCELLED
// once every reading that applies has answered and none held; otherwise it
// WAITS — no row, and the band sells nothing, the fill being unjudged. The
// indecision direction is switched off beside, so no latch joins these rows.
func TestANewFillIsJudgedOnEveryReadingThatAppliesToIt(t *testing.T) {
	withIndecision(t, false)
	both := []Finding{quietFinding(nil), swingFinding(nil)}
	closed, notClosed, rally := stairFifthBlock(), stairBlock(stairTurns, 29), stairBlock(stairBounce, 30)
	held := stairReading(t, pendingAtFourthWithTheFifth(both...), closed, true).Reasons
	broken := stairReading(t, pendingAtFourthWithTheFifth(both...), rally, false).Breaks
	price5 := stairPrices[4]
	for name, tc := range map[string]struct {
		block aggragates.SmartTakeLossIndicators
		want  *Row
	}{
		"both hold": {withLegOnAndQuiet(closed), row(ConfirmedRow(price5, quietFinding(slowDeclineReasons), swingFinding(held)))},
		"the quiet decline holds at once, the bar open":     {withLegOnAndQuiet(notClosed), row(ConfirmedRow(price5, quietFinding(slowDeclineReasons)))},
		"the quiet decline misses, the swing decline holds": {withBreaks(closed), row(ConfirmedRow(price5, swingFinding(held)))},
		"the quiet decline misses, the bar open: waits":     {withBreaks(notClosed), nil},
		"both miss: cancelled":                              {withBreaks(rally), row(CancelledRow(price5, quietFinding(slowDeclineBreakReasons), swingFinding(broken)))},
		"the quiet decline misses, no series: cancelled":    {withoutTheSeries(withBreaks(closed)), row(CancelledRow(price5, quietFinding(slowDeclineBreakReasons)))},
		"no band, the swing decline misses: waits":          {withoutTheBand(rally), nil},
		"no band, the swing decline holds":                  {withoutTheBand(closed), row(ConfirmedRow(price5, swingFinding(held)))},
		"no band, the bar open: waits":                      {withoutTheBand(notClosed), nil},
		"no band, no series: waits":                         {withoutTheSeries(withoutTheBand(closed)), nil},
	} {
		trade := pendingAtFourthWithTheFifth(both...)
		got := Apply(trade, "", slowDeclineBand+1, withBlock(tc.block))
		if tc.want == nil {
			assertUntouched(t, got, "")
			if st := rebuildState(trade); !slowDeclineFillUnjudged(trade, st) {
				t.Errorf("%s: the fill stays unjudged", name)
			}
			continue
		}
		assertRow(t, got, *tc.want)
		wantSale := tc.want.Event == EventPending && tc.block.SlowDeclineSellBand > 0
		if sold := got.Reason == reasonSellBand; sold != wantSale {
			t.Errorf("%s: the band sells on a confirmation it is served on and on no other tick, got %+v", name, got)
		}
	}
}

// A feature switched off applies to no fill: with the swing decline off the
// quiet decline alone judges, whatever the series would read; with the quiet
// decline off the swing decline does, and a ladder no reading applies to — the
// series not served — waits rather than cancels on nothing.
func TestANewFillIsJudgedOnTheFeaturesThatAreOn(t *testing.T) {
	withIndecision(t, false)
	both := []Finding{quietFinding(nil), swingFinding(nil)}
	price5 := stairPrices[4]

	withSwingDecline(t, false)
	trade := pendingAtFourthWithTheFifth(both...)
	if st := rebuildState(trade); !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) {
		t.Fatalf("fixture drifted: the swing decline is off, the ladder is pending on the quiet decline alone, got %+v", st)
	}
	assertRow(t, Apply(trade, "", slowDeclineBand+1, withBlock(withBreaks(stairFifthBlock()))), CancelledRow(price5, quietFinding(slowDeclineBreakReasons)))
	assertRow(t, Apply(trade, "", slowDeclineBand+1, withBlock(withLegOnAndQuiet(stairBlock(stairBounce, 30)))), ConfirmedRow(price5, quietFinding(slowDeclineReasons)))
	withSwingDecline(t, true)

	withQuietDecline(t, false)
	if st := rebuildState(trade); !reflect.DeepEqual(st.pendingFeatures, []string{FeatureSwingDecline}) {
		t.Fatalf("fixture drifted: the quiet decline is off, the ladder is pending on the swing decline alone, got %+v", st)
	}
	rally := stairBlock(stairBounce, 30)
	assertRow(t, Apply(trade, "", slowDeclineBand+1, withBlock(rally)), CancelledRow(price5, swingFinding(stairReading(t, trade, rally, false).Breaks)))
	closed := stairFifthBlock()
	assertRow(t, Apply(trade, "", slowDeclineBand+1, withBlock(closed)), ConfirmedRow(price5, swingFinding(stairReading(t, trade, closed, true).Reasons)))
	assertUntouched(t, Apply(trade, "", slowDeclineBand+1, withBlock(withoutTheSeries(closed))), "")
	assertUntouched(t, Apply(trade, "", slowDeclineBand+1, withBlock(stairBlock(stairTurns, 29))), "")
}

// The fill that takes the ladder to its last depth is judged by no reading,
// whatever each would answer, and the band sells: the ladder stays pending from
// where it was on the features it was pending on.
func TestTheLastDepthFillIsJudgedByNoReading(t *testing.T) {
	withIndecision(t, false)
	seventh := withRows(stairLadder(0, 9, 15, 24, 30, 36, 42), PendingRow(stairPrices[6], quietFinding(nil), swingFinding(nil)))
	eighth := withFill(seventh, stairPrices[7], stairOpen(48).Add(time.Minute))
	for name, block := range map[string]aggragates.SmartTakeLossIndicators{
		"the quiet decline holds":   withLegOnAndQuiet(stairBlock(stairDecline, 48)),
		"both miss":                 withBreaks(stairBlock([]float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200, 210, 220, 230, 240, 250, 260}, 48)),
		"no series, the band alone": {SlowDeclineSellBand: slowDeclineBand},
	} {
		assertUntouched(t, Apply(eighth, "", underTheBand, withBlock(block)), "")
		sold := Apply(eighth, "", slowDeclineBand+1, withBlock(block))
		assertForced(t, sold, reasonSellBand)
		if len(sold.Rows) != 0 {
			t.Errorf("%s: judged by no reading, got %+v", name, sold.Rows)
		}
	}
}

// A hold resets a pending gate on its first held tick whichever feature made it
// pending — the quiet decline, the swing decline or both — with one row naming
// those features and no reason, at the newest fill's price, and the ladder reads
// not pending from then on. Nothing else is written while the hold lasts,
// whatever the readings serve, and no sale is made at the band.
func TestAHeldGateIsResetOnceWhicheverFeatureMadeItPending(t *testing.T) {
	for name, findings := range map[string][]Finding{
		"the quiet decline": {quietFinding(nil)},
		"the swing decline": {swingFinding(nil)},
		"both":              {quietFinding(nil), swingFinding(nil)},
	} {
		held := heldBy(withRows(stairFourth(), PendingRow(stairPrices[3], findings...)), stairOpen(25))
		block := withVerdict(stairFourthBlock())
		ticked, got := engineTick(held, "", slowDeclineBand+1, stairOpen(26), withBlock(block))
		assertRow(t, got, ResetRow(stairPrices[3], findings...))
		assertNoSale(t, got, "")
		if st := rebuildState(ticked); st.slowDeclinePending || len(st.pendingFeatures) != 0 || !st.depthPriorityHeld {
			t.Errorf("%s: reset, held and not pending, got %+v", name, st)
		}
		for _, reading := range []aggragates.SmartTakeLossIndicators{block, withLegOnAndQuiet(block), withBreaks(block)} {
			assertUntouched(t, Apply(ticked, "", slowDeclineBand+1, withBlock(reading)), "")
		}
		if resets := carriedEvents(ticked, GateSlowDecline, EventReset); resets != 1 || len(ticked.Logs) != len(held.Logs)+1 {
			t.Errorf("%s: one reset row and one reset event, got %d events and %d new rows", name, resets, len(ticked.Logs)-len(held.Logs))
		}
	}

	// A held ladder that is not pending triggers on nothing: both declines read
	// for it, and the hold pauses them both.
	heldNotPending := heldBy(stairFourth(), stairOpen(25))
	assertUntouched(t, Apply(heldNotPending, "", slowDeclineBand+1, withBlock(withVerdict(stairFourthBlock()))), "")
}

// The next fill ends the hold, and the gate goes pending again only by a
// trigger: the quiet decline's, or the swing decline's, read on both for the
// ladder the reset left not pending.
func TestTheNextFillAfterTheResetGoesPendingOnATrigger(t *testing.T) {
	held := heldBy(withRows(stairFourth(), PendingRow(stairPrices[3], quietFinding(nil), swingFinding(nil))), stairOpen(25))
	reset, _ := engineTick(held, "", underTheBand, stairOpen(26), withBlock(stairFourthBlock()))
	sixth := withFill(reset, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	decline := stairBlock(stairDecline, 36)
	assertUntouched(t, Apply(sixth, "", underTheBand, withBlock(withoutTheSeries(decline))), "")
	reasons := stairReading(t, sixth, decline, true).Reasons
	assertRow(t, Apply(sixth, "", underTheBand, withBlock(decline)), PendingRow(stairPrices[5], indecisionFinding(nil), swingFinding(reasons)))
	assertRow(t, Apply(sixth, "", underTheBand, withBlock(withVerdict(withoutTheSeries(decline)))), PendingRow(stairPrices[5], quietFinding(slowDeclineReasons)))
}

// The swing decline latches a ladder that is already pending — the quiet
// decline made it pending, or the swing decline was off or unread then — once
// it reads for the ladder's judged newest fill inside the read bars of that
// fill's bar: one row of its own, the latch with the swing decline's reasons, an
// event of the latched kind carrying the indecision alone. The ladder stays
// pending on what it was pending on. Past the read bars, with the series not
// served, from the quiet decline's reading and with the indecision off, nothing
// is written; while a hold lasts the row is the reset alone; and once latched,
// never again.
func TestTheSwingDeclineLatchesAnAlreadyPendingLadderInsideTheReadBars(t *testing.T) {
	pending := withRows(stairFourth(), PendingRow(stairPrices[3], quietFinding(slowDeclineReasons)))
	block := stairFourthBlock()
	reading := stairReading(t, pending, block, true)

	latched, got := engineTick(pending, "", underTheBand, stairOpen(26), withBlock(block))
	assertRow(t, got, LatchedRow(stairPrices[3], swingFinding(reading.Reasons)))
	if got.Rows[0].Event != EventLatched || !reflect.DeepEqual(got.Rows[0].Features, []string{FeatureIndecision}) || !slices.Equal(got.Rows[0].Reasons, reading.Reasons) {
		t.Errorf("the row is of the latched kind, carries the indecision alone and names the swing decline's reasons, got %+v", got.Rows[0])
	}
	if st := rebuildState(latched); !st.indecision || !st.slowDeclinePending || !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) || st.slowDeclinePendingFrom != stairPrices[3] {
		t.Errorf("latched, and still pending from the fourth fill on the quiet decline, got %+v", st)
	}
	assertUntouched(t, Apply(latched, "", underTheBand, withBlock(block)), "")

	turns := append(slices.Clone(stairTurns), 84, 83, 82, 81)
	for lastBar, writes := range map[int]bool{
		24:                                     true,
		24 + swingdecline.SwingDeclineReadBars: true,
		24 + swingdecline.SwingDeclineReadBars + 1: false,
	} {
		got := Apply(pending, "", underTheBand, withBlock(stairBlock(turns, lastBar)))
		if (len(got.Rows) == 1 && got.Rows[0].Event == EventLatched) != writes || len(got.Rows) > 1 {
			t.Errorf("series to bar %d: latch row %v, want %v (%+v)", lastBar, len(got.Rows) == 1, writes, got.Rows)
		}
	}
	assertUntouched(t, Apply(pending, "", underTheBand, withBlock(withoutTheSeries(block))), "")
	assertUntouched(t, Apply(pending, "", underTheBand, withBlock(stairBlock(stairTurns, 23))), "")
	verdictOnly := withVerdict(stairBlock(stairTurns, 23))
	assertUntouched(t, Apply(pending, "", underTheBand, withBlock(verdictOnly)), "")

	withIndecision(t, false)
	assertUntouched(t, Apply(pending, "", underTheBand, withBlock(block)), "")
	withIndecision(t, true)
	assertRow(t, Apply(heldBy(pending, stairOpen(25)), "", underTheBand, withBlock(block)), ResetRow(stairPrices[3], quietFinding(nil)))
}

// The three features read on their own switches: with the quiet decline off the
// swing decline triggers alone and latches, with the swing decline off the quiet
// decline triggers alone and nothing latches beside it, with both off the gate
// triggers nothing and the indecision direction, on its own switch, still latches
// on the sophos reading.
func TestTheFeaturesTriggerOnTheirOwnSwitches(t *testing.T) {
	block := withVerdict(stairFourthBlock())
	reasons := stairReading(t, stairFourth(), block, true).Reasons

	withQuietDecline(t, false)
	assertRow(t, Apply(stairFourth(), "", underTheBand, withBlock(block)), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(reasons)))
	withQuietDecline(t, true)

	withSwingDecline(t, false)
	assertRow(t, Apply(stairFourth(), "", underTheBand, withBlock(block)), PendingRow(stairPrices[3], quietFinding(slowDeclineReasons)))

	withQuietDecline(t, false)
	assertUntouched(t, Apply(stairFourth(), "", underTheBand, withBlock(block)), "")
	assertRow(t, Apply(stairFourth(), "", underTheBand, indecisionReading()), LatchedRow(stairPrices[3], indecisionFinding(indecisionReasons)))
}

// A ladder's life on the gate, tick by tick, one row and one event each: both
// declines trigger on one tick with the latch beside them, the fifth fill is
// confirmed on both readings, the sixth is cancelled on both, and the seventh
// goes pending again on the verdict, and on the shape when it reads. The rows
// and the events written are the ones Apply handed back, in order.
func TestAGatesLifeIsOneRowAndOneEventAtATime(t *testing.T) {
	trade := stairFourth()
	var want []Row
	tick := func(block aggragates.SmartTakeLossIndicators, at time.Time, row Row) {
		t.Helper()
		var got Result
		trade, got = engineTick(trade, "", underTheBand, at, withBlock(block))
		assertRow(t, got, row)
		want = append(want, row)
		assertNoSale(t, got, "")
	}

	first := withVerdict(stairFourthBlock())
	tick(first, stairOpen(26), PendingRow(stairPrices[3], indecisionFinding(nil), quietFinding(slowDeclineReasons), swingFinding(stairReading(t, trade, first, true).Reasons)))

	trade = withFill(trade, stairPrices[4], fifthFillAt)
	second := withLegOnAndQuiet(stairFifthBlock())
	tick(second, stairOpen(31), ConfirmedRow(stairPrices[4], quietFinding(slowDeclineReasons), swingFinding(stairReading(t, trade, second, true).Reasons)))

	trade = withFill(trade, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	third := withBreaks(stairBlock(stairRally, 36))
	tick(third, stairOpen(37), CancelledRow(stairPrices[5], quietFinding(slowDeclineBreakReasons), swingFinding(stairReading(t, trade, third, false).Breaks)))
	if st := rebuildState(trade); st.slowDeclinePending || !st.indecision {
		t.Fatalf("cancelled on both readings and latched, got %+v", st)
	}

	trade = withFill(trade, stairPrices[6], stairOpen(42).Add(17*time.Minute))
	fourth := withVerdict(stairBlock(stairDecline, 42))
	findings := []Finding{quietFinding(slowDeclineReasons)}
	if reading, holds := swingdecline.Trigger(swingFills(rebuildState(trade).fills), swingdecline.SeriesOf(fourth), swingdecline.TrendOf(fourth)); holds {
		findings = append(findings, swingFinding(reading.Reasons))
	}
	tick(fourth, stairOpen(43), PendingRow(stairPrices[6], findings...))

	messages, events := expectedRows(want...)
	if got := writtenRows(trade); !reflect.DeepEqual(got, messages) {
		t.Errorf("rows %v, want %v", got, messages)
	}
	if got := writtenEvents(t, trade); !reflect.DeepEqual(got, events) {
		t.Errorf("events %v, want the rows' %v", got, events)
	}
	if len(trade.Logs) != len(want) || len(trade.StrategyEvents) != len(want) {
		t.Errorf("one row and one event for every tick, got %d and %d for %d", len(trade.Logs), len(trade.StrategyEvents), len(want))
	}
}
