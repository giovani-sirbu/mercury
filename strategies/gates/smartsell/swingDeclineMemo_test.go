package smartsell

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/swingdecline"
)

// The memo of the swing decline's reading (swingDeclineMemo) is exact: it answers
// what swingdecline.TriggerAtLastBar would, and works the answer out again whenever
// an input of it has moved. Every expectation below is the reading's own answer, so
// the tests hold under any calibration of the rule. The memo over many ladders and
// bars at once is swingDeclineMemoGrid_test.go's, the memo wired into the gate
// swingDeclineMemoWiring_test.go's.

// countingRead is swingdecline.TriggerAtLastBar counting the answers it works out.
type countingRead struct{ calls atomic.Int64 }

func (counter *countingRead) read(fills []series.Fill, tape series.Series, trend series.Trend) (swingdecline.Reading, bool) {
	counter.calls.Add(1)
	return swingdecline.TriggerAtLastBar(fills, tape, trend)
}

// memoInputs is what a call of the memo is made of.
type memoInputs struct {
	fills []entryFill
	tape  series.Series
	trend series.Trend
}

// memoLadder is the fills of the staircase's ladder with a fill in each of these bars.
func memoLadder(bars ...int) []entryFill {
	return rebuildState(stairLadder(bars...)).fills
}

func memoSeries(turns []float64, lastBar int) series.Series {
	return series.SeriesOf(stairBlock(turns, lastBar))
}

// baseInputs is a five-fill ladder and the series it was read on, under a trend
// that meets the need: the call every case below moves one input of.
func baseInputs() memoInputs {
	return memoInputs{
		fills: memoLadder(0, 9, 15, 24, 30),
		tape:  memoSeries(stairDecline, 36),
		trend: series.Trend{Down: patternTrendDown, Valid: patternTrendValid},
	}
}

func (in memoInputs) answerOf(memo *swingDeclineMemo) (swingdecline.Reading, bool) {
	return memo.answer(in.fills, in.tape, in.trend)
}

// edited is the inputs after edit has had its way with them: the fills and the
// series' arrays are copies, so the inputs it was called on stay as they were.
func (in memoInputs) edited(edit func(*memoInputs)) memoInputs {
	in.fills = slices.Clone(in.fills)
	in.tape = series.Series{Opens: slices.Clone(in.tape.Opens), Closes: slices.Clone(in.tape.Closes)}
	edit(&in)
	return in
}

// assertAnswersAsTheReadingDoes fails unless the memo answers the inputs with
// exactly what swingdecline.TriggerAtLastBar answers, the whole reading.
func assertAnswersAsTheReadingDoes(t *testing.T, name string, memo *swingDeclineMemo, in memoInputs) {
	t.Helper()
	want, wantHolds := swingdecline.TriggerAtLastBar(swingFills(in.fills), in.tape, in.trend)
	if got, holds := in.answerOf(memo); holds != wantHolds || !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: the memo answers %v %+v, the reading %v %+v", name, holds, got, wantHolds, want)
	}
}

func memoSize(memo *swingDeclineMemo) int {
	memo.mu.RLock()
	defer memo.mu.RUnlock()
	return len(memo.entries)
}

// The same inputs are worked out once, whatever the number of prints: every call
// answers the whole reading, and only the first reads a window.
func TestTheMemoWorksTheSameInputsOutOnce(t *testing.T) {
	counter := &countingRead{}
	memo := newSwingDeclineMemo(4, counter.read)
	for call := 0; call < 5; call++ {
		assertAnswersAsTheReadingDoes(t, "the same inputs", memo, baseInputs())
	}
	if got := counter.calls.Load(); got != 1 {
		t.Errorf("five prints on the same inputs worked the answer out %d times, want once", got)
	}
}

// An answer is worked out again when any input of it moves, never answered from the
// inputs it was worked out for: a new bar of the series, one older bar's open or
// close, a new fill, the bar of any fill — the reading starts a window at each of
// them, and the newest ends one — and either count of the trend. The key finds an
// entry by the first fill, the newest fill, the number of fills and the series' last
// bar (ladderKey): a move of an input it holds puts the moved inputs under an entry
// of their own, any other replaces the entry where it stands.
func TestTheMemoWorksTheAnswerOutAgainWhenAnyInputMoves(t *testing.T) {
	type move struct {
		name  string
		keyed bool
		edit  func(*memoInputs)
	}
	base := baseInputs()
	newest := len(base.fills) - 1
	moves := []move{
		{"a new bar", true, func(in *memoInputs) { in.tape = memoSeries(stairDecline, 37) }},
		{"an older bar's open", false, func(in *memoInputs) { in.tape.Opens[5]++ }},
		{"an older bar's close", false, func(in *memoInputs) { in.tape.Closes[5] += 0.01 }},
		{"a new fill", true, func(in *memoInputs) { in.fills = memoLadder(0, 9, 15, 24, 30, 36) }},
		{"the rows down", false, func(in *memoInputs) { in.trend.Down++ }},
		{"the rows valid", false, func(in *memoInputs) { in.trend.Valid++ }},
		{"the first fill's price", true, func(in *memoInputs) { in.fills[0].Price++ }},
	}
	for fill := range base.fills {
		moves = append(moves, move{
			name:  fmt.Sprintf("the bar of fill %d", fill+1),
			keyed: fill == 0 || fill == newest,
			edit:  func(in *memoInputs) { in.fills[fill].At = in.fills[fill].At.Add(time.Hour) },
		})
	}
	for _, tc := range moves {
		moved := base.edited(tc.edit)
		counter := &countingRead{}
		memo := newSwingDeclineMemo(4, counter.read)
		assertAnswersAsTheReadingDoes(t, tc.name+", before", memo, base)
		assertAnswersAsTheReadingDoes(t, tc.name, memo, moved)
		if got := counter.calls.Load(); got != 2 {
			t.Errorf("%s: the answer was worked out %d times over two different calls, want twice", tc.name, got)
		}
		assertAnswersAsTheReadingDoes(t, tc.name+", again", memo, moved)
		entries := 1
		if tc.keyed {
			entries = 2
		}
		if got, size := counter.calls.Load(), memoSize(memo); got != 2 || size != entries {
			t.Errorf("%s: %d answers worked out and %d entries kept after the moved inputs came again, want 2 and %d", tc.name, got, size, entries)
		}
	}
}

// assertTheMemoForgetsTheOldest asks a memo of three entries for each of the inputs
// in turn: it never holds more than its bound, the newest inputs are then answered
// from memory without a read, and the oldest is worked out again.
func assertTheMemoForgetsTheOldest(t *testing.T, name string, inputs []memoInputs) {
	t.Helper()
	const bound = 3
	counter := &countingRead{}
	memo := newSwingDeclineMemo(bound, counter.read)
	for index, in := range inputs {
		in.answerOf(memo)
		if size, want := memoSize(memo), min(index+1, bound); size != want {
			t.Fatalf("%s: after %d inputs the memo holds %d entries, want %d", name, index+1, size, want)
		}
	}
	worked := counter.calls.Load()
	for _, in := range inputs[len(inputs)-bound:] {
		assertAnswersAsTheReadingDoes(t, name+", a newest input", memo, in)
	}
	if got := counter.calls.Load(); got != worked {
		t.Errorf("%s: the %d newest inputs were worked out again: %d answers, want %d", name, bound, got, worked)
	}
	assertAnswersAsTheReadingDoes(t, name+", the oldest input", memo, inputs[0])
	if got, size := counter.calls.Load(), memoSize(memo); got != worked+1 || size != bound || len(memo.order) != bound {
		t.Errorf("%s: the oldest input forgotten: %d answers (want %d), %d entries and %d keys (want %d)", name, got, worked+1, size, len(memo.order), bound)
	}
}

// The memo holds at most its bound of entries, forgetting the oldest first: the
// newest are answered from memory and a forgotten one is worked out again. It holds
// for ladders and for one ladder growing by a fill, which is an entry for each
// depth, its oldest depths going first; and a bound under one is one.
func TestTheMemoHoldsNoMoreEntriesThanItsBound(t *testing.T) {
	tape, trend := memoSeries(stairDecline, 40), baseInputs().trend
	var ladders, depths []memoInputs
	for index := 0; index < 8; index++ {
		ladders = append(ladders, memoInputs{fills: memoLadder(index, 9+index, 15+index, 24+index), tape: tape, trend: trend})
	}
	bars := []int{0, 9, 15, 24, 30, 36, 42, 48}
	for depth := 3; depth <= len(bars); depth++ {
		depths = append(depths, memoInputs{fills: memoLadder(bars[:depth]...), tape: tape, trend: trend})
	}
	assertTheMemoForgetsTheOldest(t, "ladders", ladders)
	assertTheMemoForgetsTheOldest(t, "the depths of one ladder", depths)

	one := newSwingDeclineMemo(0, swingdecline.TriggerAtLastBar)
	ladders[0].answerOf(one)
	ladders[1].answerOf(one)
	if size := memoSize(one); size != 1 {
		t.Errorf("a bound under one is one, the memo holds %d entries", size)
	}
}
