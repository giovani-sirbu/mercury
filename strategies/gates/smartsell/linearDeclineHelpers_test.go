package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/lineardecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The linear decline reads whether a ladder's fills sit on one straight line and,
// with the SMC trend direction down, latches the early take profit
// (linearDeclineReads, latchIndecision). Its reading, thresholds and texts are
// gates/smartsell/lineardecline's, which pins them in its own tests: these tests are about
// what the gate does with the answer. Every fixture is therefore built from the
// constants in force — a ladder on one line, every close of the series sophos
// serves on it — and no test goes red when the rule is retuned. They set every
// switch they rely on (onlyTheLinearDecline) and never assume the shipped ones.
//
// This file is what the tests share: the switches, the fixture of a ladder on one
// line and the block sophos serves for it. The tests are in linearDecline_test.go
// (what the gate reads of a ladder), linearDeclineLatch_test.go (the latch it lands)
// and linearDeclinePrecedence_test.go (what comes before it and what a print costs).

const (
	// linearLead is how many bars the series holds before the bar of the first fill,
	// linearGap how many lie between the bars of two successive fills and linearTail
	// how many it runs on past the bar of the newest fill.
	linearLead = 2
	linearGap  = 6
	linearTail = 4
	// linearOffset is how far into its bar each fill is stamped: an odd minute and
	// second, so that no test leans on a stamp sitting on a bar's open.
	linearOffset = 17*time.Minute + 9*time.Second
	// linearFirstPrice is the price of the first fill, which the line starts from.
	linearFirstPrice = 100.0
)

// linearFall is the net fall, in percent of the window's first fill's price, the ladder
// makes across the window, from the window's first fill to the newest: half as much
// again as the least the rule asks (lineardecline.MinFallPct), and never under a fixed
// step, so that a retune toward zero still leaves a fall to read. linearOver is a
// distance, in percent, off the line that is over the bound of every deviation
// (lineardecline.MaxDeviationPct), however the rule is calibrated. linearSlope is the
// slope, in percent per hour, of the line that falls linearFall across the window's
// fills, linearGap bars apart.
var (
	linearFall  = 1.5 * math.Max(-lineardecline.MinFallPct, 1)
	linearOver  = 1.5 * math.Max(lineardecline.MaxDeviationPct, 1)
	linearSlope = -linearFall / float64((lineardecline.Window-1)*linearGap)
)

// linearFinding is the finding of the linear decline, the reasons given, as
// quietFinding, swingFinding and indecisionFinding are the other features'.
func linearFinding(reasons []string) Finding {
	return Finding{Feature: FeatureLinearDecline, Reasons: reasons}
}

// withLinearDecline is withIndecision for the linear decline, the feature that makes a ladder
// pending and sells; the tests of the line were written for the latch alone (withEarlyTakeProfitLinearDecline).
func withLinearDecline(t *testing.T, on bool) {
	t.Helper()
	previous := linearDecline
	linearDecline = on
	t.Cleanup(func() { linearDecline = previous })
}

// onlyTheLinearDecline runs the calling test with the switches the tests of the line
// assume: the early take profit and its latch on the line on, the linear decline's own
// sale off, the quiet decline and the swing decline, which would write rows of their own,
// off, and the loss cap and the depth bound of the shared sale off. A test about another
// switch moves it after.
func onlyTheLinearDecline(t *testing.T) {
	t.Helper()
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitLinearDecline(t, true)
	withLinearDecline(t, false)
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withAllowedLoss(t, anyLoss)
	withSaleIgnoreLastDepths(t, 0)
}

// needsTheLinearTrend skips a test that has a trend fail a line when the condition is
// off: at lineardecline.MinSmcTrendDown zero or below no trend is read, so none can
// fail.
func needsTheLinearTrend(t *testing.T) {
	t.Helper()
	if lineardecline.MinSmcTrendDown <= 0 {
		t.Skip("lineardecline.MinSmcTrendDown is zero or below: the condition is off and no trend fails a line")
	}
}

// linearPriceAt is the price of the line of that slope hours after the first fill's
// stamp.
func linearPriceAt(slope, hours float64) float64 {
	return linearFirstPrice * (1 + slope/100*hours)
}

// linearOpen is the open time of the bar at index of the series: the fixture day's
// hours.
func linearOpen(index int) time.Time {
	return testutil.At("00:00:00").Add(time.Duration(index) * time.Hour)
}

// linearFills are the fills of a ladder depth deep on the line of that slope, a fill
// every linearGap bars, each stamped inside its bar.
func linearFills(slope float64, depth int) []testutil.LadderFill {
	fills := make([]testutil.LadderFill, depth)
	for index := range fills {
		fills[index] = testutil.LadderFill{
			Price: linearPriceAt(slope, float64(index*linearGap)),
			At:    linearOpen(linearLead + index*linearGap).Add(linearOffset),
		}
	}
	return fills
}

// linearLadder is the long spot ladder of those fills on the plain w3s row.
func linearLadder(slope float64, depth int) aggragates.Trades {
	return testutil.LadderTrade(false, linearFills(slope, depth)...)
}

// linearBlock is what sophos serves for a ladder depth fills deep on that line: the
// closed 1h bars from linearLead bars before the first fill's bar to tail bars past
// the bar of the depth-th fill, every close the line's price at the bar's close
// time, and a Trend Direction row reading down on down of its rows.
func linearBlock(slope float64, depth, tail, down int) aggragates.SmartSellIndicators {
	bars := linearLead + (depth-1)*linearGap + 1 + tail
	block := aggragates.SmartSellIndicators{
		SeriesOpens:   make([]int64, bars),
		SeriesCloses:  make([]float64, bars),
		SmcTrendDown:  down,
		SmcTrendValid: linearTrendValid,
	}
	for index := range bars {
		block.SeriesOpens[index] = linearOpen(index).UnixMilli()
		block.SeriesCloses[index] = linearPriceAt(slope, float64(index+1-linearLead)-linearOffset.Hours())
	}
	return block
}

// linearScene is the ladder linearDepth deep on one line and the block served for it
// under a Trend Direction row reading down on down rows.
func linearScene(down int) (aggragates.Trades, aggragates.SmartSellIndicators) {
	return linearLadder(linearSlope, linearDepth), linearBlock(linearSlope, linearDepth, linearTail, down)
}

// linearReading is lineardecline.Trigger's answer for the ladder on the block's
// series and trend. It fails the test when the line does not hold, so a fixture that
// drifts is told from a gate that misreads it.
func linearReading(t *testing.T, trade aggragates.Trades, block aggragates.SmartSellIndicators) lineardecline.Reading {
	t.Helper()
	fills := swingFills(rebuildState(trade).fills)
	tape, trend := series.SeriesOf(block), series.TrendOf(block)
	reading, holds := lineardecline.Trigger(fills, tape, trend)
	if !holds {
		t.Fatalf("fixture drifted: the ladder must sit on one line, breaks %q", lineardecline.Breaks(fills, tape, trend))
	}
	return reading
}

// linearRow is the row the linear decline's latch writes for that ladder: the latched
// row at its newest fill, naming the reasons of the line. It follows whatever
// constants the rule is tuned to.
func linearRow(t *testing.T, trade aggragates.Trades, block aggragates.SmartSellIndicators) Row {
	t.Helper()
	reading := linearReading(t, trade, block)
	return LatchedRow(rebuildState(trade).lastFill().Price, linearFinding(reading.Reasons))
}

// linearDepth is how deep the ladder the linear decline reads stands:
// lineardecline.ArmDepth fills, and never fewer than the window the rule reads
// (lineardecline.Window) nor than the early take profit watches a ladder from.
// linearTrendValid is how many rows the Trend Direction row reads, never fewer
// than the rule asks to read down. linearTrendMet is the count of rows down that
// just meets the need, linearTrendShort one a row short of it.
var (
	linearDepth      = max(lineardecline.ArmDepth, earlytakeprofit.ArmDepth, lineardecline.Window)
	linearTrendValid = max(3, lineardecline.MinSmcTrendDown)
	linearTrendMet   = max(lineardecline.MinSmcTrendDown, 0)
	linearTrendShort = lineardecline.MinSmcTrendDown - 1
)
