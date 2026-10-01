package slowpattern

import (
	"sort"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Series is the closed 1h bars sophos serves for the rule, as two parallel
// arrays, oldest first: Opens the open time of each bar in ms and Closes its
// close. A series whose arrays are empty or of different lengths is no series:
// nothing is read from it (Served).
type Series struct {
	Opens  []int64
	Closes []float64
}

// SeriesOf is the series of a smart take loss block: the two arrays sophos
// serves, as they decoded. A sophos without them, or one that could not read
// its window, serves none, and the series is not served.
func SeriesOf(block aggragates.SmartTakeLossIndicators) Series {
	return Series{Opens: block.SlowPatternOpens, Closes: block.SlowPatternCloses}
}

// Served is whether the series holds any bar: the two arrays carry the same
// number of them, and at least one.
func (series Series) Served() bool {
	return len(series.Opens) > 0 && len(series.Opens) == len(series.Closes)
}

// LastBarOpen is the open time, in ms, of the newest bar of the series — the
// last closed bar sophos read — and zero when the series is not served.
func (series Series) LastBarOpen() int64 {
	if !series.Served() {
		return 0
	}
	return series.Opens[len(series.Opens)-1]
}

// indexOf is the position of the bar opening at open, by binary search on the
// open times: never arithmetic on an index, since an exchange leaves gaps in a
// series. It answers false for a bar the series does not hold.
func (series Series) indexOf(open int64) (int, bool) {
	if !series.Served() {
		return 0, false
	}
	index := sort.Search(len(series.Opens), func(at int) bool { return series.Opens[at] >= open })
	if index == len(series.Opens) || series.Opens[index] != open {
		return 0, false
	}
	return index, true
}

// window is the positions of the bars opening at startAt and endAt, when a
// window between them can be read: both bars are in the series, the end is not
// before the start, the window spans at least the bars one pivot needs — the
// bar itself and SlowPatternPivotBars each side — and no close in it is at or
// under zero, which no market has and a percentage cannot be taken from.
func (series Series) window(startAt, endAt int64) (start, end int, ok bool) {
	start, startFound := series.indexOf(startAt)
	end, endFound := series.indexOf(endAt)
	if !startFound || !endFound || end < start || end-start+1 < 2*SlowPatternPivotBars+1 {
		return 0, 0, false
	}
	for _, closed := range series.Closes[start : end+1] {
		if closed <= 0 {
			return 0, 0, false
		}
	}
	return start, end, true
}
