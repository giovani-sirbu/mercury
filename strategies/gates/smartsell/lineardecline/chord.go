package lineardecline

import (
	"math"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
)

// msPerHour is the hour in ms, the unit a chord's slope is read in.
const msPerHour = float64(time.Hour / time.Millisecond)

// chord is the line from the window's first fill to a later one: the start price and stamp, the
// price per ms, and slope, the net move in percent of the first price per hour (Reading.Slope).
type chord struct {
	startMs    int64
	startPrice float64
	perMs      float64
	slope      float64
}

// newChord draws the chord from the first fill to a later one, or a zero chord with the misses:
// missStamps (no bar, or the later fill not after the first), missPrices (a price not above zero).
func newChord(first, later series.Fill) (chord, misses) {
	var broken misses
	startMs, endMs := first.At.UnixMilli(), later.At.UnixMilli()
	if series.BarOpen(first.At) == 0 || series.BarOpen(later.At) == 0 || endMs <= startMs {
		broken |= missStamps
	}
	if !(first.Price > 0) || !(later.Price > 0) {
		broken |= missPrices
	}
	if broken != 0 {
		return chord{}, broken
	}
	elapsed := float64(endMs - startMs)
	return chord{
		startMs:    startMs,
		startPrice: first.Price,
		perMs:      (later.Price - first.Price) / elapsed,
		slope:      (later.Price/first.Price - 1) * 100 * msPerHour / elapsed,
	}, 0
}

// at is the line's price at a stamp in ms, past its end fill too.
func (line chord) at(ms int64) float64 {
	return line.startPrice + line.perMs*float64(ms-line.startMs)
}

// span is the bars of the window's first fill, the fill before the newest and the newest in the
// series; a chord's bars run from start to its own end, inclusive.
type span struct {
	start, previousEnd, newestEnd int
}

// spanOf finds the three fills' bars in the series by open time (series.Series.IndexOf); false
// when the series is not served or a bar is missing (an exchange leaves gaps).
func spanOf(bars series.Series, first, previous, newest series.Fill) (span, bool) {
	if !bars.Served() {
		return span{}, false
	}
	start, startFound := bars.IndexOf(series.BarOpen(first.At))
	previousEnd, previousFound := bars.IndexOf(series.BarOpen(previous.At))
	newestEnd, newestFound := bars.IndexOf(series.BarOpen(newest.At))
	return span{start: start, previousEnd: previousEnd, newestEnd: newestEnd}, startFound && previousFound && newestFound
}

// deviations is the largest deviation of each chord over its own bars (deviationOf at the bar's
// close time), in one pass; false on a close not above zero.
func deviations(bars series.Series, at span, newest, previous chord) (newestMax, previousMax float64, ok bool) {
	last := max(at.previousEnd, at.newestEnd)
	for index := at.start; index <= last; index++ {
		closed := bars.Closes[index]
		if !(closed > 0) {
			return 0, 0, false
		}
		closeAt := bars.Opens[index] + series.BarMs
		if index <= at.newestEnd {
			newestMax = max(newestMax, deviationOf(closed, newest.at(closeAt)))
		}
		if index <= at.previousEnd {
			previousMax = max(previousMax, deviationOf(closed, previous.at(closeAt)))
		}
	}
	return newestMax, previousMax, true
}

// deviationOf is how far a close lies from the line in absolute percent; infinite for a line not
// above zero.
func deviationOf(closed, line float64) float64 {
	if !(line > 0) {
		return math.Inf(1)
	}
	return math.Abs((closed/line - 1) * 100)
}
