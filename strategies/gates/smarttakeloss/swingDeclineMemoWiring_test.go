package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The swing decline's memo as the gate uses it, in the default run: swingDeclineReads
// asks it only after the cheap conditions, a print it holds an answer for allocates a
// handful of times, and prints of several ladders at once read through it without a
// race. The memo's own tests are swingDeclineMemo_test.go's and
// swingDeclineMemoGrid_test.go's, the benchmarks that measure it
// swingDeclineBench_test.go's.

// maxAllocsPerPrint is the most allocations one Apply on a watched ladder makes
// once the memo holds its answer: the tick's own, and the copy of the reading the
// memo hands out. The read itself allocates a hundred and more.
const maxAllocsPerPrint = 3

// A print the memo answers costs a handful of allocations, not the read's: the
// requirement the benchmarks measure, held as a count that does not move with the
// machine.
func TestAPrintTheMemoAnswersAllocatesAHandful(t *testing.T) {
	withLinearDecline(t, false)
	trade, ai := benchTick(t, 0, patternTrendDown, true)
	if allocs := testing.AllocsPerRun(50, func() { Apply(trade, "", underTheBand, ai) }); allocs > maxAllocsPerPrint {
		t.Errorf("a print inside the hour allocates %v times, want at most %d", allocs, maxAllocsPerPrint)
	}
}

// withSwingMemo runs the calling test or benchmark with memo as the process's memo
// and puts the process's own back when it ends. Like the package's switches it is
// package state, so a test that calls it never runs in parallel.
func withSwingMemo(tb testing.TB, memo *swingDeclineMemo) {
	tb.Helper()
	previous := swingMemo
	swingMemo = memo
	tb.Cleanup(func() { swingMemo = previous })
}

// The gate asks the memo only after the cheap conditions: a trend short of the need
// and a ladder the swing decline does not watch read no window and leave nothing in
// the memo. A watched ladder under a trend that meets the need is worked out once
// for the prints of an hour, each answered with the rows the first one wrote, and
// again for a new bar, a different trend and a new fill.
func TestSwingDeclineAsksTheMemoOnlyForAWatchedLadderUnderATrendThatMeetsTheNeed(t *testing.T) {
	counter := &countingRead{}
	memo := newSwingDeclineMemo(4, counter.read)
	withSwingMemo(t, memo)
	bars := []int{0, 9, 15, 24, 30, 36, 42}
	depth := min(swingdecline.ArmDepth, len(bars)-1)
	trade, shallow := stairLadder(bars[:depth]...), stairLadder(bars[:max(depth-1, 1)]...)
	met := lastBarBlock(stairDecline, 48, patternTrendDown)
	needsAScenario(t, rebuildState(trade).swingDeclineWatched && !rebuildState(shallow).swingDeclineWatched, "a ladder the swing decline watches and one it does not")

	type tick struct {
		trade aggragates.Trades
		block aggragates.SmartTakeLossIndicators
	}
	reads := map[string]tick{"a ladder not watched": {shallow, met}}
	if swingdecline.MinSmcTrendDown > 0 {
		reads["a trend short of the need"] = tick{trade, withSmcTrend(met, shortTrendDown, patternTrendValid)}
	}
	for name, one := range reads {
		for round := 0; round < 3; round++ {
			Apply(one.trade, "", underTheBand, withBlock(one.block))
		}
		if got, size := counter.calls.Load(), memoSize(memo); got != 0 || size != 0 {
			t.Errorf("%s: %d answers worked out and %d entries kept, want none", name, got, size)
		}
	}

	first := Apply(trade, "", underTheBand, withBlock(met))
	for round := 1; round < 6; round++ {
		if again := Apply(trade, "", underTheBand, withBlock(met)); !reflect.DeepEqual(again, first) {
			t.Fatalf("print %d answers %+v, the first print %+v", round, again, first)
		}
	}
	if got := counter.calls.Load(); got != 1 {
		t.Errorf("six prints inside the hour worked the answer out %d times, want once", got)
	}
	moved := map[string]tick{
		"a new bar":         {trade, lastBarBlock(stairDecline, 47, patternTrendDown)},
		"a different trend": {trade, withSmcTrend(stairBlock(stairDecline, 48), patternTrendDown, patternTrendValid+1)},
		"a new fill":        {withFill(trade, stairPrices[depth], stairOpen(bars[depth]).Add(17*time.Minute)), met},
	}
	for name, one := range moved {
		before := counter.calls.Load()
		Apply(one.trade, "", underTheBand, withBlock(one.block))
		Apply(one.trade, "", underTheBand, withBlock(one.block))
		if got := counter.calls.Load(); got != before+1 {
			t.Errorf("%s: two prints worked the answer out %d times, want once", name, got-before)
		}
	}
}

// Prints of several ladders at once — hermes ticks them concurrently — read through
// the one memo without a race, and each gets the answer it gets alone, under a memo
// so small that its ladders keep forgetting one another, with every caller editing
// what it is handed.
func TestSwingDeclineReadsConcurrentPrintsAnswerAsTheyDoAlone(t *testing.T) {
	withSwingMemo(t, newSwingDeclineMemo(2, swingdecline.TriggerAtLastBar))
	ladders, tape, _ := gridInputs()
	type job struct {
		st    state
		block aggragates.SmartTakeLossIndicators
		want  swingdecline.Reading
		holds bool
	}
	var jobs []job
	for index := 0; index < 6; index++ {
		in := memoInputs{fills: ladders[4+index], tape: tape[2*index], trend: series.Trend{Down: patternTrendDown, Valid: patternTrendValid}}
		want, holds := swingdecline.TriggerAtLastBar(swingFills(in.fills), in.tape, in.trend)
		jobs = append(jobs, job{state{fills: in.fills, swingDeclineWatched: true}, blockOf(in), want, holds})
	}
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for round := 0; round < 100; round++ {
				one := jobs[(worker+round)%len(jobs)]
				got, holds := swingDeclineReads(one.st, one.block)
				if holds != one.holds || !reflect.DeepEqual(got, one.want) {
					t.Errorf("worker %d round %d: %v %+v, alone %v %+v", worker, round, holds, got, one.holds, one.want)
					return
				}
				scribble(got)
			}
		}()
	}
	workers.Wait()
}
