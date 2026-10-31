package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The performance requirement of the swing decline's every-tick read, measured on
// Apply: the one call the engines make on every price print of an armed trade. A
// deep watched ladder that is not pending reads the swing decline on each print
// (swingDeclineReads), against a series of benchSeriesBars bars; a trend that meets
// swingdecline.MinSmcTrendDown lets the read reach its windows, where
// swingdecline.TriggerAtLastBar costs many times the rest of the tick. The memo
// (swingDeclineMemo) answers a print inside an hour — same fills, same series, same
// trend — with a lookup. Not run by default:
//
//	go test -run '^$' -bench 'ApplyWatchedLadder|ApplyLadders|SwingDeclineReads' -benchmem ./strategies/gates/smartsell/
//
// The fixture is checked for what it claims and skips, naming the constants in
// force, under a calibration it does not hold under. The same cost is held as a
// count of allocations by swingDeclineMemoWiring_test.go's tests, which run by
// default.

// benchSeriesBars is how many bars the series of the benchmarks holds: as long a
// window as sophos serves the rule, the one the read was measured on.
const benchSeriesBars = 360

// benchFillBars are the bars of the staircase's timeline (stairOpen) the fills of
// the benchmarks' ladder fall in: a deep ladder, every fill inside the series, so
// every window from an earlier fill is readable. benchSiblingFillBars is a second
// ladder that shares its first fill and differs in the newest alone.
var (
	benchFillBars        = []int{150, 170, 200, 240, 280, 320, 355}
	benchSiblingFillBars = []int{150, 170, 200, 240, 280, 320, 354}
)

// benchBlock serves benchSeriesBars bars of a market that rises with a wiggle, the
// first of them the staircase's bar shift: no window of it reads a decline, so the
// read walks every window and misses. The fill window sophos read is served beside
// the bars, and the Trend Direction row reads down of the patternTrendValid rows of
// the dashboard.
func benchBlock(shift, down int) aggragates.SmartSellIndicators {
	block := aggragates.SmartSellIndicators{QuietDeclineFillFrom: fillWindowFrom.UnixMilli()}
	for bar := shift; bar < shift+benchSeriesBars; bar++ {
		closed := 100 + 0.2*float64(bar) + 2*math.Sin(0.7*float64(bar))
		block.SeriesOpens = append(block.SeriesOpens, stairOpen(bar).UnixMilli())
		block.SeriesCloses = append(block.SeriesCloses, closed)
		block.SeriesHighs = append(block.SeriesHighs, closed)
		block.SeriesLows = append(block.SeriesLows, closed)
	}
	return withSmcTrend(block, down, patternTrendValid)
}

// benchTick is the deep ladder of benchFillBars and the tick the benchmarks serve
// it; benchTickOf is the same for the ladder with a fill in each of these bars. Both
// are checked for what they claim: the ladder is watched and not pending, the trend
// meets the need or not as asked, and Apply writes no row — when the trend meets the
// need, because the windows it reads miss.
func benchTick(tb testing.TB, shift, down int, meets bool) (aggragates.Trades, aggragates.AIIndicators) {
	tb.Helper()
	return benchTickOf(tb, benchFillBars, shift, down, meets)
}

func benchTickOf(tb testing.TB, fillBars []int, shift, down int, meets bool) (aggragates.Trades, aggragates.AIIndicators) {
	tb.Helper()
	trade, block := stairLadder(fillBars...), benchBlock(shift, down)
	if st := rebuildState(trade); !st.swingDeclineWatched || st.slowDeclinePending {
		tb.Skipf("the ladder is not watched and not pending: %+v", st)
	}
	if got := swingdecline.MeetsTheNeed(series.TrendOf(block)); got != meets {
		tb.Skipf("the trend %d of %d rows meets the need: %v, want %v (%s)", down, patternTrendValid, got, meets, constantsInForce())
	}
	if reading, holds := readsAsOf(trade, block); meets && (holds || reading.StartAt == 0) {
		tb.Skipf("the read must reach a window and miss, got holds=%v %+v (%s)", holds, reading, constantsInForce())
	}
	ai := withBlock(block)
	if got := Apply(trade, "", underTheBand, ai); len(got.Rows) != 0 {
		tb.Fatalf("fixture drifted: the ladder must stay not pending, got %+v", got.Rows)
	}
	return trade, ai
}

// reportReads reports how many answers the counter worked out per print: close to
// none when the memo answers the prints, one when every print reads the windows.
func reportReads(b *testing.B, counter *countingRead) {
	b.ReportMetric(float64(counter.calls.Load())/float64(b.N), "reads/print")
}

// BenchmarkApplyWatchedLadderTrendMet is the print the memo is for: the same fills,
// the same series and the same trend as the print before — one more print inside
// the hour — on a trend that meets the need.
func BenchmarkApplyWatchedLadderTrendMet(b *testing.B) {
	trade, ai := benchTick(b, 0, patternTrendDown, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Apply(trade, "", underTheBand, ai)
	}
}

// BenchmarkSwingDeclineReadsOnAHeldAnswer is the same print without the rest of the
// tick: swingDeclineReads alone, on inputs the memo already holds — the lookup, the
// comparison of the inputs and the copy of the reading it hands out.
func BenchmarkSwingDeclineReadsOnAHeldAnswer(b *testing.B) {
	trade, ai := benchTick(b, 0, patternTrendDown, true)
	st, block := rebuildState(trade), ai.SmartSell
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		swingDeclineReads(st, block)
	}
}

// BenchmarkApplyWatchedLadderTrendShort is the print whose trend misses the need:
// no window is read, with or without the memo, which this one keeps from being
// asked.
func BenchmarkApplyWatchedLadderTrendShort(b *testing.B) {
	trade, ai := benchTick(b, 0, shortTrendDown, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Apply(trade, "", underTheBand, ai)
	}
}

// BenchmarkApplyLaddersSharingAFirstFillTakingTurns is the prints of backtests that
// run side by side in one process, as the engines meet them: two ladders that share
// a first fill and differ in the newest, each read on two series that end on
// different bars, so the prints come from four different inputs in turn. Each is
// worked out once and every print after it is answered from memory, as the print
// of BenchmarkApplyWatchedLadderTrendMet is: the reads per print it reports are
// none. Found by the first fill alone, the four took turns at one entry and every
// print read the windows afresh.
func BenchmarkApplyLaddersSharingAFirstFillTakingTurns(b *testing.B) {
	counter := &countingRead{}
	withSwingMemo(b, newSwingDeclineMemo(swingDeclineMemoBound, counter.read))
	type tick struct {
		trade aggragates.Trades
		ai    aggragates.AIIndicators
	}
	var ticks []tick
	for _, fillBars := range [][]int{benchFillBars, benchSiblingFillBars} {
		for _, shift := range []int{0, 1} {
			trade, ai := benchTickOf(b, fillBars, shift, patternTrendDown, true)
			ticks = append(ticks, tick{trade, ai})
		}
	}
	counter.calls.Store(0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		one := ticks[i%len(ticks)]
		Apply(one.trade, "", underTheBand, one.ai)
	}
	reportReads(b, counter)
}

// BenchmarkApplyWatchedLadderNewSeriesEveryPrint is the worst case, which the
// engines only ever meet on the first print of an hour: a series whose last bar the
// memo holds no entry for, so the print reads the windows afresh and the memo only
// pays to remember it. The series, each ending on a bar of its own, rotate through
// one more than the memo has entries, so every print finds its entry already
// forgotten — the oldest goes first — and each one is a new entry with the oldest
// forgotten for it: one read per print.
func BenchmarkApplyWatchedLadderNewSeriesEveryPrint(b *testing.B) {
	const rotation = 3
	counter := &countingRead{}
	withSwingMemo(b, newSwingDeclineMemo(rotation-1, counter.read))
	var trade aggragates.Trades
	ticks := make([]aggragates.AIIndicators, rotation)
	for shift := range ticks {
		trade, ticks[shift] = benchTick(b, shift, patternTrendDown, true)
	}
	counter.calls.Store(0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Apply(trade, "", underTheBand, ticks[i%rotation])
	}
	reportReads(b, counter)
}
