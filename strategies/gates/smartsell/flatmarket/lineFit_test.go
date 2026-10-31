package flatmarket

import (
	"math"
	"testing"
)

// Closes on a straight line fit it exactly: the line's value at every bar is the close, its move
// over the window the line's, and no bar counts against the bound.
func TestFitRecoversAStraightLine(t *testing.T) {
	for _, movePct := range []float64{0, -9.84, 3.5} {
		bars := lineBars(lineFixtureBars, movePct, nil, nil)
		fitted := fit(bars.Closes)
		for index, closed := range bars.Closes {
			if math.Abs(fitted.at(index)-closed) > 1e-9 {
				t.Fatalf("move %v: the line reads %v at bar %d, the close is %v", movePct, fitted.at(index), index, closed)
			}
		}
		if got := fitted.movePct(lineFixtureBars - 1); math.Abs(got-movePct) > 1e-9 {
			t.Fatalf("the line moves %v over the window, want %v", got, movePct)
		}
		found := fitted.read(bars, fixtureBound)
		if found.counted != 0 || found.wide != 0 || found.above > 1e-9 || found.below < -1e-9 || math.Abs(found.widest) > 1e-9 {
			t.Fatalf("bars on the line count nothing, got %+v", found)
		}
	}
}

// A line that cannot be read answers what no check passes: no bars give the zero line, one bar a
// flat line through it, a start at or under zero a NaN move and an infinite deviation.
func TestFitFailsClosed(t *testing.T) {
	if got := fit(nil); got != (line{}) {
		t.Fatalf("no closes fit %+v, want the zero line", got)
	}
	if got := fit([]float64{42}); got.start != 42 || got.perBar != 0 {
		t.Fatalf("one close fits %+v, want a flat line through it", got)
	}
	if got := (line{start: -1, perBar: 1}).movePct(3); !math.IsNaN(got) {
		t.Fatalf("a line starting under zero moves %v, want NaN", got)
	}
	if got := deviationPct(100, 0); !math.IsInf(got, 1) {
		t.Fatalf("a close against a line at zero deviates %v, want +Inf", got)
	}
	if flat(math.NaN(), fixtureBound) || declines(math.NaN(), "long") {
		t.Fatal("a NaN move is neither flat nor a decline")
	}
}
