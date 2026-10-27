package smarttakeloss

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/freefallprevention"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The latch on the free fall prevention reading reads the gate's own reading (the last closed bar or
// the window's share) and nothing else, whatever freefallprevention.Enabled says, so these tests
// switch the gate off and judge what the latch does with the answer.

// freeFallLatchDepth is the depth the latch is first read at: the arm depth of both the early take
// profit and the gate, never under one fill.
var freeFallLatchDepth = max(earlytakeprofit.ArmDepth, freefallprevention.ArmDepth, 1)

// onlyTheFreeFallPreventionLatch runs the calling test with the early take profit and its latch on
// the free fall prevention reading on, the gate itself and every decline off, the shared sale
// unbounded and capital protection off.
func onlyTheFreeFallPreventionLatch(t *testing.T) {
	t.Helper()
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitFreeFallPrevention(t, true)
	withFreeFallPrevention(t, false)
	withFlatMarket(t, false)
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withLinearDecline(t, false)
	withCapitalProtectionExit(t, false)
	withAllowedLoss(t, anyLoss)
	withSellLossIgnoreLastDepths(t, 0)
}

// freeFallLatchRow is the row the latch writes for the ladder: the latched row at its newest fill
// naming the reasons the reading gives.
func freeFallLatchRow(trade aggragates.Trades, reasons []string) Row {
	return LatchedRow(rebuildState(trade).lastFill().Price, Finding{Feature: FeatureFreeFallPrevention, Reasons: reasons})
}

// latchedFreeFallRows counts the rows of the trade that latched on the free fall prevention reading.
func latchedFreeFallRows(trade aggragates.Trades) int {
	return carriedRows(trade, "early take profit latched (free fall prevention:")
}

// The reading alone latches a watched ladder on one row of its own, at every depth and whatever the
// proposal and the print, the gate's own switch off: the latched kind of the slow decline gate
// naming the reading at the newest fill, the latch alone among its features, the reasons as the
// gate words them, no episode and no sale.
func TestTheFreeFallPreventionReadingLatchesAWatchedLadderOnItsOwnRow(t *testing.T) {
	onlyTheFreeFallPreventionLatch(t)
	proposals := slices.Concat([]string{"", "buy", "takeProfit", "sell"}, armingProposals)
	for depth := freeFallLatchDepth; depth < freeFallLatchDepth+3 && depth < lastDepthFills; depth++ {
		trade := latchLadder(depth)
		ai := freeFallMarket(trade)
		want := freeFallLatchRow(trade, freeFallLastBar)
		message := "smartTakeLoss: early take profit latched (free fall prevention: 1h in free fall on the last closed bar)"
		if want.Message != message || want.Gate != GateSlowDecline || want.Event != EventLatched || want.Price != trade.PositionPrice {
			t.Fatalf("depth %d: the gate's latched kind naming the reading at the newest fill, got %+v", depth, want)
		}
		if !slices.Equal(want.Features, []string{FeatureEarlyTakeProfit}) || !slices.Equal(want.Reasons, freeFallLastBar) {
			t.Fatalf("depth %d: the latch alone and the reading's reasons, got %+v", depth, want)
		}
		reanchored := trade
		reanchored.PositionPrice += 5
		assertRow(t, Apply(reanchored, "", trade.PositionPrice, ai), want)
		for _, price := range []float64{trade.PositionPrice, trade.PositionPrice / 2} {
			for _, position := range proposals {
				got := Apply(trade, position, price, ai)
				assertRow(t, got, want)
				assertNoSale(t, got, position)
			}
		}
		if st := rebuildState(withRows(trade, want)); st.freeFallPreventionActive || st.freeFallPreventionWatched {
			t.Fatalf("depth %d: the latch opens no episode of the gate, which is off, got %+v", depth, st)
		}
	}

	trade := latchLadder(freeFallLatchDepth)
	for _, miss := range freeFallMisses(aggragates.SideLong) {
		for _, position := range []string{"", "buy", "stopLoss"} {
			assertUntouched(t, Apply(trade, position, trade.PositionPrice, miss), position)
		}
	}
}

// The window's share latches as the last bar does, the reasons naming the bars.
func TestTheFreeFallPreventionShareLatchesToo(t *testing.T) {
	onlyTheFreeFallPreventionLatch(t)
	trade := latchLadder(freeFallLatchDepth)
	bars := freefallprevention.Window
	ai := withBlock(freeFallSeries(aggragates.SideLong, bars))
	reasons := []string{fmt.Sprintf("1h in free fall on %d of %d %s bars, over %.0f%%", bars, bars, freefallprevention.Timeframe, float64(freefallprevention.MinShare)*100)}
	want := freeFallLatchRow(trade, reasons)
	if want.Message != "smartTakeLoss: early take profit latched (free fall prevention: "+reasons[0]+")" {
		t.Fatalf("fixture drifted: the row words the share, got %q", want.Message)
	}
	assertRow(t, Apply(trade, "", trade.PositionPrice, ai), want)

	inverse := testutil.LadderTrade(true, fills(freeFallLatchDepth, "17:38:00")...)
	assertUntouched(t, Apply(inverse, "", inverse.PositionPrice, withBlock(freeFallSeries(aggragates.SideShort, bars))), "")
}

// The row goes out once; the fold reads the ladder latched and the take profit reads its position
// price; the latch lives for good, the switch off included, and the events carry the latch alone.
func TestTheFreeFallPreventionLatchGoesOutOnceAndTheFoldAndTheTakeProfitReadIt(t *testing.T) {
	onlyTheFreeFallPreventionLatch(t)
	trade := latchLadder(max(freeFallLatchDepth, 2))
	ai := freeFallMarket(trade)
	price, fromAverage, fromPosition := overBreakEven(trade)
	if !(fromAverage < fromPosition) {
		t.Fatal("fixture drifted: the position price's move must exceed the average entry price's")
	}
	if got := TakeProfitPercentage(trade, price, fromAverage); got != fromAverage {
		t.Fatalf("control: an unlatched ladder's take profit reads the input %v, got %v", fromAverage, got)
	}

	latched, got := engineTick(trade, "", price, testutil.At("18:00:00"), ai)
	want := freeFallLatchRow(trade, freeFallLastBar)
	assertRow(t, got, want)
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.earlyTakeProfitWatched || st.slowDeclinePending {
		t.Fatalf("the tick after: latched, watched and not pending, got %+v", st)
	}
	var data EventData
	events := latched.StrategyEventsOf(aggragates.StrategyParamSmartTakeLoss, GateSlowDecline)
	if len(events) != 1 || events[0].DecodeData(&data) != nil || data.Event != EventLatched || !slices.Equal(data.Features, []string{FeatureEarlyTakeProfit}) {
		t.Fatalf("one latched event carrying the latch alone, got %+v", events)
	}
	if data.Price != want.Price || !slices.Equal(data.Reasons, want.Reasons) {
		t.Fatalf("the event carries the newest fill's price and the reading's reasons, got %+v", data)
	}
	if got := TakeProfitPercentage(latched, price, fromAverage); got != fromPosition {
		t.Fatalf("a latched ladder's take profit reads its position price %v, got %v", fromPosition, got)
	}

	later := []aggragates.AIIndicators{ai, freeFallMisses(aggragates.SideLong)["no row in free fall"], withBlock(aggragates.SmartTakeLossIndicators{})}
	for index, reading := range later {
		latched, got = engineTick(latched, "", price, testutil.At("18:15:00").Add(time.Duration(index)*time.Minute), reading)
		assertUntouched(t, got, "")
	}
	withEarlyTakeProfitFreeFallPrevention(t, false)
	if st := rebuildState(latched); !st.earlyTakeProfit || TakeProfitPercentage(latched, price, fromAverage) != fromPosition {
		t.Fatalf("nothing takes the latch away, the switch off included, got %+v", st)
	}
	if rows, latches := latchedFreeFallRows(latched), carriedLatches(latched); rows != 1 || latches != 1 || carriedEvents(latched, GateSlowDecline, EventLatched) != 1 {
		t.Fatalf("one row and one latched event over every tick, got %d and %d in %+v", rows, latches, latched.Logs)
	}
	assertUntouched(t, Apply(latchedBy(trade), "", price, ai), "")
}

// The latch has its own switch and the gate its own: the latch off, the reading latches nothing
// while the gate, on, still opens its episode; the latch on beside the gate on, the latched row goes
// out first and the triggered row after it on the same tick; the gate off, the latch reads all the same.
func TestTheFreeFallPreventionLatchAndTheGateAreSwitchedApart(t *testing.T) {
	onlyTheFreeFallPreventionLatch(t)
	trade := latchLadder(freeFallLatchDepth)
	ai := freeFallMarket(trade)
	newest := trade.PositionPrice
	resting := withRestingOrder(trade)

	withEarlyTakeProfitFreeFallPrevention(t, false)
	assertUntouched(t, Apply(trade, "", newest, ai), "")
	withFreeFallPrevention(t, true)
	assertWaits(t, Apply(resting, "buy", newest, ai), "buy", FreeFallPreventionTriggeredRow(newest, freeFallLastBar))
	if st := rebuildState(withRows(trade, FreeFallPreventionTriggeredRow(newest, freeFallLastBar))); st.earlyTakeProfit {
		t.Fatalf("the episode's triggered row latches nothing, got %+v", st)
	}

	withEarlyTakeProfitFreeFallPrevention(t, true)
	assertWaits(t, Apply(resting, "buy", newest, ai), "buy", freeFallLatchRow(trade, freeFallLastBar), FreeFallPreventionTriggeredRow(newest, freeFallLastBar))

	withFreeFallPrevention(t, false)
	assertRow(t, Apply(trade, "", newest, ai), freeFallLatchRow(trade, freeFallLastBar))
	withEarlyTakeProfit(t, false)
	assertUntouched(t, Apply(trade, "", newest, ai), "")
}

// A tick a depth priority holds reads nothing, the sophos flag aside; the ladder's next fill ends the
// hold and the reading latches it; with the pause off the hold changes nothing.
func TestTheFreeFallPreventionLatchReadsNoTickADepthPriorityHolds(t *testing.T) {
	onlyTheFreeFallPreventionLatch(t)
	withDepthPriorityPause(t, true)
	trade := latchLadder(freeFallLatchDepth)
	ai := freeFallMarket(trade)
	newest := trade.PositionPrice
	held := heldBy(trade, trade.History[len(trade.History)-1].CreatedAt.Add(time.Minute))
	if !rebuildState(held).depthPriorityHeld {
		t.Fatal("fixture drifted: the ladder must read held")
	}
	for _, position := range []string{"", "stopLoss", "takeProfit"} {
		assertUntouched(t, Apply(held, position, newest, ai), position)
	}
	flag := ai
	flag.SmartTakeLoss.Indecision, flag.SmartTakeLoss.QuietDeclineBreakReasons = true, indecisionReasons
	assertRow(t, Apply(held, "", newest, flag), LatchedRow(newest, indecisionFinding(indecisionReasons)))

	withDepthPriorityPause(t, false)
	assertRow(t, Apply(held, "", newest, ai), freeFallLatchRow(held, freeFallLastBar))
	withDepthPriorityPause(t, true)

	next := fills(freeFallLatchDepth+1, "19:00:00")[freeFallLatchDepth]
	resumed := withFill(held, next.Price, next.At)
	if rebuildState(resumed).depthPriorityHeld {
		t.Fatal("fixture drifted: the next fill ends the hold")
	}
	assertRow(t, Apply(resumed, "", next.Price, ai), freeFallLatchRow(resumed, freeFallLastBar))
}

// The latch reads a ladder only on a tick that wrote no row of its own, and only from the gate's
// arm depth: asked directly, each guard turns the answer to no, the quiet decline's row on the tick
// included, with the reading holding on every other count.
func TestTheFreeFallPreventionLatchReadingNeedsItsGuards(t *testing.T) {
	onlyTheFreeFallPreventionLatch(t)
	trade := latchLadder(freeFallLatchDepth)
	block := freeFallMarket(trade).SmartTakeLoss
	st := rebuildState(trade)

	reading, reads := earlyTakeProfitFreeFallPreventionReads(st, block, tick{})
	if !reads || !reading.Holds || !slices.Equal(reading.Reasons, freeFallLastBar) {
		t.Fatalf("control: the reading holds on the guards met, got %+v %v", reading, reads)
	}
	if _, reads := earlyTakeProfitFreeFallPreventionReads(st, freeFallMisses(aggragates.SideLong)["no row in free fall"].SmartTakeLoss, tick{}); reads {
		t.Error("a reading that misses latches nothing")
	}
	for _, kind := range []rowKind{rowTriggered, rowConfirmed, rowCancelled, rowReset, rowLatched} {
		if _, reads := earlyTakeProfitFreeFallPreventionReads(st, block, tick{kind: kind}); reads {
			t.Errorf("a tick that wrote a row of kind %d is left to the next tick", kind)
		}
	}
	held := st
	held.depthPriorityHeld = true
	if _, reads := earlyTakeProfitFreeFallPreventionReads(held, block, tick{}); reads {
		t.Error("a tick a depth priority holds reads nothing")
	}
	shallow := rebuildState(testutil.LadderTrade(false, fills(freefallprevention.ArmDepth-1, "17:38:00")...))
	if _, reads := earlyTakeProfitFreeFallPreventionReads(shallow, block, tick{}); reads {
		t.Errorf("under freefallprevention.ArmDepth (%d) fills the latch reads nothing, got %d fills", freefallprevention.ArmDepth, len(shallow.fills))
	}
	withEarlyTakeProfitFreeFallPrevention(t, false)
	if _, reads := earlyTakeProfitFreeFallPreventionReads(st, block, tick{}); reads {
		t.Error("the latch's switch off reads nothing")
	}
	withEarlyTakeProfitFreeFallPrevention(t, true)

	short := latchLadder(freeFallLatchDepth - 1)
	assertUntouched(t, Apply(short, "", short.PositionPrice, freeFallMarket(short)), "")
}

// A tick another reading wrote the row on leaves the latch to the next tick: the quiet decline goes
// pending first, its own latch off so that the pending row carries no marker.
func TestAnotherRowOnTheTickLeavesTheFreeFallPreventionLatchToTheNextTick(t *testing.T) {
	onlyTheFreeFallPreventionLatch(t)
	withQuietDecline(t, true)
	withEarlyTakeProfitQuietDecline(t, false)
	trade := watchedTrade()
	if !quietDeclineWatched(trade) || !earlyTakeProfitWatched(trade) || capitalProtectionWatched(trade) {
		t.Skip("the quiet decline or the early take profit does not watch the fixture ladder, or it stands at its last depth")
	}
	ai := withBlock(withVerdict(freeFallMarket(trade).SmartTakeLoss))
	gain := gainPrint(trade)

	pending, got := tickResting(trade, "", gain, testutil.At("18:00:00"), ai)
	assertRow(t, got, quietPending(slowDeclineLastFill))
	if st := rebuildState(pending); !st.slowDeclinePending || st.earlyTakeProfit {
		t.Fatalf("the tick that went pending latches nothing, got %+v", st)
	}
	latched, got := tickResting(pending, "", gain, testutil.At("18:15:00"), ai)
	assertRow(t, got, freeFallLatchRow(pending, freeFallLastBar))
	assertNoSale(t, got, "")
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.slowDeclinePending {
		t.Fatalf("the next tick latches the ladder, which stays pending, got %+v", st)
	}
	assertUntouched(t, Apply(withRestingOrder(latched), "", gain, ai), "")
}

// The readings before it in the latch's order win the one row of the tick: sophos' indecision and the
// flat market's bars, each alone when it reads beside the free fall, and this one once they do not.
func TestTheFreeFallPreventionLatchComesLastInTheLatchOrder(t *testing.T) {
	onlyTheFreeFallPreventionLatch(t)
	trade := latchLadder(freeFallLatchDepth)
	free := freeFallMarket(trade)
	newest := trade.PositionPrice

	indecision := indecisionReading()
	indecision.SmartTakeLoss = inFreeFall(indecision.SmartTakeLoss, []bool{true, false, false}, nil)
	assertRow(t, Apply(trade, "", newest, indecision), LatchedRow(newest, indecisionFinding(indecisionReasons)))

	if name := noFlatMarketMarket(trade.StrategyPair.StrategySettings[0].Percentage); name != "" {
		t.Skipf("%s leaves no market that holds for the fixture ladders to be read under", name)
	}
	both := latchMarket(t, trade, 0, 0)
	both.SmartTakeLoss = inFreeFall(both.SmartTakeLoss, []bool{true, false, false}, nil)
	flat := latchRowOf(t, trade, both.SmartTakeLoss)
	assertRow(t, Apply(trade, "", newest, both), flat)
	if reasons := flat.Reasons; len(reasons) == 0 || flat.Message == freeFallLatchRow(trade, freeFallLastBar).Message {
		t.Fatalf("fixture drifted: the flat market's bars name their own reasons, got %+v", flat)
	}

	withEarlyTakeProfitFlatMarket(t, false)
	assertRow(t, Apply(trade, "", newest, both), freeFallLatchRow(trade, freeFallLastBar))
	assertRow(t, Apply(trade, "", newest, free), freeFallLatchRow(trade, freeFallLastBar))
}
