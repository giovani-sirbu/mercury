package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/lineardecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/quietdecline"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// linearSwingReasons is what a swing decline stubbed to hold names.
var linearSwingReasons = []string{"the swing decline reads here"}

// The read is asked on every print of a watched ladder that is not latched, so a
// print it does not read allocates nothing. It answers at once for a trend that
// misses the need, a ladder shallower than the arm depth, a newest fill with no
// stamp, a ladder it never reads, a tick a depth priority holds and a tick whose row
// another reading wrote. A print whose trend is met and whose line misses costs the
// pass over the bars and no more: the three fills the rule reads are handed over by
// value, no slice is built of them, and no text is formatted.
func TestTheLinearDeclineAllocatesNothingForAPrintItDoesNotRead(t *testing.T) {
	onlyTheLinearDecline(t)
	trade, block := linearScene(linearTrendMet)
	st := rebuildState(trade)
	if _, reads := linearDeclineReads(st, block, tick{}); !reads {
		t.Fatal("control: the ladder on one line is read")
	}
	offTrade, offBlock := linearScene(linearTrendMet)
	offBlock.SeriesCloses[linearLead+linearGap/2] *= 1 + linearOver/100
	off := rebuildState(offTrade)
	if reading, reads := linearDeclineReads(off, offBlock, tick{}); reads || reading.Deviation <= lineardecline.MaxDeviationPct {
		t.Fatalf("control: a close off the line is a miss that read the bars, got reads %v, %+v", reads, reading)
	}
	unstamped := rebuildState(trade)
	unstamped.fills = slices.Clone(unstamped.fills)
	unstamped.fills[len(unstamped.fills)-1].At = time.Time{}
	held := st
	held.depthPriorityHeld = true
	futures := trade
	futures.Strategy.TradeType = aggragates.Futures

	type printCase struct {
		trade aggragates.Trades
		st    state
		block aggragates.SmartTakeLossIndicators
		found tick
	}
	prints := map[string]printCase{
		"a newest fill with no stamp":           {trade, unstamped, block, tick{}},
		"a futures ladder":                      {futures, rebuildState(futures), block, tick{}},
		"a tick a depth priority holds":         {trade, held, block, tick{}},
		"a tick that wrote another row":         {trade, st, block, tick{kind: rowCancelled}},
		"a line that misses with the trend met": {offTrade, off, offBlock, tick{}},
	}
	if shallow := lineardecline.ArmDepth - 1; shallow >= 1 {
		prefix := linearLadder(linearSlope, shallow)
		prints["a ladder shallower than the arm depth"] = printCase{prefix, rebuildState(prefix), block, tick{}}
	}
	if lineardecline.MinSmcTrendDown > 0 {
		prints["a trend short of the need"] = printCase{trade, st, withSmcTrend(block, linearTrendShort, linearTrendValid), tick{}}
		prints["a trend not read"] = printCase{trade, st, withSmcTrend(block, 0, 0), tick{}}
	}
	for name, one := range prints {
		if _, reads := linearDeclineReads(one.st, one.block, one.found); reads {
			t.Fatalf("%s: the print is read", name)
		}
		allocs := testing.AllocsPerRun(50, func() {
			linearDeclineReads(one.st, one.block, one.found)
		})
		if allocs != 0 {
			t.Errorf("%s: the print allocated %v times, want none", name, allocs)
		}
	}
	withEarlyTakeProfitLinearDecline(t, false)
	switchedOff := rebuildState(trade)
	allocs := testing.AllocsPerRun(50, func() {
		linearDeclineReads(switchedOff, block, tick{})
	})
	if allocs != 0 {
		t.Errorf("switched off: the print allocated %v times, want none", allocs)
	}
}

// The three fills the rule reads are the window's first (lineardecline.Window back from
// the newest), the one before the newest and the newest at every depth. A ladder
// shallower than the window answers zero fills and never indexes past itself, which
// the arm depth could otherwise make it do: it follows a bound that a retune can take
// below the window.
func TestTheLinearDeclineReadsTheWindowsFirstTheFillBeforeTheNewestAndTheNewest(t *testing.T) {
	for count := 0; count <= 6; count++ {
		fills := make([]entryFill, count)
		for index := range fills {
			fills[index] = entryFill{Price: float64(index + 1), At: linearOpen(index)}
		}
		first, previous, newest := linearDeclineFills(fills)
		want, read := [3]series.Fill{}, swingFills(fills)
		if count >= lineardecline.Window {
			want = [3]series.Fill{read[count-lineardecline.Window], read[count-2], read[count-1]}
		}
		if got := [3]series.Fill{first, previous, newest}; got != want {
			t.Errorf("%d fills: got %+v, want %+v", count, got, want)
		}
	}
}

// A depth priority hold pauses the linear decline as it does the swing decline: a
// held ladder is read on no tick, whatever the series and the trend serve, and the
// ladder's next fill ends the hold, from which the line is read again, with that
// fill as its newest. With the pause switched off the hold rows are ignored.
func TestTheLinearDeclineReadsNoTickADepthPriorityHolds(t *testing.T) {
	onlyTheLinearDecline(t)
	withDepthPriorityPause(t, true)
	slope := linearSlope
	trade, block := linearScene(linearTrendMet)
	newestAt := trade.History[len(trade.History)-1].CreatedAt
	held := heldBy(trade, newestAt.Add(time.Minute))
	if !rebuildState(held).depthPriorityHeld {
		t.Fatal("fixture drifted: the ladder must read held")
	}
	for _, position := range []string{"", "stopLoss", "takeProfit"} {
		assertUntouched(t, Apply(held, position, trade.PositionPrice, withBlock(block)), position)
	}

	withDepthPriorityPause(t, false)
	assertRow(t, Apply(held, "", trade.PositionPrice, withBlock(block)), linearRow(t, held, block))
	withDepthPriorityPause(t, true)

	next := linearFills(slope, linearDepth+1)[linearDepth]
	resumed := withFill(held, next.Price, next.At)
	resumedBlock := linearBlock(slope, linearDepth+1, linearTail, linearTrendMet)
	if rebuildState(resumed).depthPriorityHeld {
		t.Fatal("fixture drifted: the next fill ends the hold")
	}
	assertRow(t, Apply(resumed, "", next.Price, withBlock(resumedBlock)), linearRow(t, resumed, resumedBlock))
}

// A tick that wrote another row does not latch on the line: the judgement of a
// pending ladder's new fill writes its cancelled row or its confirmed one, which the
// latch would have shared, and the line is read again on the next tick, which writes
// the latch as the row of its own. The ticks run at a print over the ladder's break even,
// where the shared action, which activates the stopLoss of a ladder in buy on a real
// loss alone, writes nothing; at a print under it a ladder the judgement leaves pending has
// its stopLoss activated on the tick of the confirmation, the activation row after the
// gate's own, and the line still leaves the latch to the next tick.
func TestAnotherRowOnTheTickLeavesTheLatchToTheNextTick(t *testing.T) {
	onlyTheLinearDecline(t)
	withQuietDecline(t, true)
	if linearDepth >= lastDepthFills || linearDepth < quietdecline.ArmDepth {
		t.Skip("the quiet decline does not watch the ladder, or it stands at its last depth, where a new fill is never judged")
	}
	slope := linearSlope
	fills := linearFills(slope, linearDepth)
	pendingFrom := fills[linearDepth-2]
	trade := withRow(testutil.LadderTrade(false, fills...), PendingRow(pendingFrom.Price, quietFinding(nil)), pendingFrom.At.Add(time.Minute))
	newest := rebuildState(trade).lastFill().Price
	if st := rebuildState(trade); !st.slowDeclinePending || !slowDeclineFillUnjudged(trade, st) {
		t.Fatalf("fixture drifted: pending from the fill before the newest, the newest unjudged, got %+v", st)
	}
	gain, loss := gainPrint(trade), trade.PositionPrice

	for name, tc := range map[string]struct {
		serve func(aggragates.SmartTakeLossIndicators) aggragates.SmartTakeLossIndicators
		row   Row
		stays bool
	}{
		"a fill it cancels":  {withBreaks, CancelledRow(newest, quietFinding(quietDeclineBreakReasons)), false},
		"a fill it confirms": {withLegOnAndQuiet, ConfirmedRow(newest, quietFinding(slowDeclineReasons)), true},
	} {
		block := tc.serve(linearBlock(slope, linearDepth, linearTail, linearTrendMet))
		linearReading(t, trade, block)

		judged, got := tickResting(trade, "", gain, testutil.At("23:00:00"), withBlock(block))
		assertRow(t, got, tc.row)
		assertNoSale(t, got, "")
		if rebuildState(judged).earlyTakeProfit {
			t.Fatalf("%s: the tick that wrote another row latches nothing on the line", name)
		}
		latched, got := tickResting(judged, "", gain, testutil.At("23:15:00"), withBlock(block))
		assertRow(t, got, linearRow(t, judged, block))
		if !rebuildState(latched).earlyTakeProfit {
			t.Fatalf("%s: the next tick latches the ladder", name)
		}
		assertUntouched(t, Apply(withRestingOrder(latched), "", gain, withBlock(block)), "")

		atALoss := Apply(trade, "", loss, withBlock(block))
		if tc.stays {
			assertActivated(t, atALoss, tc.row, StopLossActivatedRow(newest, -handLossPct(trade, loss), FeatureQuietDecline))
		} else {
			assertWaits(t, atALoss, "", tc.row)
		}
	}
}

// With the swing decline on and stubbed to read, the ladder on one line goes pending on
// it, the latch beside it with no reasons of its own and no line named; a ladder it
// already made pending, whose newest fill is judged, is latched on its reasons; and
// stubbed to read nothing it leaves both ticks to the line. The ticks run at a print
// over the ladder's break even, where the shared action activates nothing.
func TestTheSwingDeclineKeepsPrecedenceOverTheLine(t *testing.T) {
	onlyTheLinearDecline(t)
	withSwingDecline(t, true)
	depth := max(linearDepth, swingdecline.ArmDepth)
	if depth >= lastDepthFills {
		t.Skip("swingdecline.ArmDepth reaches the last depth, where a new fill is never judged")
	}
	slope := linearSlope
	trade := linearLadder(slope, depth)
	down := max(swingdecline.MinSmcTrendDown, linearTrendMet)
	block := withSmcTrend(linearBlock(slope, depth, linearTail, down), down, max(linearTrendValid, down))
	newest := trade.PositionPrice
	pending := withRow(trade, PendingRow(newest, swingFinding(nil)), testutil.At("23:00:00"))
	line := linearRow(t, trade, block)

	reading := func(holds bool) swingReadFunc {
		return func([]series.Fill, series.Series, series.Trend) (swingdecline.Reading, bool) {
			return swingdecline.Reading{Holds: holds, Reasons: linearSwingReasons}, holds
		}
	}

	gain := gainPrint(trade)

	withSwingMemo(t, newSwingDeclineMemo(4, reading(true)))
	goes := Apply(withRestingOrder(trade), "", gain, withBlock(block))
	assertRow(t, goes, PendingRow(newest, indecisionFinding(nil), swingFinding(linearSwingReasons)))
	judged := Apply(withRestingOrder(pending), "", gain, withBlock(block))
	assertRow(t, judged, LatchedRow(newest, swingFinding(linearSwingReasons)))

	withSwingMemo(t, newSwingDeclineMemo(4, reading(false)))
	assertRow(t, Apply(withRestingOrder(trade), "", gain, withBlock(block)), line)
	assertRow(t, Apply(withRestingOrder(pending), "", gain, withBlock(block)), line)
}
