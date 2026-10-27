package lineardecline

import "github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"

// Trigger is whether a ladder's newest Window fills sit on one line as of the tick, and the reading
// that decided: the trend first (MeetsTheNeed), then the fill count, then the two chords in one
// pass over the bars (assessAt); the same fills can trigger on a later tick whose trend turned down.
func Trigger(fills []series.Fill, bars series.Series, trend series.Trend) (Reading, bool) {
	if !MeetsTheNeed(trend) {
		return Reading{ToDepth: len(fills)}, false
	}
	first, previous, newest := readFills(fills)
	return TriggerAt(first, previous, newest, len(fills), bars, trend)
}

// TriggerAt is Trigger on the three fills the rule reads (the window's first, the one before the
// newest, the newest) and the newest's depth, passed by value so nothing allocates; a ladder under
// Window is read from none of them.
func TriggerAt(first, previous, newest series.Fill, depth int, bars series.Series, trend series.Trend) (Reading, bool) {
	if !MeetsTheNeed(trend) {
		return Reading{ToDepth: depth}, false
	}
	reading, missed := assessAt(first, previous, newest, depth, bars)
	if missed != 0 {
		return reading, false
	}
	reading.Holds = true
	reading.Reasons = holdReasons(reading, trend, MinSmcTrendDown)
	return reading, true
}

// Breaks names every miss of a ladder's line, the trend last, nothing when Trigger holds; it reads
// the line whatever the trend, shares assessAt with Trigger, and formats, so not for the hot path.
func Breaks(fills []series.Fill, bars series.Series, trend series.Trend) []string {
	reading, missed := assess(fills, bars)
	return breakReasons(reading, missed, trend, MinSmcTrendDown)
}

// assess is assessAt for a ladder held as a slice of fills.
func assess(fills []series.Fill, bars series.Series) (Reading, misses) {
	first, previous, newest := readFills(fills)
	return assessAt(first, previous, newest, len(fills), bars)
}

// assessAt reads the line on the three fills and the bars, the trend apart, and answers the reading
// and the conditions it misses, in cost order: the fill count, the chords (newChord), the fills'
// bars (spanOf), one pass over the bars (deviations); it formats and allocates nothing.
func assessAt(first, previous, newest series.Fill, depth int, bars series.Series) (Reading, misses) {
	reading := Reading{ToDepth: depth}
	if depth < Window {
		return reading, missFills
	}
	reading.FromDepth = depth - Window + 1
	reading.StartAt, reading.EndAt = series.BarOpen(first.At), series.BarOpen(newest.At)

	newestChord, broken := newChord(first, newest)
	previousChord, previousBroken := newChord(first, previous)
	if broken |= previousBroken; broken != 0 {
		return reading, broken
	}
	at, found := spanOf(bars, first, previous, newest)
	if !found {
		return reading, missBars
	}
	newestMax, previousMax, closed := deviations(bars, at, newestChord, previousChord)
	if !closed {
		return reading, missCloses
	}
	reading.Slope, reading.PreviousSlope = newestChord.slope, previousChord.slope
	reading.Deviation, reading.PreviousDeviation = newestMax, previousMax
	reading.NetPct = (newest.Price/first.Price - 1) * 100
	return reading, missesOf(reading)
}

// readFills are the window's first fill, the one before the newest and the newest; zero fills under
// Window.
func readFills(fills []series.Fill) (first, previous, newest series.Fill) {
	depth := len(fills)
	if depth < Window {
		return series.Fill{}, series.Fill{}, series.Fill{}
	}
	return fills[depth-Window], fills[depth-2], fills[depth-1]
}
