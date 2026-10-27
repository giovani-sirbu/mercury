package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The swing decline reads on every tick of a watched ladder that is not pending,
// to the last closed bar of the series (swingDeclineReads). These tests move that
// bar tick by tick over the staircases of swingDecline_test.go and set the gate's
// rows against swingdecline.TriggerAtLastBar's own answer, so they follow
// whatever constants the rule is tuned to: a staircase that does not read as a
// test needs under the constants in force skips it, the constants named. The
// trend's side of the read is in swingDeclineEveryTickTrend_test.go. The ticks that
// read the gate's own row run at a print over the ladder's break even (gainPrint) with a buy order resting,
// where the ladder in buy is activated nothing and the row stands alone.

// lastBarBlock serves the staircase of turns closed to bar last, the Trend
// Direction row reading down on down of the dashboard's patternTrendValid rows.
func lastBarBlock(turns []float64, last, down int) aggragates.SmartTakeLossIndicators {
	return withSmcTrend(stairBlock(turns, last), down, patternTrendValid)
}

// readsAsOf is what a tick asks of the swing decline for a ladder the gate has not
// made pending: swingdecline.TriggerAtLastBar on the block.
func readsAsOf(trade aggragates.Trades, block aggragates.SmartTakeLossIndicators) (swingdecline.Reading, bool) {
	return swingdecline.TriggerAtLastBar(swingFills(rebuildState(trade).fills), series.SeriesOf(block), series.TrendOf(block))
}

// stairReadingAsOfNow is stairReading for that reading, the windows ending at the
// series' last bar. It fails the test when the reading is not the one asked for.
func stairReadingAsOfNow(t *testing.T, trade aggragates.Trades, block aggragates.SmartTakeLossIndicators, holds bool) swingdecline.Reading {
	t.Helper()
	reading, got := readsAsOf(trade, block)
	if got != holds {
		t.Fatalf("fixture drifted: the swing decline reads holds=%v for the ladder as of the last bar, want %v: %+v", got, holds, reading)
	}
	return reading
}

// firstHeldBar is the first bar from first to last whose series, closed there and
// under a Trend Direction row of down rows, holds the ladder's decline, or -1.
func firstHeldBar(trade aggragates.Trades, turns []float64, first, last, down int) int {
	for bar := first; bar <= last; bar++ {
		if _, holds := readsAsOf(trade, lastBarBlock(turns, bar, down)); holds {
			return bar
		}
	}
	return -1
}

// needsAScenario skips a test whose staircase does not read as it needs under the
// constants in force: a calibration moves the bars a staircase holds on, not what
// the gate does with the answer.
func needsAScenario(t *testing.T, premise bool, what string) {
	t.Helper()
	if !premise {
		t.Skipf("%s does not hold under the constants in force (%s)", what, constantsInForce())
	}
}

// A decline that missed at the newest fill's bar and holds on a later one goes
// pending on the first tick of that bar, with no new fill, and on no tick before
// it. The five-fill ladder never went pending and the rally breaks its window at
// the newest fill's bar, where the judgement of a fill (swingdecline.Trigger)
// misses; the series that runs on reads the staircase again. The row names the
// window up to the last closed bar.
func TestSwingDeclineGoesPendingOnTheFirstBarThatHoldsWithNoNewFill(t *testing.T) {
	withLinearDecline(t, false)
	trade, fillBar := stairLadder(0, 9, 15, 24, 30), 30
	missed := lastBarBlock(stairRally, fillBar, patternTrendDown)
	_, judged := swingdecline.Trigger(swingFills(rebuildState(trade).fills), series.SeriesOf(missed), series.TrendOf(missed))
	later := firstHeldBar(trade, stairRally, fillBar+1, 63, patternTrendDown)
	needsAScenario(t, rebuildState(trade).swingDeclineWatched && !judged && later > fillBar, "a window that misses at the newest fill's bar and holds later")

	for bar := fillBar; bar < later; bar++ {
		assertUntouched(t, applyAtAGain(trade, "", withBlock(lastBarBlock(stairRally, bar, patternTrendDown))), "")
	}
	block := lastBarBlock(stairRally, later, patternTrendDown)
	reading := stairReadingAsOfNow(t, trade, block, true)
	pending, got := tickAtAGain(trade, "", stairOpen(later+1), withBlock(block))
	assertRow(t, got, PendingRow(stairPrices[4], indecisionFinding(nil), swingFinding(reading.Reasons)))
	if end := stairOpen(later).UTC().Format("2006-01-02 15:04"); reading.EndAt != stairOpen(later).UnixMilli() || !strings.Contains(got.Rows[0].Message, " to "+end+" UTC") {
		t.Errorf("the row names the window up to the last closed bar, %s: %q", end, got.Rows[0].Message)
	}
	if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[4] || !st.earlyTakeProfit {
		t.Errorf("pending from the fifth fill and latched, got %+v", st)
	}
}

// A cancelled ladder is read again on every tick and goes pending again on a later
// bar, with no new fill: the fifth fill was cancelled where the rally breaks the
// window, the ticks of that same series write nothing — the same windows, no
// flip-flop — and the first bar of the staircase that carries on holds writes the
// pending row, none before it.
func TestSwingDeclineACancelledLadderGoesPendingAgainOnALaterBar(t *testing.T) {
	pending := pendingAtFourthWithTheFifth(indecisionFinding(nil), swingFinding(nil))
	cancelled := withRows(pending, CancelledRow(stairPrices[4], swingFinding(nil)))
	later := firstHeldBar(cancelled, stairRally, 30, 63, patternTrendDown)
	needsAScenario(t, rebuildState(cancelled).swingDeclineWatched && later > 30, "a window that misses at the rally and holds again on a later bar")

	for bar := 30; bar < later; bar++ {
		assertUntouched(t, applyAtAGain(cancelled, "", withBlock(lastBarBlock(stairRally, bar, patternTrendDown))), "")
	}
	block := lastBarBlock(stairRally, later, patternTrendDown)
	assertRow(t, applyAtAGain(cancelled, "", withBlock(block)), PendingRow(stairPrices[4], swingFinding(stairReadingAsOfNow(t, cancelled, block, true).Reasons)))
}

// The judgement of a pending ladder's new fill is unchanged: its windows end at the
// bar of that fill, whatever bars the series has run on to, where the read of a
// ladder that is not pending ends at the last closed bar.
func TestSwingDeclineJudgesANewFillAtItsOwnBarWhateverTheSeriesRunsOnTo(t *testing.T) {
	withSwingDecline(t, true)
	withSwingDeclineResetOnNewDepth(t, true)
	fifth, block := pendingAtFourthWithTheFifth(indecisionFinding(nil), swingFinding(nil)), lastBarBlock(stairDecline, 42, patternTrendDown)
	judged, holds := swingdecline.Trigger(swingFills(rebuildState(fifth).fills), series.SeriesOf(block), series.TrendOf(block))
	asOfNow, _ := readsAsOf(fifth, block)
	needsAScenario(t, holds && judged.EndAt != asOfNow.EndAt, "a window that holds at the fifth fill's bar")

	assertRow(t, applyAtAGain(fifth, "", withBlock(block)), ConfirmedRow(stairPrices[4], swingFinding(judged.Reasons)))
	if judged.EndAt != stairOpen(30).UnixMilli() || asOfNow.EndAt != stairOpen(42).UnixMilli() {
		t.Errorf("the judgement ends at the fifth fill's bar and the read at the last closed bar, got %d and %d", judged.EndAt, asOfNow.EndAt)
	}
}

// A pending ladder is neither confirmed nor cancelled between its fills — nothing
// re-confirms it — so a bar that breaks its window cancels nothing, writes no row
// and takes no action away, whatever the series reads on any tick: the shared action
// goes on at every tick, the stopLoss of the ladder in buy activated at a print under
// its break even and the ladder in stopLoss sold, whatever the ladder proposes.
func TestSwingDeclineNeverCancelsAPendingLadderBetweenItsFills(t *testing.T) {
	withBothDeclines(t)
	pending, broke := withRows(stairFourth(), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(nil))), 0
	needsAScenario(t, rebuildState(pending).slowDeclinePending, "a swing decline that watches the fourth fill")
	for bar := 25; bar <= 63; bar++ {
		block := lastBarBlock(stairRally, bar, patternTrendDown)
		if _, holds := readsAsOf(pending, block); !holds {
			broke++
		}
		assertActedOn(t, pending, withBlock(block))
	}
	needsAScenario(t, broke > 0, "a bar that breaks the window")
}

// A depth priority hold pauses the swing decline on every tick, however far the
// series has run: the ladder it would read goes pending when unheld and writes
// nothing when held, while the indecision latch of the sophos reading, which no
// hold pauses, still lands on the held tick.
func TestSwingDeclineAHeldLadderIsNotReadOnAnyBarButStillLatches(t *testing.T) {
	trade, block := stairFourth(), lastBarBlock(stairDecline, 42, patternTrendDown)
	reading, holds := readsAsOf(trade, block)
	needsAScenario(t, rebuildState(trade).swingDeclineWatched && holds, "a staircase that holds at the last bar")

	assertRow(t, applyAtAGain(trade, "", withBlock(block)), PendingRow(stairPrices[3], indecisionFinding(nil), swingFinding(reading.Reasons)))
	held := heldBy(trade, stairOpen(25))
	assertUntouched(t, Apply(held, "", underTheBand, withBlock(block)), "")
	block.Indecision, block.QuietDeclineBreakReasons = true, indecisionReasons
	assertRow(t, Apply(held, "", underTheBand, withBlock(block)), LatchedRow(stairPrices[3], indecisionFinding(indecisionReasons)))
}
