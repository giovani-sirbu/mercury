package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"reflect"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The swing decline's memo over many ladders, series and trends at once: what it
// shares with its callers, what it answers over a grid of inputs, and the ladders
// and bars that take turns through it. As in swingDeclineMemo_test.go, every
// expectation is the reading's own answer.

// A served array rewritten in place — the same backing array, the same length —
// is a different series: the memo compares what the arrays hold, not where they
// are, so it never answers for the series as it was. The ladder is the deepest
// the staircase's prices carry, so the series is read whatever depth the rule asks.
func TestTheMemoSeesAServedArrayRewrittenInPlace(t *testing.T) {
	counter := &countingRead{}
	memo := newSwingDeclineMemo(4, counter.read)
	in := baseInputs()
	in.fills, in.tape = memoLadder(0, 9, 15, 24, 30, 36, 42, 48), memoSeries(stairDecline, 48)
	before, _ := in.answerOf(memo)
	in.tape.Closes[len(in.tape.Closes)-1] *= 3
	if after, _ := swingdecline.TriggerAtLastBar(swingFills(in.fills), in.tape, in.trend); reflect.DeepEqual(before, after) {
		t.Skipf("a last bar tripled does not move the reading under the constants in force (%s)", constantsInForce())
	}
	assertAnswersAsTheReadingDoes(t, "the last close tripled in place", memo, in)
	if got := counter.calls.Load(); got != 2 {
		t.Errorf("the rewritten series was answered from memory: %d answers worked out, want 2", got)
	}
}

// scribble edits what a caller was handed, as a caller may: every string of the
// reading's Reasons and Breaks overwritten.
func scribble(reading swingdecline.Reading) {
	for _, text := range [][]string{reading.Reasons, reading.Breaks} {
		for index := range text {
			text[index] = "scribbled"
		}
	}
}

// The memo gives out copies: a caller that edits the slices of the reading it was
// handed — the one it worked out and the ones it remembered alike — leaves the
// next answer as the reading answers it, and two answers share no backing array.
// It holds for every input of the grid, whichever of Reasons and Breaks the
// reading fills.
func TestTheMemoSharesNoSliceWithItsCallers(t *testing.T) {
	ladders, tape, trends := gridInputs()
	memo := newSwingDeclineMemo(swingDeclineMemoBound, swingdecline.TriggerAtLastBar)
	for _, fills := range ladders {
		for _, served := range tape {
			for _, trend := range trends {
				in := memoInputs{fills: fills, tape: served, trend: trend}
				var handed []swingdecline.Reading
				for call := 0; call < 3; call++ {
					got, _ := in.answerOf(memo)
					handed = append(handed, got)
					scribble(got)
					assertAnswersAsTheReadingDoes(t, "after a caller scribbled on its answer", memo, in)
				}
				first, second := handed[0], handed[1]
				if (len(first.Reasons) > 0 && &first.Reasons[0] == &second.Reasons[0]) || (len(first.Breaks) > 0 && &first.Breaks[0] == &second.Breaks[0]) {
					t.Fatalf("two answers of the same inputs share a backing array: %+v", first)
				}
			}
		}
	}
}

// gridInputs are ladders, series and trends over the staircase: deep and shallow
// ladders, two fills in one bar, a fill with no stamp, series that end at every
// stage of the decline, a rally, a cut one, a mismatched one and none, and every
// shape of the trend's counts.
func gridInputs() (ladders [][]entryFill, tape []series.Series, trends []series.Trend) {
	unstamped := func(fill int) []entryFill {
		fills := memoLadder(0, 9, 15, 24, 30)
		fills[fill].At = time.Time{}
		return fills
	}
	ladders = [][]entryFill{nil, memoLadder(0), memoLadder(0, 9), memoLadder(0, 9, 15), memoLadder(0, 9, 15, 24)}
	ladders = append(ladders, memoLadder(0, 9, 15, 24, 30), memoLadder(0, 9, 15, 24, 30, 36), memoLadder(0, 0, 9, 15), memoLadder(6, 9, 15, 24))
	ladders = append(ladders, unstamped(0), unstamped(2), unstamped(4))
	for _, last := range []int{10, 24, 30, 36, 45, 60} {
		tape = append(tape, memoSeries(stairDecline, last), memoSeries(stairRally, last))
	}
	cut, mismatched := memoSeries(stairDecline, 45), memoSeries(stairDecline, 45)
	cut.Opens, cut.Closes, cut.Highs, cut.Lows = cut.Opens[20:], cut.Closes[20:], cut.Highs[20:], cut.Lows[20:]
	mismatched.Closes = mismatched.Closes[1:]
	tape = append(tape, cut, mismatched, series.Series{})
	for _, trend := range [][2]int{{0, 0}, {shortTrendDown, patternTrendValid}, {patternTrendDown, patternTrendValid}, {patternTrendValid, patternTrendValid}, {patternTrendDown, 0}} {
		trends = append(trends, series.Trend{Down: trend[0], Valid: trend[1]})
	}
	return ladders, tape, trends
}

// blockOf is the block that serves the inputs' series and trend.
func blockOf(in memoInputs) aggragates.SmartSellIndicators {
	return aggragates.SmartSellIndicators{
		SeriesOpens:   in.tape.Opens,
		SeriesCloses:  in.tape.Closes,
		SeriesHighs:   in.tape.Highs,
		SeriesLows:    in.tape.Lows,
		SmcTrendDown:  in.trend.Down,
		SmcTrendValid: in.trend.Valid,
	}
}

// Over every ladder, series and trend of the grid, in one order after another so
// that a ladder meets series and trends that are not its last, the memo answers
// exactly what the reading answers — a hit and a miss alike, under a bound that
// forgets nothing and under one that forgets all but three entries — and so does
// the gate's swingDeclineReads: the reading for a trend that meets the need, none
// for one that does not.
func TestTheMemoAnswersExactlyWhatTheReadingDoesOverAGrid(t *testing.T) {
	ladders, tape, trends := gridInputs()
	for _, bound := range []int{swingDeclineMemoBound, 3} {
		memo := newSwingDeclineMemo(bound, swingdecline.TriggerAtLastBar)
		for _, fills := range ladders {
			for _, served := range tape {
				for _, trend := range trends {
					in := memoInputs{fills: fills, tape: served, trend: trend}
					assertAnswersAsTheReadingDoes(t, "a miss", memo, in)
					assertAnswersAsTheReadingDoes(t, "a hit", memo, in)
					want, wantHolds := swingdecline.Reading{}, false
					if swingdecline.MeetsTheNeed(trend) {
						want, wantHolds = swingdecline.TriggerAtLastBar(swingFills(fills), served, trend)
					}
					if got, holds := swingDeclineReads(state{fills: fills, swingDeclineWatched: true}, blockOf(in)); holds != wantHolds || !reflect.DeepEqual(got, want) {
						t.Fatalf("swingDeclineReads: %v %+v, want %v %+v", holds, got, wantHolds, want)
					}
				}
			}
		}
	}
}

// Ladders that share a first fill — the same pair run twice in one process, as the
// backtests that run side by side do until their ladders diverge — and one ladder
// read on two bars, as an hour turns, take turns through one memo: each distinct
// input is worked out once, whatever the number of prints, and every print after
// it is answered from memory with the entry of its own inputs. The ladders differ
// in what the key holds beyond the first fill, one pair in the newest fill, another
// in the number of fills alone (two fills stamped alike end both), and the two bars
// in the series' last bar (ladderKey): found by the first fill alone, each print
// would take the entry of the print before it and work the reading out afresh.
func TestTheMemoWorksEachInputOutOnceWhileLaddersAndBarsTakeTurns(t *testing.T) {
	trend := baseInputs().trend
	var inputs []memoInputs
	for _, bars := range [][]int{{0, 9, 15, 24, 30}, {0, 9, 15, 24, 33}, {0, 9, 15, 24}, {0, 9, 15, 24, 24}} {
		for _, last := range []int{36, 37} {
			inputs = append(inputs, memoInputs{fills: memoLadder(bars...), tape: memoSeries(stairDecline, last), trend: trend})
		}
	}
	counter := &countingRead{}
	memo := newSwingDeclineMemo(len(inputs), counter.read)
	for round := 1; round <= 4; round++ {
		for _, in := range inputs {
			assertAnswersAsTheReadingDoes(t, "a print in its turn", memo, in)
		}
		if got := counter.calls.Load(); got != int64(len(inputs)) {
			t.Fatalf("after %d rounds of prints %d answers were worked out, want %d: one for each distinct input", round, got, len(inputs))
		}
	}
	if size := memoSize(memo); size != len(inputs) {
		t.Errorf("the memo holds %d entries, want one for each of the %d distinct inputs", size, len(inputs))
	}
}
