package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// stairTurns is the staircase of gates/swingdecline's own tests, three bars a
// leg — bars 0 to 30 — which reads a decline at each of its fills' bars: four
// down legs from lower highs by bar 24, five by bar 30.
var stairTurns = []float64{94, 100, 96, 98, 93, 95, 90, 92, 88, 89, 85}

// stairBounce is the same staircase with its last leg a rally instead, which
// reads no decline at bar 30.
var stairBounce = []float64{94, 100, 96, 98, 93, 95, 90, 92, 88, 110, 112}

// stairPrices are the w3s ladder's entry prices, the fills of stairLadder.
var stairPrices = []float64{201.46, 192.81, 188.58, 184.45, 179.78, 175.83, 171.97, 168.2}

// stairOpen is the open time of the staircase's bar: the fixture day's hours.
func stairOpen(bar int) time.Time {
	return testutil.At("00:00:00").Add(time.Duration(bar) * time.Hour)
}

// stairBlock serves bars 0 to lastBar of the staircase of turns as the block's
// series, with the sell band beside them and a Trend Direction row that just meets
// the need (withSmcTrend, patternTrendDown of patternTrendValid rows down): the
// tests that read a staircase's shape read it under a trend that never decides.
func stairBlock(turns []float64, lastBar int) aggragates.SmartTakeLossIndicators {
	closes := []float64{turns[0]}
	for index := 1; index < len(turns); index++ {
		from, to := turns[index-1], turns[index]
		closes = append(closes, from+(to-from)/3, from+2*(to-from)/3, to)
	}
	block := aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: slowDeclineBand}
	for bar := 0; bar <= lastBar && bar < len(closes); bar++ {
		block.SwingDeclineOpens = append(block.SwingDeclineOpens, stairOpen(bar).UnixMilli())
		block.SwingDeclineCloses = append(block.SwingDeclineCloses, closes[bar])
	}
	return withSmcTrend(block, patternTrendDown, patternTrendValid)
}

// stairLadder is the w3s ladder with a fill inside each of these bars of the
// staircase, at the w3s prices, in order.
func stairLadder(bars ...int) aggragates.Trades {
	fills := make([]testutil.LadderFill, len(bars))
	for index, bar := range bars {
		fills[index] = testutil.LadderFill{Price: stairPrices[index], At: stairOpen(bar).Add(17*time.Minute + 9*time.Second)}
	}
	return testutil.LadderTrade(false, fills...)
}

// stairReading is what the swing decline reads for the ladder on the block's
// series and trend: swingdecline.Trigger, whose own tests pin its texts, so the
// rows these tests expect follow whatever constants the rule is tuned to. It
// fails the test when the reading is not the one asked for.
func stairReading(t *testing.T, trade aggragates.Trades, block aggragates.SmartTakeLossIndicators, holds bool) swingdecline.Reading {
	t.Helper()
	reading, got := swingdecline.Trigger(swingFills(rebuildState(trade).fills), swingdecline.SeriesOf(block), swingdecline.TrendOf(block))
	if got != holds {
		t.Fatalf("fixture drifted: the swing decline reads holds=%v for the ladder, want %v: %+v", got, holds, reading)
	}
	return reading
}

// stairFourthBlock and stairFifthBlock are the staircase's series as the ticks
// that judge the fourth and the fifth fill serve it: closed to bar 24 and to bar
// 30, the bars the fills sit in.
func stairFourthBlock() aggragates.SmartTakeLossIndicators {
	return stairBlock(stairTurns, 24)
}

func stairFifthBlock() aggragates.SmartTakeLossIndicators {
	return stairBlock(stairTurns, 30)
}

// stairFourth is the ladder the staircase reads on at bar 24, its fourth
// fill's bar, before any tick; stairPending is that ladder with the rows of the
// tick it went pending on written: the one row, pending on the swing decline and
// latching the indecision direction beside it.
func stairFourth() aggragates.Trades {
	return stairLadder(0, 9, 15, 24)
}

func stairPending(t *testing.T) aggragates.Trades {
	t.Helper()
	trade, got := engineTick(stairFourth(), "", slowDeclineBand-1, testutil.At("00:30:00"), withBlock(stairFourthBlock()))
	if len(got.Rows) != 1 || got.Rows[0].Event != EventPending {
		t.Fatalf("fixture drifted: the staircase must go pending at bar 24, got %+v", got)
	}
	return trade
}

// stairFifthFill is the pending, latched staircase ladder (stairPending) with a
// fifth fill stamped inside bar 30, the bar the staircase's second reading ends at.
func stairFifthFill(t *testing.T) aggragates.Trades {
	t.Helper()
	return withFill(stairPending(t), stairPrices[4], stairOpen(30).Add(17*time.Minute+9*time.Second))
}

// The swing decline goes pending on the staircase at the fourth fill, and the
// row is the one a ladder that latches too writes: pending on the swing
// decline, the indecision direction beside it with no reasons of its own.
func stairPendingRow(t *testing.T) Row {
	t.Helper()
	return PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(stairReading(t, stairFourth(), stairFourthBlock(), true).Reasons))
}

// The read is bound to the bar that holds the newest fill, once that bar has
// closed: the bar before it reads nothing, the bar itself and the
// SwingDeclineReadBars after it write the pending row, and a bar later than that
// writes none — a ladder already deep at deploy, or whose read bars got no tick,
// never triggers late.
func TestSwingDeclineReadsOnlyInTheBarsAfterTheFill(t *testing.T) {
	trade := stairFourth()
	for lastBar, want := range map[int]bool{23: false, 24: true, 24 + swingdecline.SwingDeclineReadBars: true, 24 + swingdecline.SwingDeclineReadBars + 1: false} {
		got := Apply(trade, "", slowDeclineBand-1, withBlock(stairBlock(append(stairTurns, 84, 83, 82, 81), lastBar)))
		if (len(got.Rows) != 0) != want {
			t.Errorf("series to bar %d: pending row %v, want %v", lastBar, len(got.Rows) != 0, want)
		}
	}
}

// No series, no read: a block without the keys, one that serves none, and one
// whose arrays differ in length leave the ladder as it was — no row, no sale.
func TestSwingDeclineWaitsWithoutASeries(t *testing.T) {
	mismatched := stairFourthBlock()
	mismatched.SwingDeclineCloses = mismatched.SwingDeclineCloses[1:]
	for name, block := range map[string]aggragates.SmartTakeLossIndicators{
		"no keys":    {SlowDeclineSellBand: slowDeclineBand},
		"empty":      {SwingDeclineOpens: []int64{}, SwingDeclineCloses: []float64{}},
		"mismatched": mismatched,
	} {
		assertUntouched(t, Apply(stairFourth(), "", slowDeclineBand-1, withBlock(block)), "")
		if got := Apply(stairFourth(), "", slowDeclineBand+1, withBlock(block)); got.Reason != "" {
			t.Errorf("%s: a ladder not pending sells nothing at the band, got %+v", name, got)
		}
	}
}

// Only a long spot parent ladder from SwingDeclineArmDepth fills is read: one
// fill short, an inverse ladder, a futures one, a child, a strategy without
// the flag and the switch off write nothing; an impasse strategy's parent is
// read like any other. A futures ladder is still watched by the quiet decline,
// which reads no verdict here.
func TestSwingDeclineWatchesLongSpotParentsFromTheArmDepth(t *testing.T) {
	block := withBlock(stairFourthBlock())
	// One fill short of the arm depth, the newest still in bar 24.
	short := append(append([]int(nil), []int{0, 9, 15}[:max(0, min(SwingDeclineArmDepth-2, 3))]...), 24)
	for name, tc := range map[string]struct {
		mutate func(*aggragates.Trades)
		reads  bool
	}{
		"a long spot parent":          {func(*aggragates.Trades) {}, true},
		"an impasse strategy":         {func(trade *aggragates.Trades) { trade.Strategy.Params.Impasse = true }, true},
		"one fill short of the arm":   {func(trade *aggragates.Trades) { *trade = stairLadder(short...) }, false},
		"a futures ladder":            {func(trade *aggragates.Trades) { trade.Strategy.TradeType = aggragates.Futures }, false},
		"a child":                     {func(trade *aggragates.Trades) { trade.ParentID = 7 }, false},
		"a strategy without the flag": {func(trade *aggragates.Trades) { trade.Strategy.Params.SmartTakeLoss = false }, false},
		"an inverse ladder": {func(trade *aggragates.Trades) {
			trade.Inverse = true
			for index := range trade.History {
				trade.History[index].Type = "SELL"
			}
		}, false},
	} {
		trade := stairFourth()
		tc.mutate(&trade)
		if got := Apply(trade, "", slowDeclineBand-1, block); (len(got.Rows) != 0) != tc.reads {
			t.Errorf("%s: pending row %v, want %v", name, len(got.Rows) != 0, tc.reads)
		}
	}
	withSwingDecline(t, false)
	assertUntouched(t, Apply(stairFourth(), "", slowDeclineBand-1, block), "")
}

// The swing decline writes one row on the staircase at its fourth fill: the
// ladder goes pending on it, and the indecision direction, which watches the
// ladder too, latches on the same row. The row names the window and the
// readings that held, and the trend last.
func TestSwingDeclineGoesPendingAndLatchesOnOneRow(t *testing.T) {
	got := Apply(stairFourth(), "", slowDeclineBand-1, withBlock(stairFourthBlock()))
	want := stairPendingRow(t)
	assertRow(t, got, want)
	if want.Event != EventPending || want.Gate != GateSlowDecline || len(want.Features) != 2 || want.Features[0] != FeatureIndecision || want.Features[1] != FeatureSwingDecline {
		t.Fatalf("the row is the gate's pending kind on the indecision direction and the swing decline, got %+v", want)
	}
	assertNoSale(t, got, "")
	if st := rebuildState(withRow(stairFourth(), want, time.Time{})); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !st.indecision || len(st.pendingFeatures) != 1 || st.pendingFeatures[0] != FeatureSwingDecline {
		t.Fatalf("the row leaves the ladder pending from its fill on the swing decline, and latched, got %+v", st)
	}
}

// A ladder the swing decline made pending without a latch — the indecision
// direction was off when it went pending — is latched by the confirmation of
// its next fill once the direction watches it again: the confirmed row names the
// latch beside the swing decline, and no second one is written after it.
func TestSwingDeclineLatchesAConfirmationOnceToo(t *testing.T) {
	withIndecision(t, false)
	pending := stairPending(t)
	withIndecision(t, true)
	if rebuildState(pending).indecision {
		t.Fatal("fixture drifted: the ladder went pending without a latch")
	}
	fifth := withFill(pending, stairPrices[4], stairOpen(30).Add(17*time.Minute+9*time.Second))
	reading := stairReading(t, fifth, stairFifthBlock(), true)
	confirmed, got := engineTick(fifth, "", underTheBand, stairOpen(31), withBlock(stairFifthBlock()))
	assertRow(t, got, ConfirmedRow(stairPrices[4], indecisionFinding(nil), swingFinding(reading.Reasons)))
	sixth := withFill(confirmed, stairPrices[5], stairOpen(36).Add(17*time.Minute))
	sixthBlock := stairBlock(stairDecline, 36)
	again := Apply(sixth, "", underTheBand, withBlock(sixthBlock))
	assertRow(t, again, ConfirmedRow(stairPrices[5], swingFinding(stairReading(t, sixth, sixthBlock, true).Reasons)))
}
