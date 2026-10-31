package smartsell

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// linearLookBackReasons is what sophos names for the bar of the indecision look-back
// that latches a ladder.
var linearLookBackReasons = []string{"the indecision stood on a bar of the look-back"}

// A ladder whose fills sit on one line, under a trend that meets the need, gets ONE
// row on the first tick that reads it, at every depth from the arm depth: the latch
// of the early take profit, of kind rowLatched, naming the linear decline's
// reasons under its own label, at the newest fill's price. The proposal passes
// untouched and nothing is sold — whether the ladder proposes an add, an arming of a
// depth, nothing, or a close it rests in, and at a print far under the newest fill
// too, the line making no ladder pending, so that the shared sale has no trigger to
// replace an arming with — and an impasse strategy's parent is read like any other.
func TestTheLinearDeclineLatchesALadderOnOneLineOnItsOwnRow(t *testing.T) {
	onlyTheLinearDecline(t)
	proposals := slices.Concat([]string{"", "buy", "takeProfit", "sell"}, armingProposals)
	for depth := linearDepth; depth <= linearDepth+2; depth++ {
		slope := linearSlope
		trade, block := linearLadder(slope, depth), linearBlock(slope, depth, linearTail, linearTrendMet)
		want := linearRow(t, trade, block)
		reasons := strings.Join(want.Reasons, ", ")
		message := "smartSell: early take profit latched (linear decline: " + reasons + ")"
		if want.Message != message || want.Gate != GateSlowDecline || want.Event != EventLatched || len(want.Reasons) == 0 {
			t.Fatalf("depth %d: the row is the gate's latched kind naming the linear decline, got %+v", depth, want)
		}
		if !slices.Equal(want.Features, []string{FeatureEarlyTakeProfit}) || want.Price != trade.PositionPrice {
			t.Fatalf("depth %d: the event carries the indecision alone, at the newest fill, got %+v", depth, want)
		}
		for _, price := range []float64{trade.PositionPrice, trade.PositionPrice / 2} {
			for _, position := range proposals {
				got := Apply(trade, position, price, withBlock(block))
				assertRow(t, got, want)
				assertNoSale(t, got, position)
			}
		}
	}

	trade, block := linearScene(linearTrendMet)
	want := linearRow(t, trade, block)
	resting := trade
	resting.PositionType = "takeProfit"
	assertRow(t, Apply(resting, "", trade.PositionPrice, withBlock(block)), want)
	impasse := trade
	impasse.Strategy.Params.Impasse = true
	assertRow(t, Apply(impasse, "", trade.PositionPrice, withBlock(block)), want)
}

// The row goes out once, and its event is the latched kind carrying the indecision
// alone: from the tick after, rebuildState reads the ladder latched and not pending,
// and no second row or event goes out however often the line comes again, the series
// grows, the trend turns short or sophos serves nothing. Nothing takes the latch
// away — not the feature switched off after it — and the take profit of a latched
// ladder reads its position price, as it does for any latch.
func TestTheLinearDeclineLatchesOnceAndTheFoldAndTheTakeProfitReadIt(t *testing.T) {
	onlyTheLinearDecline(t)
	trade, block := linearScene(linearTrendMet)
	price, fromAverage, fromPosition := overBreakEven(trade)
	if !(fromAverage < fromPosition) {
		t.Fatal("fixture drifted: the position price's move must exceed the average entry price's")
	}
	if got := TakeProfitPercentage(trade, price, fromAverage); got != fromAverage {
		t.Fatalf("control: an unlatched ladder's take profit reads the input %v, got %v", fromAverage, got)
	}

	latched, got := engineTick(trade, "", trade.PositionPrice, testutil.At("23:00:00"), withBlock(block))
	assertRow(t, got, linearRow(t, trade, block))
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.earlyTakeProfitWatched || st.slowDeclinePending {
		t.Fatalf("the tick after: latched, watched and not pending, got %+v", st)
	}
	var data EventData
	events := latched.StrategyEventsOf(aggragates.StrategyParamSmartSell, GateSlowDecline)
	if len(events) != 1 || events[0].DecodeData(&data) != nil || data.Event != EventLatched || !slices.Equal(data.Features, []string{FeatureEarlyTakeProfit}) {
		t.Fatalf("one latched event carrying the indecision alone, got %+v", events)
	}
	if got := TakeProfitPercentage(latched, price, fromAverage); got != fromPosition {
		t.Fatalf("a latched ladder's take profit reads its position price %v, got %v", fromPosition, got)
	}

	slope := linearSlope
	ticks := []aggragates.AIIndicators{
		withBlock(block),
		withBlock(linearBlock(slope, linearDepth, 3*linearTail, linearTrendMet)),
		withBlock(withSmcTrend(block, linearTrendShort, linearTrendValid)),
		withBlock(aggragates.SmartSellIndicators{}),
	}
	for index, ai := range ticks {
		var next Result
		latched, next = engineTick(latched, "", trade.PositionPrice, testutil.At("23:15:00").Add(time.Duration(index)*time.Minute), ai)
		assertUntouched(t, next, "")
	}
	withLinearDecline(t, false)
	if st := rebuildState(latched); !st.earlyTakeProfit {
		t.Fatalf("nothing takes the latch away, the feature switched off included, got %+v", st)
	}
	if got := TakeProfitPercentage(latched, price, fromAverage); got != fromPosition {
		t.Fatalf("the latch stays with the feature off: the take profit reads %v, got %v", fromPosition, got)
	}
	rows, latches := carriedRows(latched, "linear decline"), carriedLatches(latched)
	if rows != 1 || latches != 1 || carriedEvents(latched, GateSlowDecline, EventLatched) != 1 {
		t.Fatalf("one row naming the line and one latched event over every tick, got %d and %d in %+v", rows, latches, latched.Logs)
	}

	other := latchedBy(trade)
	assertUntouched(t, Apply(other, "", trade.PositionPrice, withBlock(block)), "")
}

// The line is read on every tick with no bound on the bars since the newest fill,
// the chords being fixed by the fills until the next one: a trend that misses the
// need leaves a ladder unlatched however many bars go by, and the first tick whose
// trend turns down latches it, with no new fill, on the very row the line would have
// written earlier. The bar that holds the newest fill must have closed — sophos
// serves closed bars — so a series that stops a bar short of it reads nothing.
func TestTheLinearDeclineLatchesOnALaterBarWhenTheTrendTurnsDownWithNoNewFill(t *testing.T) {
	onlyTheLinearDecline(t)
	needsTheLinearTrend(t)
	slope := linearSlope
	trade := linearLadder(slope, linearDepth)
	for tail := 0; tail <= 3*linearTail; tail += 2 {
		short := linearBlock(slope, linearDepth, tail, linearTrendShort)
		assertUntouched(t, Apply(trade, "", trade.PositionPrice, withBlock(short)), "")
	}
	later := linearBlock(slope, linearDepth, 5*linearTail, linearTrendMet)
	assertRow(t, Apply(trade, "", trade.PositionPrice, withBlock(later)), linearRow(t, trade, later))

	before := linearBlock(slope, linearDepth-1, linearGap-1, linearTrendMet)
	if len(before.SeriesOpens) != linearLead+(linearDepth-1)*linearGap {
		t.Fatalf("fixture drifted: the series must stop a bar short of the newest fill's, got %d bars", len(before.SeriesOpens))
	}
	assertUntouched(t, Apply(trade, "", trade.PositionPrice, withBlock(before)), "")
	closed := linearBlock(slope, linearDepth-1, linearGap, linearTrendMet)
	assertRow(t, Apply(trade, "", trade.PositionPrice, withBlock(closed)), linearRow(t, trade, closed))
}

// A pending ladder's new fill the gate cannot judge yet — no window read served, so the
// quiet decline's reading is not taken — writes no row on that tick, so the line latches
// on it all the same: the latch is the row, and the ladder stays pending with its newest
// fill unjudged, which sells nothing, a tick that arms a depth included.
func TestTheLinearDeclineLatchesOnATickThatWaitsOnANewFill(t *testing.T) {
	onlyTheLinearDecline(t)
	withQuietDecline(t, true)
	if linearDepth >= lastDepthFills || linearDepth < quietdecline.ArmDepth {
		t.Skip("the quiet decline does not watch the ladder, or it stands at its last depth, where a new fill is never judged")
	}
	slope := linearSlope
	fills := linearFills(slope, linearDepth)
	pendingFrom := fills[linearDepth-2]
	trade := withRow(testutil.LadderTrade(false, fills...), PendingRow(pendingFrom.Price, quietFinding(nil)), pendingFrom.At.Add(time.Minute))
	block := linearBlock(slope, linearDepth, linearTail, linearTrendMet)
	if st := rebuildState(trade); !st.slowDeclinePending || !slowDeclineFillUnjudged(trade, st) {
		t.Fatalf("fixture drifted: pending from the fill before the newest, the newest unjudged, got %+v", st)
	}

	got := Apply(trade, "", trade.PositionPrice, withBlock(block))
	assertRow(t, got, linearRow(t, trade, block))
	assertNoSale(t, got, "")
	for _, position := range armingProposals {
		arming := Apply(trade, position, armingPrice(trade), withBlock(block))
		assertRow(t, arming, linearRow(t, trade, block))
		assertNoSale(t, arming, position)
	}
	latched := withRow(trade, got.Rows[0], testutil.At("23:00:00"))
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.slowDeclinePending || !slowDeclineFillUnjudged(latched, st) {
		t.Fatalf("the latch leaves the ladder pending with its newest fill unjudged, got %+v", st)
	}
}

// The sophos ways of latching keep their precedence over the line, each naming its own
// reasons and never the line's: the flag of the last closed bar and the look-back. The
// swing decline's precedence is TestTheSwingDeclineKeepsPrecedenceOverTheLine's, in
// linearDeclinePrecedence_test.go.
func TestTheOtherWaysOfLatchingKeepPrecedenceOverTheLine(t *testing.T) {
	onlyTheLinearDecline(t)
	trade, block := linearScene(linearTrendMet)
	newest := trade.PositionPrice
	line := linearRow(t, trade, block)
	assertRow(t, Apply(trade, "", newest, withBlock(block)), line)

	flag := block
	flag.Indecision, flag.QuietDeclineBreakReasons = true, indecisionReasons
	assertRow(t, Apply(trade, "", newest, withBlock(flag)), LatchedRow(newest, indecisionFinding(indecisionReasons)))

	recent := block
	recent.IndecisionRecentAt, recent.IndecisionRecentReasons = linearOpen(0).UnixMilli(), linearLookBackReasons
	assertRow(t, Apply(trade, "", newest, withBlock(recent)), LatchedRow(newest, indecisionFinding(linearLookBackReasons)))
}
