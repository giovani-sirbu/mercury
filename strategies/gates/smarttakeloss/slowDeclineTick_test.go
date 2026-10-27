package smarttakeloss

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
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
	block.QuietDeclineVerdict = true
	block.QuietDeclineExitReasons = slowDeclineReasons
	block.QuietDeclineFillFrom = fillWindowFrom.UnixMilli()
	return block
}

// withLegOnAndQuiet is the block with the leg on and quiet served, what holds a
// pending ladder's new fill on the quiet decline, and the fill window that marks
// the window as read.
func withLegOnAndQuiet(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.QuietDeclineLegQuiet = true
	block.QuietDeclineExitReasons = slowDeclineReasons
	block.QuietDeclineFillFrom = fillWindowFrom.UnixMilli()
	return block
}

// withBreaks is the block with what sophos names for a reading that missed, and
// the fill window that marks the window as read.
func withBreaks(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.QuietDeclineBreakReasons = quietDeclineBreakReasons
	block.QuietDeclineFillFrom = fillWindowFrom.UnixMilli()
	return block
}

// withoutTheWindow is the block with no fill window served: sophos did not read
// its window, so the quiet decline's reading of a new fill cannot be taken and
// it has nothing to say.
func withoutTheWindow(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.QuietDeclineFillFrom = 0
	return block
}

// withoutTheSeries is the block with no series served.
func withoutTheSeries(block aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators {
	block.SeriesOpens, block.SeriesCloses, block.SeriesHighs, block.SeriesLows = nil, nil, nil, nil
	return block
}

// stairBouncedBlock is the staircase closed to bar 30 with its last leg a rally:
// the swing decline reads no window of the fourth fill as holding as of that bar,
// and so reads nothing for a ladder on the quiet decline's tick.
func stairBouncedBlock() aggragates.SmartTakeLossIndicators {
	return stairBlock(stairBounce, 30)
}

// fifthFillAt is the stamp of the staircase ladder's fifth fill, inside bar 30.
var fifthFillAt = stairOpen(30).Add(17*time.Minute + 9*time.Second)

// pendingAtFourthWithTheFifth is the staircase ladder pending from its fourth
// fill on the findings' features, a fifth fill landed inside bar 30 and not
// judged yet: no latch is written, and none is needed for the judgement.
func pendingAtFourthWithTheFifth(findings ...Finding) aggragates.Trades {
	return withFill(withRows(stairFourth(), PendingRow(stairPrices[3], findings...)), stairPrices[4], fifthFillAt)
}

// Whichever decline triggers, the gate goes pending from the newest fill on it,
// one decline at a time: the first of the declines watching the ladder that
// triggers (firstDecline: the quiet decline, then the swing decline), so that
// the verdict and the shape reading on the same tick make the ladder pending on
// the verdict alone, and the row names that one decline; the sophos latch beside
// it when it is served on that tick. The quiet decline does not latch, the swing
// decline does. The tick runs at a print over the ladder's break even
// (gainPrint) with a buy order resting, where the ladder in buy is activated
// nothing and the gate's row stands alone.
func TestTheGateGoesPendingOnWhicheverFeaturesTrigger(t *testing.T) {
	withLinearDecline(t, false)
	both := withVerdict(stairFourthBlock())
	reasons := stairReading(t, stairFourth(), both, true).Reasons
	sophosLatch := withVerdict(stairFourthBlock())
	sophosLatch.Indecision, sophosLatch.QuietDeclineBreakReasons = true, indecisionReasons
	windowAfterTheFill := withVerdict(stairFourthBlock())
	windowAfterTheFill.QuietDeclineFillFrom = stairOpen(25).UnixMilli()
	for name, tc := range map[string]struct {
		block aggragates.SmartTakeLossIndicators
		want  *Row
		on    []string
	}{
		"both declines read, the quiet decline first": {
			both, row(PendingRow(stairPrices[3], quietFinding(slowDeclineReasons))),
			[]string{FeatureQuietDecline},
		},
		"the quiet decline, a series whose last bar breaks the window": {
			withVerdict(stairBouncedBlock()), row(PendingRow(stairPrices[3], quietFinding(slowDeclineReasons))),
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
		"both declines read and the sophos latch, the quiet decline first": {
			sophosLatch, row(PendingRow(stairPrices[3], indecisionFinding(indecisionReasons), quietFinding(slowDeclineReasons))),
			[]string{FeatureQuietDecline},
		},
		"neither": {stairBouncedBlock(), nil, nil},
	} {
		ticked, got := tickAtAGain(stairFourth(), "", stairOpen(26), withBlock(tc.block))
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

// When the quiet decline wins the tick's row, the early take profit's latch on the swing reading does
// not join it: the quiet decline does not latch, and a tick that wrote a row carries the swing reading
// no further. The latch lands on the NEXT tick as a row of its own, the swing decline's reasons on the
// latched kind, at the newest fill's price, and the ladder stays pending on the quiet decline alone; a
// tick after that writes nothing, each tick one row and one event. The ticks run at a print over the
// ladder's break even (gainPrint) with a buy order resting, where the ladder in buy is activated
// nothing and the gate's row stands alone.
func TestTheSwingLatchOfAQuietLadderLandsOnTheNextTickAsARowOfItsOwn(t *testing.T) {
	withLinearDecline(t, false)
	block := withVerdict(stairFourthBlock())
	reasons := stairReading(t, stairFourth(), block, true).Reasons

	pending, first := tickAtAGain(stairFourth(), "", stairOpen(26), withBlock(block))
	assertRow(t, first, PendingRow(stairPrices[3], quietFinding(slowDeclineReasons)))
	if st := rebuildState(pending); latchNamed(first) || st.earlyTakeProfit || !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) {
		t.Fatalf("the quiet decline's tick carries no latch and leaves the ladder pending on it alone, got %+v", st)
	}

	latched, second := tickAtAGain(pending, "", stairOpen(27), withBlock(block))
	assertRow(t, second, LatchedRow(stairPrices[3], swingFinding(reasons)))
	if second.Rows[0].Event != EventLatched || !reflect.DeepEqual(second.Rows[0].Features, []string{FeatureEarlyTakeProfit}) {
		t.Errorf("the next tick's row is of the latched kind and carries the latch alone, got %+v", second.Rows[0])
	}
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) {
		t.Errorf("latched, and still pending from the fourth fill on the quiet decline alone, got %+v", st)
	}

	_, third := tickAtAGain(latched, "", stairOpen(28), withBlock(block))
	assertUntouched(t, third, "")
	if len(latched.Logs) != 2 || len(latched.StrategyEvents) != 2 || carriedEvents(latched, GateSlowDecline, EventLatched) != 1 {
		t.Errorf("one row and one event for each of the two ticks, the latch written once, got %d rows and %d events", len(latched.Logs), len(latched.StrategyEvents))
	}
}

// row is the address of a row, for the tables whose expectation is none.
func row(built Row) *Row {
	return &built
}

// The quiet decline never latches the ladder: a ladder it makes pending is not
// latched by that tick or any after it, however it reads — the row of its going
// pending names no indecision, and the take profit it reads is the newest
// fill's alone. The ticks run at a print over the ladder's break even (gainPrint) with a buy order resting,
// where the pending ladder in buy is activated nothing and writes no row.
func TestTheQuietDeclineNeverLatches(t *testing.T) {
	withLinearDecline(t, false)
	block := withVerdict(stairBouncedBlock())
	gain := gainPrint(stairFourth())
	pending, got := tickResting(stairFourth(), "", gain, stairOpen(26), withBlock(block))
	assertRow(t, got, PendingRow(stairPrices[3], quietFinding(slowDeclineReasons)))
	if latchNamed(got) || rebuildState(pending).earlyTakeProfit {
		t.Fatalf("the quiet decline writes no latch, got %+v", got)
	}
	for _, reading := range []aggragates.SmartTakeLossIndicators{block, withLegOnAndQuiet(block), withBreaks(block), withVerdict(stairBouncedBlock())} {
		assertUntouched(t, Apply(withRestingOrder(pending), "", gain, withBlock(reading)), "")
	}
	if rebuildState(pending).earlyTakeProfit {
		t.Fatal("nothing the quiet decline reads latches the ladder")
	}
}

// A ladder pending on either feature, or both, is acted on at every tick under the
// one reason: the ladder in buy has its stopLoss activated at a print under its break
// even, the activation row naming the decline features the ladder is pending on,
// whichever made it so, and nothing is done at a print over it; the ladder in
// stopLoss is sold, and the exit row names the same features.
func TestAPendingLadderIsActedOnWhicheverFeatureMadeItPending(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, false)
	for name, tc := range map[string]struct {
		findings []Finding
		features []string
		label    string
	}{
		"the quiet decline": {[]Finding{quietFinding(nil)}, []string{FeatureQuietDecline}, "quiet decline"},
		"the swing decline": {[]Finding{swingFinding(nil)}, []string{FeatureSwingDecline}, "swing decline"},
		"both":              {[]Finding{quietFinding(nil), swingFinding(nil)}, []string{FeatureQuietDecline, FeatureSwingDecline}, "quiet decline and swing decline"},
	} {
		trade := withRows(stairFourth(), PendingRow(stairPrices[3], tc.findings...))
		block := withBlock(stairFourthBlock())
		armed := armingPrice(trade)
		for _, position := range everyProposal() {
			assertUntouched(t, applyAtAGain(trade, position, block), position)
			assertActivated(t, Apply(trade, position, armed, block), StopLossActivatedRow(stairPrices[3], -handLossPct(trade, armed), tc.features...))
			assertSold(t, Apply(restingIn(trade, "stopLoss"), position, armed, block), reasonSlowDecline)
		}
		got := Apply(restingIn(trade, "stopLoss"), "stopLoss", armed, block)
		assertSold(t, got, reasonSlowDecline)
		want := fmt.Sprintf("smartTakeLoss: sell at slow decline %.2f (%s)", armed, tc.label)
		if exit := ExitRow(trade, got.Reason, armed); exit.Message != want || exit.Gate != GateSlowDecline || exit.Event != EventSold {
			t.Errorf("%s: the exit row reads %+v, want %q under the slow decline gate", name, exit, want)
		}
	}
}

// A new fill a pending ladder takes is judged on the readings that apply to it.
// The quiet decline applies on a ladder it watches and answers on a tick that
// serves the fill window, which marks the window as read; the swing decline
// applies while the series is served and answers once it has closed the fill's
// bar. Any reading that answers holding CONFIRMS the fill at once, on the
// features that held; the fill is CANCELLED once every reading that applies has
// answered and none held; otherwise it WAITS — no row, and the shared action does
// nothing, the fill being unjudged. A confirmed fill is acted on from the tick that
// confirms it, the stopLoss of the ladder in buy activated and the ladder in stopLoss
// sold, and a cancelled one is not. The judgement is read at a print over the ladder's
// break even (gainPrint) with a buy order resting, where the row stands alone. The early take profit is
// switched off beside, so no latch joins these rows.
func TestANewFillIsJudgedOnEveryReadingThatAppliesToIt(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, false)
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
		"both miss: cancelled":                              {withBreaks(rally), row(CancelledRow(price5, quietFinding(quietDeclineBreakReasons), swingFinding(broken)))},
		"the quiet decline misses, no series: cancelled":    {withoutTheSeries(withBreaks(closed)), row(CancelledRow(price5, quietFinding(quietDeclineBreakReasons)))},
		"no window, the swing decline misses: waits":        {withoutTheWindow(rally), nil},
		"no window, the swing decline holds":                {withoutTheWindow(closed), row(ConfirmedRow(price5, swingFinding(held)))},
		"no window, the bar open: waits":                    {withoutTheWindow(notClosed), nil},
		"no window, no series: waits":                       {withoutTheSeries(withoutTheWindow(closed)), nil},
	} {
		trade := pendingAtFourthWithTheFifth(both...)
		if tc.want == nil {
			assertSoldNever(t, trade, withBlock(tc.block))
			if st := rebuildState(trade); !slowDeclineFillUnjudged(trade, st) {
				t.Errorf("%s: the fill stays unjudged", name)
			}
			continue
		}
		got := applyAtAGain(trade, "", withBlock(tc.block))
		assertRow(t, got, *tc.want)
		assertNoSale(t, got, "")
		if tc.want.Event == EventPending {
			assertActedOn(t, trade, withBlock(tc.block), *tc.want)
		} else {
			assertSoldNever(t, trade, withBlock(tc.block), *tc.want)
		}
	}
}

// A feature switched off applies to no fill: with the swing decline off the
// quiet decline alone judges, whatever the series would read; with the quiet
// decline off the swing decline does, and a ladder no reading applies to — the
// series not served — waits rather than cancels on nothing. The judgements are read at
// a print over the ladder's break even (gainPrint) with a buy order resting, where the ladder in buy is
// activated nothing and the row stands alone.
func TestANewFillIsJudgedOnTheFeaturesThatAreOn(t *testing.T) {
	withEarlyTakeProfit(t, false)
	both := []Finding{quietFinding(nil), swingFinding(nil)}
	price5 := stairPrices[4]

	withSwingDecline(t, false)
	trade := pendingAtFourthWithTheFifth(both...)
	gain := gainPrint(trade)
	if st := rebuildState(trade); !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) {
		t.Fatalf("fixture drifted: the swing decline is off, the ladder is pending on the quiet decline alone, got %+v", st)
	}
	assertRow(t, Apply(withRestingOrder(trade), "", gain, withBlock(withBreaks(stairFifthBlock()))), CancelledRow(price5, quietFinding(quietDeclineBreakReasons)))
	assertRow(t, Apply(withRestingOrder(trade), "", gain, withBlock(withLegOnAndQuiet(stairBlock(stairBounce, 30)))), ConfirmedRow(price5, quietFinding(slowDeclineReasons)))
	withSwingDecline(t, true)

	withQuietDecline(t, false)
	if st := rebuildState(trade); !reflect.DeepEqual(st.pendingFeatures, []string{FeatureSwingDecline}) {
		t.Fatalf("fixture drifted: the quiet decline is off, the ladder is pending on the swing decline alone, got %+v", st)
	}
	rally := stairBlock(stairBounce, 30)
	assertRow(t, Apply(withRestingOrder(trade), "", gain, withBlock(rally)), CancelledRow(price5, swingFinding(stairReading(t, trade, rally, false).Breaks)))
	closed := stairFifthBlock()
	assertRow(t, Apply(withRestingOrder(trade), "", gain, withBlock(closed)), ConfirmedRow(price5, swingFinding(stairReading(t, trade, closed, true).Reasons)))
	assertSoldNever(t, trade, withBlock(withoutTheSeries(closed)))
	assertSoldNever(t, trade, withBlock(stairBlock(stairTurns, 29)))
}

// The fill that takes the ladder to its last depth is judged by no reading,
// whatever each would answer, and the shared action acts on it at once, the depth
// bound being off here (TestMain): the ladder stays pending from where it was on the
// features it was pending on, writes no row, and has its stopLoss activated in buy
// and is sold in stopLoss, as any pending ladder is.
func TestTheLastDepthFillIsJudgedByNoReading(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, false)
	seventh := withRows(stairLadder(0, 9, 15, 24, 30, 36, 42), PendingRow(stairPrices[6], quietFinding(nil), swingFinding(nil)))
	eighth := withFill(seventh, stairPrices[7], stairOpen(48).Add(time.Minute))
	for name, block := range map[string]aggragates.SmartTakeLossIndicators{
		"the quiet decline holds":     withLegOnAndQuiet(stairBlock(stairDecline, 48)),
		"both miss":                   withBreaks(stairBlock([]float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200, 210, 220, 230, 240, 250, 260}, 48)),
		"no series, the window alone": withEveryRowTrend(aggragates.SmartTakeLossIndicators{QuietDeclineFillFrom: fillWindowFrom.UnixMilli()}),
	} {
		t.Run(name, func(t *testing.T) {
			assertActedOn(t, eighth, withBlock(block))
		})
	}
}

// A hold resets a pending gate on its first held tick whichever feature made it
// pending — the quiet decline, the swing decline or both — with one row naming
// those features and no reason, at the newest fill's price, and the ladder reads
// not pending from then on. Nothing else is written while the hold lasts,
// whatever the readings serve, and nothing is activated or sold, in buy or in
// stopLoss: the hold pauses the gates, and the shared action they ask goes unasked
// with them.
func TestAHeldGateIsResetOnceWhicheverFeatureMadeItPending(t *testing.T) {
	withQuietDecline(t, true)
	withSwingDecline(t, true)
	for name, findings := range map[string][]Finding{
		"the quiet decline": {quietFinding(nil)},
		"the swing decline": {swingFinding(nil)},
		"both":              {quietFinding(nil), swingFinding(nil)},
	} {
		held := heldBy(withRows(stairFourth(), PendingRow(stairPrices[3], findings...)), stairOpen(25))
		block := withVerdict(stairFourthBlock())
		ticked, got := engineTick(held, "stopLoss", armingPrice(held), stairOpen(26), withBlock(block))
		assertRow(t, got, ResetRow(stairPrices[3], findings...))
		assertNoSale(t, got, "stopLoss")
		if st := rebuildState(ticked); st.slowDeclinePending || len(st.pendingFeatures) != 0 || !st.depthPriorityHeld {
			t.Errorf("%s: reset, held and not pending, got %+v", name, st)
		}
		for _, reading := range []aggragates.SmartTakeLossIndicators{block, withLegOnAndQuiet(block), withBreaks(block)} {
			for _, state := range []string{"buy", "stopLoss"} {
				assertUntouched(t, Apply(restingIn(ticked, state), "", slowDeclineBand+1, withBlock(reading)), "")
				assertUntouched(t, Apply(restingIn(ticked, state), "stopLoss", armingPrice(ticked), withBlock(reading)), "stopLoss")
			}
		}
		if resets := carriedEvents(ticked, GateSlowDecline, EventReset); resets != 1 || len(ticked.Logs) != len(held.Logs)+1 {
			t.Errorf("%s: one reset row and one reset event, got %d events and %d new rows", name, resets, len(ticked.Logs)-len(held.Logs))
		}
	}

	// A held ladder that is not pending triggers on nothing: both declines read
	// for it, and the hold pauses them both.
	heldNotPending := heldBy(stairFourth(), stairOpen(25))
	assertUntouched(t, Apply(heldNotPending, "", slowDeclineBand+1, withBlock(withVerdict(stairFourthBlock()))), "")
	assertUntouched(t, Apply(heldNotPending, "stopLoss", armingPrice(heldNotPending), withBlock(withVerdict(stairFourthBlock()))), "stopLoss")
}

// The next fill ends the hold, and the gate goes pending again only by a
// trigger: the quiet decline's, or the swing decline's, read on both for the
// ladder the reset left not pending. The ticks run at a print over the ladder's
// break even (gainPrint) with a buy order resting, where the ladder in buy is activated nothing and the
// pending row stands alone.
func TestTheNextFillAfterTheResetGoesPendingOnATrigger(t *testing.T) {
	held := heldBy(withRows(stairFourth(), PendingRow(stairPrices[3], quietFinding(nil), swingFinding(nil))), stairOpen(25))
	reset, _ := engineTick(held, "", underTheBand, stairOpen(26), withBlock(stairFourthBlock()))
	sixth := withFill(reset, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	gain, decline := gainPrint(sixth), stairBlock(stairDecline, 36)
	assertUntouched(t, Apply(withRestingOrder(sixth), "", gain, withBlock(withoutTheSeries(decline))), "")
	reasons := stairReading(t, sixth, decline, true).Reasons
	assertRow(t, Apply(withRestingOrder(sixth), "", gain, withBlock(decline)), PendingRow(stairPrices[5], indecisionFinding(nil), swingFinding(reasons)))
	assertRow(t, Apply(withRestingOrder(sixth), "", gain, withBlock(withVerdict(withoutTheSeries(decline)))), PendingRow(stairPrices[5], quietFinding(slowDeclineReasons)))
}

// The swing decline latches a ladder that is already pending — the quiet
// decline made it pending, or the swing decline was off or unread then — on the
// first tick that reads for the ladder's judged newest fill, whatever the bars
// since that fill: one row of its own, the latch with the swing decline's
// reasons, an event of the latched kind carrying the indecision alone. The ladder
// stays pending on what it was pending on. With a last bar that breaks the
// window, the series not served, from the quiet decline's reading and with the
// indecision off, nothing is written; while a hold lasts the row is the reset
// alone; and once latched, never again. The ladder is pending, so the shared
// action acts on it all along: the ticks that read the latch's row alone run at a
// print over the ladder's break even (gainPrint) with a buy order resting, where the ladder in buy is
// activated nothing, and at a print under it the latched ladder is activated with
// the activation row alone, the latch not written a second time.
func TestTheSwingDeclineLatchesAnAlreadyPendingLadderOnTheTickThatReadsForIt(t *testing.T) {
	withLinearDecline(t, false)
	pending := withRows(stairFourth(), PendingRow(stairPrices[3], quietFinding(slowDeclineReasons)))
	block := stairFourthBlock()
	reading := stairReading(t, pending, block, true)
	gain := gainPrint(pending)

	latched, got := tickResting(pending, "", gain, stairOpen(26), withBlock(block))
	assertRow(t, got, LatchedRow(stairPrices[3], swingFinding(reading.Reasons)))
	if got.Rows[0].Event != EventLatched || !reflect.DeepEqual(got.Rows[0].Features, []string{FeatureEarlyTakeProfit}) || !slices.Equal(got.Rows[0].Reasons, reading.Reasons) {
		t.Errorf("the row is of the latched kind, carries the indecision alone and names the swing decline's reasons, got %+v", got.Rows[0])
	}
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.slowDeclinePending || !reflect.DeepEqual(st.pendingFeatures, []string{FeatureQuietDecline}) || st.slowDeclinePendingFrom != stairPrices[3] {
		t.Errorf("latched, and still pending from the fourth fill on the quiet decline, got %+v", st)
	}
	assertUntouched(t, Apply(withRestingOrder(latched), "", gain, withBlock(block)), "")
	armed := armingPrice(latched)
	assertActivated(t, Apply(latched, "", armed, withBlock(block)), StopLossActivatedRow(stairPrices[3], -handLossPct(latched, armed), FeatureQuietDecline))

	turns := append(slices.Clone(stairTurns), 84, 83, 82, 81)
	for lastBar := 0; lastBar < 1+3*(len(turns)-1); lastBar++ {
		read, holds := readsAsOf(pending, stairBlock(turns, lastBar))
		got := Apply(withRestingOrder(pending), "", gain, withBlock(stairBlock(turns, lastBar)))
		if (len(got.Rows) == 1 && got.Rows[0].Event == EventLatched) != holds || len(got.Rows) > 1 {
			t.Errorf("series to bar %d: latch row %v, want %v (%+v)", lastBar, len(got.Rows) == 1, holds, got.Rows)
		}
		if holds {
			assertRow(t, got, LatchedRow(stairPrices[3], swingFinding(read.Reasons)))
		}
	}
	assertUntouched(t, Apply(withRestingOrder(pending), "", gain, withBlock(withoutTheSeries(block))), "")
	assertUntouched(t, Apply(withRestingOrder(pending), "", gain, withBlock(stairBouncedBlock())), "")
	verdictOnly := withVerdict(stairBouncedBlock())
	assertUntouched(t, Apply(withRestingOrder(pending), "", gain, withBlock(verdictOnly)), "")

	withEarlyTakeProfit(t, false)
	assertUntouched(t, Apply(withRestingOrder(pending), "", gain, withBlock(block)), "")
	withEarlyTakeProfit(t, true)
	assertRow(t, Apply(withRestingOrder(heldBy(pending, stairOpen(25))), "", gain, withBlock(block)), ResetRow(stairPrices[3], quietFinding(nil)))
}

// The three features read on their own switches: with the quiet decline off the
// swing decline triggers alone and latches, with the swing decline off the quiet
// decline triggers alone and nothing latches beside it, with both off the gate
// triggers nothing. The early take profit reads on its own switches: the latch on
// the swing reading fires whatever swingdecline.Enabled says (the gate off, the
// reading still latches), and with that latch off the sophos reading still does.
// The ticks run at a print over the ladder's break even (gainPrint) with a buy
// order resting, where the ladder in buy is activated nothing and the gate's row
// stands alone.
func TestTheFeaturesTriggerOnTheirOwnSwitches(t *testing.T) {
	withLinearDecline(t, false)
	block := withVerdict(stairFourthBlock())
	reasons := stairReading(t, stairFourth(), block, true).Reasons
	gain := gainPrint(stairFourth())

	withQuietDecline(t, false)
	assertRow(t, Apply(withRestingOrder(stairFourth()), "", gain, withBlock(block)), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(reasons)))
	withQuietDecline(t, true)

	withSwingDecline(t, false)
	assertRow(t, Apply(withRestingOrder(stairFourth()), "", gain, withBlock(block)), PendingRow(stairPrices[3], quietFinding(slowDeclineReasons)))

	withQuietDecline(t, false)
	assertRow(t, Apply(withRestingOrder(stairFourth()), "", gain, withBlock(block)), LatchedRow(stairPrices[3], swingFinding(reasons)))
	withEarlyTakeProfitSwingDecline(t, false)
	assertUntouched(t, Apply(withRestingOrder(stairFourth()), "", gain, withBlock(block)), "")
	assertRow(t, Apply(withRestingOrder(stairFourth()), "", gain, indecisionReading()), LatchedRow(stairPrices[3], indecisionFinding(indecisionReasons)))
}

// A ladder's life on the gate, tick by tick, one row and one event each: both
// declines read on the first tick and the quiet decline, the first, makes the
// ladder pending with the sophos latch beside it, the fifth fill is confirmed on
// the quiet decline alone (the one the ladder is pending on), the sixth is
// cancelled on it, and the seventh goes pending again on the swing decline, the
// verdict not reading. The rows and the events written are the ones Apply handed
// back, in order. Every tick runs at a print over the ladder's break even
// (gainPrint) with a buy order resting, where the ladder in buy is activated
// nothing, so that a tick's one row is the gate's own.
func TestAGatesLifeIsOneRowAndOneEventAtATime(t *testing.T) {
	trade := stairFourth()
	var want []Row
	tick := func(block aggragates.SmartTakeLossIndicators, at time.Time, row Row) {
		t.Helper()
		var got Result
		trade, got = tickAtAGain(trade, "", at, withBlock(block))
		assertRow(t, got, row)
		want = append(want, row)
		assertNoSale(t, got, "")
	}

	first := withVerdict(stairFourthBlock())
	first.Indecision, first.QuietDeclineBreakReasons = true, indecisionReasons
	stairReading(t, trade, first, true)
	tick(first, stairOpen(26), PendingRow(stairPrices[3], indecisionFinding(indecisionReasons), quietFinding(slowDeclineReasons)))

	trade = withFill(trade, stairPrices[4], fifthFillAt)
	second := withLegOnAndQuiet(stairFifthBlock())
	stairReading(t, trade, second, true)
	tick(second, stairOpen(31), ConfirmedRow(stairPrices[4], quietFinding(slowDeclineReasons)))

	trade = withFill(trade, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	third := withBreaks(stairBlock(stairRally, 36))
	stairReading(t, trade, third, false)
	tick(third, stairOpen(37), CancelledRow(stairPrices[5], quietFinding(quietDeclineBreakReasons)))
	if st := rebuildState(trade); st.slowDeclinePending || !st.earlyTakeProfit {
		t.Fatalf("cancelled on the quiet decline and latched, got %+v", st)
	}

	trade = withFill(trade, stairPrices[6], stairOpen(42).Add(17*time.Minute))
	fourth := stairBlock(stairDecline, 42)
	tick(fourth, stairOpen(43), PendingRow(stairPrices[6], swingFinding(stairReading(t, trade, fourth, true).Reasons)))

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
