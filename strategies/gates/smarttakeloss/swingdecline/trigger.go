package swingdecline

import "github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"

// Trigger reads a ladder's decline between its fills as a new fill is judged: every window from an
// earlier fill at least MinDepthsBetween depths back to the newest fill's bar, longest first
// (readWindows), the first that holds answering, folded with the tick's trend (withTrend).
func Trigger(fills []series.Fill, bars series.Series, trend series.Trend) (Reading, bool) {
	reading := readWindows(fills, bars, newestFillBar(fills)).withTrend(trend, MinSmcTrendDown)
	return reading, reading.Holds
}

// TriggerAtLastBar is Trigger with every window ending at the series' last bar instead of the newest
// fill's: how a ladder is read between fills, following the market bar by bar; it answers as
// Trigger does when the series ends at the newest fill's bar and fails closed wherever Trigger does.
func TriggerAtLastBar(fills []series.Fill, bars series.Series, trend series.Trend) (Reading, bool) {
	reading := readWindows(fills, bars, bars.LastBarOpen()).withTrend(trend, MinSmcTrendDown)
	return reading, reading.Holds
}

// newestFillBar is the newest fill's bar (series.BarOpen), zero without a fill or a stamp.
func newestFillBar(fills []series.Fill) int64 {
	if len(fills) == 0 {
		return 0
	}
	return series.BarOpen(fills[len(fills)-1].At)
}

// readWindows is the shape's half of Trigger and TriggerAtLastBar: the first window that holds,
// longest first, every window ending at endAt (fills of one bar read once, the first fill's led by
// LookbackBars), else the longest readable reading, else a reading whose Breaks say why nothing was read.
func readWindows(fills []series.Fill, bars series.Series, endAt int64) Reading {
	depth := len(fills)
	if depth < MinDepthsBetween+1 {
		return Reading{ToDepth: depth, Breaks: []string{fewFillsBreak(depth)}}
	}
	if newestFillBar(fills) == 0 {
		return Reading{FromDepth: 1, ToDepth: depth, Breaks: []string{unstampedBreak(depth)}}
	}

	var longest Reading
	var read, started bool
	var lastStartAt int64
	for from := 1; from <= depth-MinDepthsBetween; from++ {
		startAt := series.BarOpen(fills[from-1].At)
		if startAt == 0 {
			continue
		}
		if from == 1 {
			startAt = leadStart(bars, startAt)
		}
		if started && startAt == lastStartAt {
			continue
		}
		started, lastStartAt = true, startAt
		reading, readable := Read(bars, startAt, endAt, from, depth)
		if !readable {
			continue
		}
		if reading.Holds {
			return reading
		}
		if !read {
			longest, read = reading, true
		}
	}
	if read {
		return longest
	}
	return Reading{FromDepth: 1, ToDepth: depth, Breaks: []string{unreadableBreak(bars, depth)}}
}

// leadStart is where the first fill's window starts: the first bar held at or after LookbackBars
// before the fill's bar, the series' first bar when it reaches back less; the fill's bar itself
// when the series does not hold it, so the window stays unreadable, or with the lead off.
func leadStart(bars series.Series, fillBar int64) int64 {
	if _, held := bars.IndexOf(fillBar); !held || LookbackBars <= 0 {
		return fillBar
	}
	start, _ := bars.BarAtOrAfter(fillBar - int64(LookbackBars)*series.BarMs)
	return start
}
